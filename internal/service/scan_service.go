package service

import (
	"context"

	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/queue"

	"go.mongodb.org/mongo-driver/mongo"
)

// ScanService is the concrete implementation of ScanRegistrar backed by MongoDB.
type ScanService struct {
	DB  *mongo.Database
	Pub *queue.Publisher
}

func (s *ScanService) RegisterScan(ctx context.Context, req RegisterScanRequest) (*model.FirmwareScan, bool, error) {
	return RegisterScan(ctx, s.DB, s.Pub, req)
}
