package service

import (
	"context"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/mongo"
)

// VulnService is the concrete implementation of VulnManager backed by MongoDB.
type VulnService struct {
	DB *mongo.Database
}

func (s *VulnService) AddVulns(ctx context.Context, cveIDs []string) ([]model.Vulnerability, error) {
	return AddVulns(ctx, s.DB, cveIDs)
}

func (s *VulnService) ListVulns(ctx context.Context) ([]model.Vulnerability, error) {
	return ListVulns(ctx, s.DB)
}
