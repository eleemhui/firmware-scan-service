package service

import (
	"context"
	"encoding/json"
	"fmt"

	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegisterScanRequest struct {
	DeviceID        string
	FirmwareVersion string
	BinaryHash      string
	Metadata        json.RawMessage
}

// RegisterScan atomically inserts a firmware scan record and, only if it is
// new, publishes a job to the queue. Returns the scan and isNew=true on first
// registration, or the existing scan with isNew=false on duplicates.
//
// Idempotency guarantee: the UNIQUE constraint on (device_id, binary_hash)
// combined with INSERT ... ON CONFLICT DO NOTHING RETURNING means exactly one
// concurrent caller will receive the row back. All others fall through to the
// SELECT path without publishing, guaranteeing at-most-one queue message per
// (device_id, binary_hash) pair.
func RegisterScan(
	ctx context.Context,
	pool *pgxpool.Pool,
	pub *queue.Publisher,
	req RegisterScanRequest,
) (scan *model.FirmwareScan, isNew bool, err error) {
	const insertSQL = `
		INSERT INTO firmware_scans (device_id, firmware_version, binary_hash, metadata)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (device_id, binary_hash) DO NOTHING
		RETURNING id, device_id, firmware_version, binary_hash, metadata, status,
		          created_at, updated_at, scan_started_at, scan_completed_at`

	row := pool.QueryRow(ctx, insertSQL,
		req.DeviceID, req.FirmwareVersion, req.BinaryHash, req.Metadata)

	s := &model.FirmwareScan{}
	err = row.Scan(&s.ID, &s.DeviceID, &s.FirmwareVersion, &s.BinaryHash,
		&s.Metadata, &s.Status, &s.CreatedAt, &s.UpdatedAt, &s.ScanStartedAt, &s.ScanCompletedAt)

	if err == nil {
		// New record inserted — publish job to queue.
		msg, _ := json.Marshal(model.ScanJobMessage{ScanID: s.ID})
		if pubErr := pub.Publish(ctx, msg); pubErr != nil {
			// The scan is persisted; publishing failed. Log and surface the
			// error so the caller can decide how to handle it. The scan
			// record already exists with status='scheduled' so a retry of
			// the HTTP request will return 200 (existing) rather than
			// publishing again.
			return nil, false, fmt.Errorf("publish scan job: %w", pubErr)
		}
		return s, true, nil
	}

	if err != pgx.ErrNoRows {
		return nil, false, fmt.Errorf("insert firmware scan: %w", err)
	}

	// Conflict — scan already registered. Fetch and return the existing record.
	const selectSQL = `
		SELECT id, device_id, firmware_version, binary_hash, metadata, status,
		       created_at, updated_at, scan_started_at, scan_completed_at
		FROM firmware_scans
		WHERE device_id = $1 AND binary_hash = $2`

	row = pool.QueryRow(ctx, selectSQL, req.DeviceID, req.BinaryHash)
	err = row.Scan(&s.ID, &s.DeviceID, &s.FirmwareVersion, &s.BinaryHash,
		&s.Metadata, &s.Status, &s.CreatedAt, &s.UpdatedAt, &s.ScanStartedAt, &s.ScanCompletedAt)
	if err != nil {
		return nil, false, fmt.Errorf("fetch existing scan: %w", err)
	}

	return s, false, nil
}

// UpdateScanStatus updates the status and updated_at of a scan by ID.
func UpdateScanStatus(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, status string) error {
	const sql = `UPDATE firmware_scans SET status = $1, updated_at = now() WHERE id = $2`
	_, err := pool.Exec(ctx, sql, status, id)
	if err != nil {
		return fmt.Errorf("update scan status: %w", err)
	}
	return nil
}

// ClaimScan atomically transitions a scan from 'scheduled' → 'started' and
// records scan_started_at. Returns true only if this caller successfully
// claimed the scan; false means another worker already claimed it.
func ClaimScan(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (bool, error) {
	const sql = `
		UPDATE firmware_scans
		SET status = 'started', scan_started_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'scheduled'`
	tag, err := pool.Exec(ctx, sql, id)
	if err != nil {
		return false, fmt.Errorf("claim scan: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// CompleteScan transitions a scan to 'complete' and records scan_completed_at.
func CompleteScan(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	const sql = `
		UPDATE firmware_scans
		SET status = 'complete', scan_completed_at = now(), updated_at = now()
		WHERE id = $1`
	_, err := pool.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("complete scan: %w", err)
	}
	return nil
}

// RequeueStaleScan resets scans that have been stuck in 'started' for longer
// than staleAfter back to 'scheduled' and returns their IDs so the caller can
// re-publish them to the queue.
func RequeueStaleScan(ctx context.Context, pool *pgxpool.Pool, staleAfter string) ([]uuid.UUID, error) {
	const sql = `
		UPDATE firmware_scans
		SET status = 'scheduled', updated_at = now()
		WHERE status = 'started'
		  AND updated_at < now() - $1::interval
		RETURNING id`
	rows, err := pool.Query(ctx, sql, staleAfter)
	if err != nil {
		return nil, fmt.Errorf("requeue stale scans: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan stale id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
