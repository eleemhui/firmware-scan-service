package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/eleemhuis/firmware_scan_service/internal/config"
	"github.com/eleemhuis/firmware_scan_service/internal/db"
	"github.com/eleemhuis/firmware_scan_service/internal/handler"
	"github.com/eleemhuis/firmware_scan_service/internal/queue"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

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

	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("migrations applied")

	pub, err := queue.NewPublisher(cfg.AMQPUrl, cfg.QueueName)
	if err != nil {
		log.Fatalf("connect to rabbitmq: %v", err)
	}
	defer pub.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Post("/v1/firmware-scans", handler.NewScanHandler(pool, pub))
	r.Patch("/v1/findings/vulns", handler.NewAddVulnsHandler(pool))
	r.Get("/v1/findings/vulns", handler.NewListVulnsHandler(pool))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
