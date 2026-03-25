package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/queue"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
) (scan *model.FirmwareScan, isNew bool, err error) {
	coll := database.Collection("firmware_scans")
	now := time.Now()

	scan = &model.FirmwareScan{
		ID:              uuid.New().String(),
		DeviceID:        req.DeviceID,
		FirmwareVersion: req.FirmwareVersion,
		BinaryHash:      req.BinaryHash,
		Metadata:        req.Metadata,
		Status:          model.StatusScheduled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_, err = coll.InsertOne(ctx, scan)
	if err == nil {
		// New document — publish job to queue.
		msg, _ := json.Marshal(model.ScanJobMessage{ScanID: scan.ID, DeviceID: scan.DeviceID})
		if pubErr := pub.Publish(ctx, msg); pubErr != nil {
			return nil, false, fmt.Errorf("publish scan job: %w", pubErr)
		}
		return scan, true, nil
	}

	if !mongo.IsDuplicateKeyError(err) {
		return nil, false, fmt.Errorf("insert firmware scan: %w", err)
	}

	// Duplicate — fetch and return the existing record.
	existing := &model.FirmwareScan{}
	if err := coll.FindOne(ctx, bson.M{
		"device_id":   req.DeviceID,
		"binary_hash": req.BinaryHash,
	}).Decode(existing); err != nil {
		return nil, false, fmt.Errorf("fetch existing scan: %w", err)
	}

	return existing, false, nil
}

// ClaimScan atomically transitions a scan from 'scheduled' → 'started' and
// records scan_started_at. Returns true only if this caller claimed the scan;
// false means another worker already claimed it.
func ClaimScan(ctx context.Context, database *mongo.Database, id string) (bool, error) {
	now := time.Now()
	result := database.Collection("firmware_scans").FindOneAndUpdate(
		ctx,
		bson.M{"_id": id, "status": model.StatusScheduled},
		bson.M{"$set": bson.M{
			"status":          model.StatusStarted,
			"scan_started_at": now,
			"updated_at":      now,
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	if result.Err() == mongo.ErrNoDocuments {
		return false, nil
	}
	if result.Err() != nil {
		return false, fmt.Errorf("claim scan: %w", result.Err())
	}
	return true, nil
}

// CompleteScan transitions a scan to 'complete' and records scan_completed_at.
func CompleteScan(ctx context.Context, database *mongo.Database, id string) error {
	now := time.Now()
	_, err := database.Collection("firmware_scans").UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":            model.StatusComplete,
			"scan_completed_at": now,
			"updated_at":        now,
		}},
	)
	if err != nil {
		return fmt.Errorf("complete scan: %w", err)
	}
	return nil
}

// RecordVulnerabilities saves detected CVE IDs onto the scan document and
// upserts each CVE into the vulnerabilities collection, adding deviceID to its
// device_ids list.
func RecordVulnerabilities(ctx context.Context, database *mongo.Database, id string, cveIDs []string) error {
	_, err := database.Collection("firmware_scans").UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"detected_vulns": cveIDs, "updated_at": time.Now()}},
	)
	if err != nil {
		return fmt.Errorf("record vulns on scan: %w", err)
	}
	if err := AddVulnsToRegistry(ctx, database, cveIDs); err != nil {
		return fmt.Errorf("update vulnerabilities collection: %w", err)
	}
	return nil
}

// AddVulnsToRegistry upserts one document per CVE ID into the vulnerabilities
// collection. The CVE ID is the _id, guaranteeing uniqueness.
func AddVulnsToRegistry(ctx context.Context, database *mongo.Database, cveIDs []string) error {
	coll := database.Collection("vulnerabilities")
	for _, cveID := range cveIDs {
		_, err := coll.InsertOne(ctx, bson.M{"_id": cveID})
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("upsert vuln %s: %w", cveID, err)
		}
	}
	return nil
}

// RequeueStaleScan resets scans stuck in 'started' for longer than staleAfter
// back to 'scheduled' and returns their job messages so the caller can re-publish them.
func RequeueStaleScan(ctx context.Context, database *mongo.Database, staleAfter time.Duration) ([]model.ScanJobMessage, error) {
	staleTime := time.Now().Add(-staleAfter)
	coll := database.Collection("firmware_scans")

	cursor, err := coll.Find(ctx, bson.M{
		"status":     model.StatusStarted,
		"updated_at": bson.M{"$lt": staleTime},
	})
	if err != nil {
		return nil, fmt.Errorf("find stale scans: %w", err)
	}
	defer cursor.Close(ctx)

	var msgs []model.ScanJobMessage
	for cursor.Next(ctx) {
		var s model.FirmwareScan
		if err := cursor.Decode(&s); err != nil {
			return nil, fmt.Errorf("decode stale scan: %w", err)
		}
		_, err := coll.UpdateOne(ctx,
			bson.M{"_id": s.ID, "status": model.StatusStarted},
			bson.M{"$set": bson.M{"status": model.StatusScheduled, "updated_at": time.Now()}},
		)
		if err != nil {
			return nil, fmt.Errorf("reset stale scan %s: %w", s.ID, err)
		}
		msgs = append(msgs, model.ScanJobMessage{ScanID: s.ID, DeviceID: s.DeviceID})
	}
	return msgs, cursor.Err()
}

// RequeueOrphanedScheduled returns the IDs of scans that have been in 'scheduled'
// for longer than orphanAfter, indicating their RabbitMQ message was lost.
// The caller should re-publish these IDs to the queue; ClaimScan's atomic
// FindOneAndUpdate ensures a duplicate message is a no-op if a worker already claimed it.
// RequeueOrphanedScheduled finds up to 1000 of the oldest scans stuck in
// 'scheduled' whose RabbitMQ message was likely lost.  A scan is skipped if
// it was re-queued by the watchdog within the last hour, preventing repeated
// hammering of the same entry on every watchdog tick.
func RequeueOrphanedScheduled(ctx context.Context, database *mongo.Database, orphanAfter time.Duration) ([]model.ScanJobMessage, error) {
	now := time.Now()
	coll := database.Collection("firmware_scans")

	cursor, err := coll.Find(ctx,
		bson.M{
			"status":     model.StatusScheduled,
			"created_at": bson.M{"$lt": now.Add(-orphanAfter)},
			"$or": bson.A{
				bson.M{"last_requeued_at": bson.M{"$exists": false}},
				bson.M{"last_requeued_at": bson.M{"$lt": now.Add(-time.Hour)}},
			},
		},
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: 1}}). // oldest first
			SetLimit(1000),
	)
	if err != nil {
		return nil, fmt.Errorf("find orphaned scans: %w", err)
	}
	defer cursor.Close(ctx)

	var msgs []model.ScanJobMessage
	for cursor.Next(ctx) {
		var s model.FirmwareScan
		if err := cursor.Decode(&s); err != nil {
			return nil, fmt.Errorf("decode orphaned scan: %w", err)
		}
		if _, err := coll.UpdateOne(ctx,
			bson.M{"_id": s.ID},
			bson.M{"$set": bson.M{"last_requeued_at": now}},
		); err != nil {
			return nil, fmt.Errorf("stamp last_requeued_at for scan %s: %w", s.ID, err)
		}
		msgs = append(msgs, model.ScanJobMessage{ScanID: s.ID, DeviceID: s.DeviceID})
	}
	return msgs, cursor.Err()
}
