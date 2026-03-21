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
func RecordVulnerabilities(ctx context.Context, database *mongo.Database, id, deviceID string, cveIDs []string) error {
	_, err := database.Collection("firmware_scans").UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"detected_vulns": cveIDs, "updated_at": time.Now()}},
	)
	if err != nil {
		return fmt.Errorf("record vulns on scan: %w", err)
	}
	if err := AddVulnsToRegistry(ctx, database, cveIDs, deviceID); err != nil {
		return fmt.Errorf("update vulnerabilities collection: %w", err)
	}
	return nil
}

// AddVulnsToRegistry upserts one document per CVE ID into the vulnerabilities
// collection. If deviceID is non-empty it is added to that CVE's device_ids set.
func AddVulnsToRegistry(ctx context.Context, database *mongo.Database, cveIDs []string, deviceID string) error {
	coll := database.Collection("vulnerabilities")
	for _, cveID := range cveIDs {
		var update bson.M
		if deviceID != "" {
			update = bson.M{"$addToSet": bson.M{"device_ids": deviceID}}
		} else {
			update = bson.M{"$setOnInsert": bson.M{"device_ids": []string{}}}
		}
		if _, err := coll.UpdateOne(ctx, bson.M{"_id": cveID}, update, options.Update().SetUpsert(true)); err != nil {
			return fmt.Errorf("upsert vuln %s: %w", cveID, err)
		}
	}
	return nil
}

// RequeueStaleScan resets scans stuck in 'started' for longer than staleAfter
// back to 'scheduled' and returns their IDs so the caller can re-publish them.
func RequeueStaleScan(ctx context.Context, database *mongo.Database, staleAfter time.Duration) ([]string, error) {
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

	var ids []string
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
		ids = append(ids, s.ID)
	}
	return ids, cursor.Err()
}
