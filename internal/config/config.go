package config

import (
	"errors"
	"os"
)

type Config struct {
	Port      string
	DatabaseURL string
	AMQPUrl   string
	QueueName string
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		return nil, errors.New("AMQP_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	queue := os.Getenv("QUEUE_NAME")
	if queue == "" {
		queue = "firmware_scan_jobs"
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		AMQPUrl:     amqpURL,
		QueueName:   queue,
	}, nil
}
