package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewClient(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping: %w", err)
	}
	return client, nil
}

// CreateIndexes creates all required indexes. Safe to call on every startup —
// MongoDB ignores requests to create indexes that already exist.
func CreateIndexes(ctx context.Context, database *mongo.Database) error {
	// firmware_scans: unique index on (device_id, binary_hash) enforces
	// idempotency — exactly one scan per device/firmware-hash pair.
	_, err := database.Collection("firmware_scans").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "device_id", Value: 1}, {Key: "binary_hash", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uq_device_hash"),
	})
	if err != nil {
		return fmt.Errorf("create firmware_scans index: %w", err)
	}
	return nil
}
