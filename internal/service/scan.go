package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/queue"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RegisterScanRequest struct {
	DeviceID        string
	FirmwareVersion string
	BinaryHash      string
	Metadata        map[string]interface{}
}

// RegisterScan attempts to insert a new firmware scan document. If a document
// with the same (device_id, binary_hash) already exists (duplicate key error),
// it fetches and returns the existing record without publishing to the queue.
// This guarantees at-most-one queue message per unique (device_id, binary_hash).
func RegisterScan(
	ctx context.Context,
	database *mongo.Database,
	pub *queue.Publisher,
	req RegisterScanRequest,
) (*model.FirmwareScan, bool, error) {
	return registerScan(ctx, NewMongoScanRepo(database), pub, req)
}

func registerScan(
	ctx context.Context,
	repo ScanRepository,
	pub queue.MessagePublisher,
	req RegisterScanRequest,
) (scan *model.FirmwareScan, isNew bool, err error) {
	now := time.Now()
	scan = &model.FirmwareScan{
		DeviceID:        req.DeviceID,
		FirmwareVersion: req.FirmwareVersion,
		BinaryHash:      req.BinaryHash,
		Metadata:        req.Metadata,
		Status:          model.StatusScheduled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	id, insertErr := repo.Insert(ctx, scan)
	if insertErr == nil {
		scan.ID = id
		msg, _ := json.Marshal(model.ScanJobMessage{ScanID: scan.ID.Hex(), DeviceID: scan.DeviceID})
		if pubErr := pub.Publish(ctx, msg); pubErr != nil {
			return nil, false, fmt.Errorf("publish scan job: %w", pubErr)
		}
		return scan, true, nil
	}

	if !mongo.IsDuplicateKeyError(insertErr) {
		return nil, false, fmt.Errorf("insert firmware scan: %w", insertErr)
	}

	existing, err := repo.FindByDeviceHash(ctx, req.DeviceID, req.BinaryHash)
	if err != nil {
		return nil, false, fmt.Errorf("fetch existing scan: %w", err)
	}
	return existing, false, nil
}

// ClaimScan atomically transitions a scan from 'scheduled' → 'started'.
// Returns true only if this caller claimed it.
func ClaimScan(ctx context.Context, database *mongo.Database, id string) (bool, error) {
	return claimScan(ctx, NewMongoScanRepo(database), id)
}

func claimScan(ctx context.Context, repo ScanRepository, id string) (bool, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, fmt.Errorf("invalid scan id %q: %w", id, err)
	}
	return repo.Claim(ctx, oid)
}

// CompleteScan transitions a scan to 'complete'.
func CompleteScan(ctx context.Context, database *mongo.Database, id string) error {
	return completeScan(ctx, NewMongoScanRepo(database), id)
}

func completeScan(ctx context.Context, repo ScanRepository, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid scan id %q: %w", id, err)
	}
	return repo.Complete(ctx, oid)
}

// RecordVulnerabilities saves detected CVE IDs onto the scan document and
// upserts each CVE into the vulnerabilities collection.
func RecordVulnerabilities(ctx context.Context, database *mongo.Database, id string, cveIDs []string) error {
	return recordVulnerabilities(ctx, NewMongoScanRepo(database), NewMongoVulnRepo(database), id, cveIDs)
}

func recordVulnerabilities(ctx context.Context, scanRepo ScanRepository, vulnRepo VulnRepository, id string, cveIDs []string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid scan id %q: %w", id, err)
	}
	if err := scanRepo.SetVulns(ctx, oid, cveIDs); err != nil {
		return fmt.Errorf("record vulns on scan: %w", err)
	}
	if err := addVulnsToRegistry(ctx, vulnRepo, cveIDs, id); err != nil {
		return fmt.Errorf("update vulnerabilities collection: %w", err)
	}
	return nil
}

// AddVulnsToRegistry upserts one document per CVE ID into the vulnerabilities collection.
func AddVulnsToRegistry(ctx context.Context, database *mongo.Database, cveIDs []string, scanID string) error {
	return addVulnsToRegistry(ctx, NewMongoVulnRepo(database), cveIDs, scanID)
}

func addVulnsToRegistry(ctx context.Context, repo VulnRepository, cveIDs []string, scanID string) error {
	now := time.Now()
	for _, cveID := range cveIDs {
		if err := repo.UpsertCVE(ctx, cveID, scanID, now); err != nil {
			return fmt.Errorf("upsert vuln %s: %w", cveID, err)
		}
	}
	return nil
}

// RequeueStaleScan resets scans stuck in 'started' for longer than staleAfter.
func RequeueStaleScan(ctx context.Context, database *mongo.Database, staleAfter time.Duration) ([]model.ScanJobMessage, error) {
	return requeueStaleScan(ctx, NewMongoScanRepo(database), staleAfter)
}

func requeueStaleScan(ctx context.Context, repo ScanRepository, staleAfter time.Duration) ([]model.ScanJobMessage, error) {
	scans, err := repo.FindStale(ctx, time.Now().Add(-staleAfter))
	if err != nil {
		return nil, fmt.Errorf("find stale scans: %w", err)
	}
	var msgs []model.ScanJobMessage
	for _, s := range scans {
		if err := repo.ResetToScheduled(ctx, s.ID); err != nil {
			return nil, fmt.Errorf("reset stale scan %s: %w", s.ID, err)
		}
		msgs = append(msgs, model.ScanJobMessage{ScanID: s.ID.Hex(), DeviceID: s.DeviceID})
	}
	return msgs, nil
}

// RequeueOrphanedScheduled re-publishes scans stuck in 'scheduled' whose queue
// message was likely lost. Caps at 1000 oldest entries, skips those requeued
// within the last hour.
func RequeueOrphanedScheduled(ctx context.Context, database *mongo.Database, orphanAfter time.Duration) ([]model.ScanJobMessage, error) {
	return requeueOrphanedScheduled(ctx, NewMongoScanRepo(database), orphanAfter)
}

func requeueOrphanedScheduled(ctx context.Context, repo ScanRepository, orphanAfter time.Duration) ([]model.ScanJobMessage, error) {
	now := time.Now()
	scans, err := repo.FindOrphaned(ctx, now.Add(-orphanAfter), now.Add(-time.Hour), 1000)
	if err != nil {
		return nil, fmt.Errorf("find orphaned scans: %w", err)
	}
	var msgs []model.ScanJobMessage
	for _, s := range scans {
		if err := repo.StampRequeued(ctx, s.ID, now); err != nil {
			return nil, fmt.Errorf("stamp last_requeued_at for scan %s: %w", s.ID, err)
		}
		msgs = append(msgs, model.ScanJobMessage{ScanID: s.ID.Hex(), DeviceID: s.DeviceID})
	}
	return msgs, nil
}