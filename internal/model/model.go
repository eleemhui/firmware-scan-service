package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	StatusScheduled = "scheduled"
	StatusStarted   = "started"
	StatusComplete  = "complete"
	StatusFailed    = "failed"
)

type FirmwareScan struct {
	ID              uuid.UUID       `json:"id"`
	DeviceID        string          `json:"device_id"`
	FirmwareVersion string          `json:"firmware_version"`
	BinaryHash      string          `json:"binary_hash"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ScanStartedAt   *time.Time      `json:"scan_started_at,omitempty"`
	ScanCompletedAt *time.Time      `json:"scan_completed_at,omitempty"`
}

type Vuln struct {
	ID        int       `json:"id"`
	CveID     string    `json:"cve_id"`
	CreatedAt time.Time `json:"created_at"`
}

type ScanJobMessage struct {
	ScanID uuid.UUID `json:"scan_id"`
}
