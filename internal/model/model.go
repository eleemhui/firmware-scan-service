package model

import "time"

const (
	StatusScheduled = "scheduled"
	StatusStarted   = "started"
	StatusComplete  = "complete"
	StatusFailed    = "failed"
)

// FirmwareScan is stored in the firmware_scans MongoDB collection.
// _id is a UUID string generated at registration time.
type FirmwareScan struct {
	ID              string                 `bson:"_id"                         json:"id"`
	DeviceID        string                 `bson:"device_id"                   json:"device_id"`
	FirmwareVersion string                 `bson:"firmware_version"            json:"firmware_version"`
	BinaryHash      string                 `bson:"binary_hash"                 json:"binary_hash"`
	Metadata        map[string]interface{} `bson:"metadata,omitempty"          json:"metadata,omitempty"`
	Status          string                 `bson:"status"                      json:"status"`
	CreatedAt       time.Time              `bson:"created_at"                  json:"created_at"`
	UpdatedAt       time.Time              `bson:"updated_at"                  json:"updated_at"`
	ScanStartedAt   *time.Time             `bson:"scan_started_at,omitempty"   json:"scan_started_at,omitempty"`
	ScanCompletedAt *time.Time             `bson:"scan_completed_at,omitempty" json:"scan_completed_at,omitempty"`
}

type ScanJobMessage struct {
	ScanID string `json:"scan_id"`
}
