package config

import (
	"errors"
	"os"
)

type Config struct {
	Port        string
	MongoURI    string
	MongoDBName string
	AMQPUrl     string
	QueueName   string
}

func Load() (*Config, error) {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		return nil, errors.New("MONGO_URI is required")
	}

	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		return nil, errors.New("AMQP_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbName := os.Getenv("MONGO_DB")
	if dbName == "" {
		dbName = "firmware_db"
	}

	queue := os.Getenv("QUEUE_NAME")
	if queue == "" {
		queue = "firmware_scan_jobs"
	}

	return &Config{
		Port:        port,
		MongoURI:    mongoURI,
		MongoDBName: dbName,
		AMQPUrl:     amqpURL,
		QueueName:   queue,
	}, nil
}
