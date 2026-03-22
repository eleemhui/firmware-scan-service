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

	"go.mongodb.org/mongo-driver/mongo"
)

const staleThreshold = 5 * time.Minute

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client, err := db.NewClient(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("connect to mongodb: %v", err)
	}
	defer client.Disconnect(ctx)

	database := client.Database(cfg.MongoDBName)

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

	go runWatchdog(ctx, database, pub)

	log.Println("firmware_analysis_service started, waiting for jobs...")

	if err := consumer.Consume(ctx, makeHandler(database)); err != nil {
		if ctx.Err() == nil {
			log.Fatalf("consumer error: %v", err)
		}
		log.Println("consumer stopped:", err)
	}
}

func makeHandler(database *mongo.Database) func([]byte) error {
	return func(body []byte) error {
		var msg model.ScanJobMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("unmarshal message: %w", err)
		}
		return processScan(context.Background(), database, msg.ScanID, msg.DeviceID)
	}
}

func processScan(ctx context.Context, database *mongo.Database, scanID, deviceID string) error {
	claimed, err := service.ClaimScan(ctx, database, scanID)
	if err != nil {
		return fmt.Errorf("claim scan: %w", err)
	}
	if !claimed {
		log.Printf("scan %s: already claimed or complete, skipping", deviceID)
		return nil
	}

	log.Printf("scan %s: claimed, analysing...", deviceID)

	duration := time.Duration(2+rand.Intn(4)) * time.Second
	time.Sleep(duration)

	// 1 in 10 chance of detecting vulnerabilities.
	if rand.Intn(10) == 0 {
		vulns := randomCVEs()
		if err := service.RecordVulnerabilities(ctx, database, scanID, deviceID, vulns); err != nil {
			return fmt.Errorf("record vulnerabilities: %w", err)
		}
		log.Printf("scan %s: detected vulnerabilities %v", deviceID, vulns)
	}

	if err := service.CompleteScan(ctx, database, scanID); err != nil {
		return fmt.Errorf("set complete: %w", err)
	}

	log.Printf("scan %s: complete", deviceID)
	return nil
}

// randomCVEs returns 1–3 unique CVE IDs randomly selected from CVE-001 to CVE-100.
func randomCVEs() []string {
	count := 1 + rand.Intn(3)
	seen := make(map[int]bool)
	var vulns []string
	for len(vulns) < count {
		n := 1 + rand.Intn(100)
		if !seen[n] {
			seen[n] = true
			vulns = append(vulns, fmt.Sprintf("CVE-%03d", n))
		}
	}
	return vulns
}

func runWatchdog(ctx context.Context, database *mongo.Database, pub *queue.Publisher) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ids, err := service.RequeueStaleScan(ctx, database, staleThreshold)
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
