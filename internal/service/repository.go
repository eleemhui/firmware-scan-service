package service

import (
	"context"
	"time"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScanRepository abstracts all MongoDB operations on the firmware_scans collection.
type ScanRepository interface {
	Insert(ctx context.Context, scan *model.FirmwareScan) (primitive.ObjectID, error)
	FindByDeviceHash(ctx context.Context, deviceID, binaryHash string) (*model.FirmwareScan, error)
	Claim(ctx context.Context, id primitive.ObjectID) (bool, error)
	Complete(ctx context.Context, id primitive.ObjectID) error
	SetVulns(ctx context.Context, id primitive.ObjectID, cveIDs []string) error
	FindStale(ctx context.Context, before time.Time) ([]model.FirmwareScan, error)
	ResetToScheduled(ctx context.Context, id primitive.ObjectID) error
	FindOrphaned(ctx context.Context, createdBefore time.Time, requeueBefore time.Time, limit int64) ([]model.FirmwareScan, error)
	StampRequeued(ctx context.Context, id primitive.ObjectID, at time.Time) error
}

// VulnRepository abstracts all MongoDB operations on the vulnerabilities collection.
type VulnRepository interface {
	UpsertCVE(ctx context.Context, cveID, scanID string, now time.Time) error
	ListAll(ctx context.Context) ([]model.Vulnerability, error)
}