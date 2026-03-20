package handler

import (
	"encoding/json"
	"net/http"

	"firmware-scan-service/internal/queue"
	"firmware-scan-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type registerScanRequest struct {
	DeviceID        string          `json:"device_id"`
	FirmwareVersion string          `json:"firmware_version"`
	BinaryHash      string          `json:"binary_hash"`
	Metadata        json.RawMessage `json:"metadata"`
}

// NewScanHandler returns a handler for POST /v1/firmware-scans.
func NewScanHandler(pool *pgxpool.Pool, pub *queue.Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.DeviceID == "" || req.FirmwareVersion == "" || req.BinaryHash == "" {
			writeError(w, http.StatusBadRequest, "device_id, firmware_version, and binary_hash are required")
			return
		}

		scan, isNew, err := service.RegisterScan(r.Context(), pool, pub, service.RegisterScanRequest{
			DeviceID:        req.DeviceID,
			FirmwareVersion: req.FirmwareVersion,
			BinaryHash:      req.BinaryHash,
			Metadata:        req.Metadata,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to register scan")
			return
		}

		status := http.StatusOK
		if isNew {
			status = http.StatusCreated
		}
		writeJSON(w, status, scan)
	}
}
