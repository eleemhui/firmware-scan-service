package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
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

	// .1% chance of simulating a worker crash mid-scan.
	// Status remains 'started'; the watchdog will re-enqueue after staleThreshold.
	if rand.Intn(100) == 0 {
		log.Printf("scan %s: simulated worker crash — exiting", deviceID)
		os.Exit(1)
	}

	// 1% chance of simulating a worker failure to complete mid-scan.
	// Status remains 'started'; the watchdog will re-enqueue after staleThreshold.
	if rand.Intn(100) == 0 {
		log.Printf("scan %s: simulated worker scan failure with no crash", deviceID)
		return nil
	}

	// 1 in 10 chance of detecting vulnerabilities.
	if rand.Intn(10) == 0 {
		vulns := randomCVEs()
		if err := service.RecordVulnerabilities(ctx, database, scanID, vulns); err != nil {
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

