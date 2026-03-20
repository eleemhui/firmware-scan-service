package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os/signal"
	"syscall"
	"time"

	"firmware-scan-service/internal/config"
	"firmware-scan-service/internal/db"
	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/queue"
	"firmware-scan-service/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// staleThreshold is how long a scan can stay in 'started' before the watchdog
// re-enqueues it. Should be longer than the maximum expected scan duration.
const staleThreshold = "5 minutes"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	consumer, err := queue.NewConsumer(cfg.AMQPUrl, cfg.QueueName)
	if err != nil {
		log.Fatalf("connect to rabbitmq: %v", err)
	}
	defer consumer.Close()

	pub, err := queue.NewPublisher(cfg.AMQPUrl, cfg.QueueName)
	if err != nil {
		log.Fatalf("connect rabbitmq publisher: %v", err)
	}
	defer pub.Close()

	// Watchdog: periodically re-enqueue scans stuck in 'started'.
	go runWatchdog(ctx, pool, pub)

	log.Println("firmware_analysis_service started, waiting for jobs...")

	if err := consumer.Consume(ctx, makeHandler(pool)); err != nil {
		if ctx.Err() == nil {
			log.Fatalf("consumer error: %v", err)
		}
		log.Println("consumer stopped:", err)
	}
}

func makeHandler(pool *pgxpool.Pool) func([]byte) error {
	return func(body []byte) error {
		var msg model.ScanJobMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("unmarshal message: %w", err)
		}
		return processScan(context.Background(), pool, msg.ScanID)
	}
}

// processScan claims the scan with a conditional UPDATE (scheduled → started).
// If another worker already claimed it the call is a no-op — the message is
// acked without duplicate processing.
func processScan(ctx context.Context, pool *pgxpool.Pool, scanID uuid.UUID) error {
	claimed, err := service.ClaimScan(ctx, pool, scanID)
	if err != nil {
		return fmt.Errorf("claim scan: %w", err)
	}
	if !claimed {
		log.Printf("scan %s: already claimed or complete, skipping", scanID)
		return nil // ack — nothing left to do
	}

	log.Printf("scan %s: claimed, analysing...", scanID)

	// Simulate firmware analysis (2–5 seconds).
	duration := time.Duration(2+rand.Intn(4)) * time.Second
	time.Sleep(duration)

	if err := service.CompleteScan(ctx, pool, scanID); err != nil {
		return fmt.Errorf("set complete: %w", err)
	}

	log.Printf("scan %s: complete", scanID)
	return nil
}

// runWatchdog periodically finds scans stuck in 'started' beyond staleThreshold,
// resets them to 'scheduled', and re-publishes them to the queue.
// Multiple worker replicas will each run this watchdog; the conditional UPDATE
// in RequeueStaleScan means each stale scan is only claimed by one replica.
func runWatchdog(ctx context.Context, pool *pgxpool.Pool, pub *queue.Publisher) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ids, err := service.RequeueStaleScan(ctx, pool, staleThreshold)
			if err != nil {
				log.Printf("watchdog: requeue error: %v", err)
				continue
			}
			for _, id := range ids {
				msg, _ := json.Marshal(model.ScanJobMessage{ScanID: id})
				if err := pub.Publish(ctx, msg); err != nil {
					log.Printf("watchdog: publish error for scan %s: %v", id, err)
				} else {
					log.Printf("watchdog: re-enqueued stale scan %s", id)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
