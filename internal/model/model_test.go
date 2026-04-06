package model

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestFirmwareScan_JSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	scan := FirmwareScan{
		ID:              primitive.NewObjectID(),
		DeviceID:        "device-123",
		FirmwareVersion: "2.4.1",
		BinaryHash:      "abc123",
		Status:          StatusScheduled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	data, err := json.Marshal(scan)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out FirmwareScan
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if out.DeviceID != scan.DeviceID {
		t.Errorf("DeviceID: got %s, want %s", out.DeviceID, scan.DeviceID)
	}
	if out.Status != StatusScheduled {
		t.Errorf("Status: got %s, want %s", out.Status, StatusScheduled)
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []string{StatusScheduled, StatusStarted, StatusComplete, StatusFailed}
	expected := []string{"scheduled", "started", "complete", "failed"}
	for i, s := range statuses {
		if s != expected[i] {
			t.Errorf("status[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestVulnerability_JSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	v := Vulnerability{
		CveID:           "CVE-042",
		FirstDetected:   now,
		LastDetected:    now,
		FirstDetectedBy: "scan-abc",
		LastDetectedBy:  "scan-abc",
		DetectedCount:   3,
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out Vulnerability
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if out.CveID != "CVE-042" {
		t.Errorf("CveID: got %s, want CVE-042", out.CveID)
	}
	if out.DetectedCount != 3 {
		t.Errorf("DetectedCount: got %d, want 3", out.DetectedCount)
	}
}

func TestScanJobMessage_JSONRoundTrip(t *testing.T) {
	msg := ScanJobMessage{ScanID: "scan-1", DeviceID: "device-1"}
	data, _ := json.Marshal(msg)

	var out ScanJobMessage
	json.Unmarshal(data, &out)

	if out.ScanID != msg.ScanID || out.DeviceID != msg.DeviceID {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, msg)
	}
}

func TestFirmwareScan_OptionalFieldsOmitted(t *testing.T) {
	scan := FirmwareScan{
		ID:       primitive.NewObjectID(),
		DeviceID: "dev-1",
		Status:   StatusScheduled,
	}

	data, _ := json.Marshal(scan)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["metadata"]; ok {
		t.Error("metadata should be omitted when nil")
	}
	if _, ok := raw["detected_vulns"]; ok {
		t.Error("detected_vulns should be omitted when nil")
	}
	if _, ok := raw["scan_started_at"]; ok {
		t.Error("scan_started_at should be omitted when nil")
	}
}