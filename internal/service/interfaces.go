package service

import (
	"context"

	"firmware-scan-service/internal/model"
)

// ScanRegistrar registers firmware scans and returns the resulting record.
type ScanRegistrar interface {
	RegisterScan(ctx context.Context, req RegisterScanRequest) (*model.FirmwareScan, bool, error)
}

// VulnManager manages the global CVE vulnerability registry.
type VulnManager interface {
	AddVulns(ctx context.Context, cveIDs []string) ([]model.Vulnerability, error)
	ListVulns(ctx context.Context) ([]model.Vulnerability, error)
}
