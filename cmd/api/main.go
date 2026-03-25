package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"firmware-scan-service/internal/config"
	"firmware-scan-service/internal/db"
	"firmware-scan-service/internal/handler"
	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/queue"
	"firmware-scan-service/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/mongo"
)

const staleThreshold = 1 * time.Minute
const orphanedThreshold = 5 * time.Minute

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

	if err := db.CreateIndexes(ctx, database); err != nil {
		log.Fatalf("create indexes: %v", err)
	}
	log.Println("indexes ready")

	pub, err := queue.NewPublisher(cfg.AMQPUrl, cfg.QueueName)
	if err != nil {
		log.Fatalf("connect to rabbitmq: %v", err)
	}
	defer pub.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Post("/v1/firmware-scans", handler.NewScanHandler(database, pub))
	r.Patch("/v1/findings/vulns", handler.NewAddVulnsHandler(database))
	r.Get("/v1/findings/vulns", handler.NewListVulnsHandler(database))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go runWatchdog(ctx, database, pub)

	go func() {
		log.Printf("api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func randomWatchdogInterval() time.Duration {
	return time.Duration(1+rand.Intn(5)) * time.Minute
}

func runWatchdog(ctx context.Context, database *mongo.Database, pub *queue.Publisher) {
	next := randomWatchdogInterval()
	timer := time.NewTimer(next)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			log.Printf("watchdog: running stale scan check")
			publish := func(msgs []model.ScanJobMessage, label string) {
				for _, m := range msgs {
					body, _ := json.Marshal(m)
					if err := pub.Publish(ctx, body); err != nil {
						log.Printf("watchdog: publish error for %s scan %s (device %s): %v", label, m.ScanID, m.DeviceID, err)
					} else {
						log.Printf("watchdog: re-enqueued %s scan %s (device %s)", label, m.ScanID, m.DeviceID)
					}
				}
			}

			if msgs, err := service.RequeueStaleScan(ctx, database, staleThreshold); err != nil {
				log.Printf("watchdog: stale check error: %v", err)
			} else {
				publish(msgs, "stale")
			}

			if msgs, err := service.RequeueOrphanedScheduled(ctx, database, orphanedThreshold); err != nil {
				log.Printf("watchdog: orphan check error: %v", err)
			} else {
				publish(msgs, "orphaned")
			}

			next = randomWatchdogInterval()
			timer.Reset(next)
		case <-ctx.Done():
			return
		}
	}
}
