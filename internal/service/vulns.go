package service

import (
	"context"
	"fmt"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/mongo"
)

// AddVulns registers CVE IDs via the HTTP API (no device context).
func AddVulns(ctx context.Context, database *mongo.Database, cveIDs []string) ([]model.Vulnerability, error) {
	return addVulns(ctx, NewMongoVulnRepo(database), cveIDs)
}

func addVulns(ctx context.Context, repo VulnRepository, cveIDs []string) ([]model.Vulnerability, error) {
	if err := addVulnsToRegistry(ctx, repo, cveIDs, ""); err != nil {
		return nil, fmt.Errorf("add vulns: %w", err)
	}
	return repo.ListAll(ctx)
}

// ListVulns returns all vulnerability documents sorted by CVE ID.
func ListVulns(ctx context.Context, database *mongo.Database) ([]model.Vulnerability, error) {
	return NewMongoVulnRepo(database).ListAll(ctx)
}