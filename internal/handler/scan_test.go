package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"firmware-scan-service/internal/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNewScanHandler_NewScan_Returns201(t *testing.T) {
	scan := &model.FirmwareScan{
		ID:              primitive.NewObjectID(),
		DeviceID:        "device-abc",
		FirmwareVersion: "1.0.0",
		BinaryHash:      "abc123",
		Status:          model.StatusScheduled,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	svc := &mockScanService{scan: scan, isNew: true}
	handler := NewScanHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"device_id":        "device-abc",
		"firmware_version": "1.0.0",
		"binary_hash":      "abc123",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/firmware-scans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestNewScanHandler_DuplicateScan_Returns200(t *testing.T) {
	scan := &model.FirmwareScan{ID: primitive.NewObjectID(), DeviceID: "device-abc"}
	svc := &mockScanService{scan: scan, isNew: false}
	handler := NewScanHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"device_id":        "device-abc",
		"firmware_version": "1.0.0",
		"binary_hash":      "abc123",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/firmware-scans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestNewScanHandler_MissingFields_Returns400(t *testing.T) {
	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing device_id", map[string]string{"firmware_version": "1.0.0", "binary_hash": "abc"}},
		{"missing firmware_version", map[string]string{"device_id": "dev-1", "binary_hash": "abc"}},
		{"missing binary_hash", map[string]string{"device_id": "dev-1", "firmware_version": "1.0.0"}},
	}

	svc := &mockScanService{}
	handler := NewScanHandler(svc)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/v1/firmware-scans", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rec.Code)
			}
		})
	}
}

func TestNewScanHandler_InvalidJSON_Returns400(t *testing.T) {
	svc := &mockScanService{}
	handler := NewScanHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/firmware-scans", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestNewScanHandler_ServiceError_Returns500(t *testing.T) {
	svc := &mockScanService{err: errService}
	handler := NewScanHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"device_id":        "device-abc",
		"firmware_version": "1.0.0",
		"binary_hash":      "abc123",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/firmware-scans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}
