package service

import (
	"context"
	"fmt"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AddVulns registers CVE IDs via the HTTP API (no device context).
func AddVulns(ctx context.Context, database *mongo.Database, cveIDs []string) ([]model.Vulnerability, error) {
	if err := AddVulnsToRegistry(ctx, database, cveIDs); err != nil {
		return nil, fmt.Errorf("add vulns: %w", err)
	}
	return ListVulns(ctx, database)
}

// ListVulns returns all vulnerability documents sorted by CVE ID.
func ListVulns(ctx context.Context, database *mongo.Database) ([]model.Vulnerability, error) {
	cursor, err := database.Collection("vulnerabilities").Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.M{"_id": 1}),
	)
	if err != nil {
		return nil, fmt.Errorf("list vulns: %w", err)
	}
	defer cursor.Close(ctx)

	var vulns []model.Vulnerability
	if err := cursor.All(ctx, &vulns); err != nil {
		return nil, fmt.Errorf("decode vulns: %w", err)
	}
	return vulns, nil
}