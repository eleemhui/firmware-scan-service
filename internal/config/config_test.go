package config

import (
	"testing"
)

func TestLoad_MissingMongoURI_ReturnsError(t *testing.T) {
	t.Setenv("MONGO_URI", "")
	t.Setenv("AMQP_URL", "amqp://guest:guest@localhost:5672/")

	_, err := Load()
	if err == nil {
		t.Error("expected error when MONGO_URI is missing")
	}
}

func TestLoad_MissingAMQPURL_ReturnsError(t *testing.T) {
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("AMQP_URL", "")

	_, err := Load()
	if err == nil {
		t.Error("expected error when AMQP_URL is missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("AMQP_URL", "amqp://guest:guest@localhost:5672/")
	t.Setenv("PORT", "")
	t.Setenv("MONGO_DB", "")
	t.Setenv("QUEUE_NAME", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.MongoDBName != "firmware_db" {
		t.Errorf("expected default db firmware_db, got %s", cfg.MongoDBName)
	}
	if cfg.QueueName != "firmware_scan_jobs" {
		t.Errorf("expected default queue firmware_scan_jobs, got %s", cfg.QueueName)
	}
}

func TestLoad_ExplicitValues(t *testing.T) {
	t.Setenv("MONGO_URI", "mongodb://mongo:27017")
	t.Setenv("AMQP_URL", "amqp://user:pass@rabbit:5672/")
	t.Setenv("PORT", "9090")
	t.Setenv("MONGO_DB", "mydb")
	t.Setenv("QUEUE_NAME", "myqueue")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.MongoDBName != "mydb" {
		t.Errorf("expected db mydb, got %s", cfg.MongoDBName)
	}
	if cfg.QueueName != "myqueue" {
		t.Errorf("expected queue myqueue, got %s", cfg.QueueName)
	}
	if cfg.MongoURI != "mongodb://mongo:27017" {
		t.Errorf("unexpected MongoURI: %s", cfg.MongoURI)
	}
	if cfg.AMQPUrl != "amqp://user:pass@rabbit:5672/" {
		t.Errorf("unexpected AMQPUrl: %s", cfg.AMQPUrl)
	}
}
