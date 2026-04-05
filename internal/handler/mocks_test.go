package handler

import (
	"context"
	"errors"

	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/service"
)

// mockScanService implements service.ScanRegistrar.
type mockScanService struct {
	scan  *model.FirmwareScan
	isNew bool
	err   error
}

func (m *mockScanService) RegisterScan(_ context.Context, _ service.RegisterScanRequest) (*model.FirmwareScan, bool, error) {
	return m.scan, m.isNew, m.err
}

// mockVulnService implements service.VulnManager.
type mockVulnService struct {
	vulns []model.Vulnerability
	err   error
}

func (m *mockVulnService) AddVulns(_ context.Context, _ []string) ([]model.Vulnerability, error) {
	return m.vulns, m.err
}

func (m *mockVulnService) ListVulns(_ context.Context) ([]model.Vulnerability, error) {
	return m.vulns, m.err
}

var errService = errors.New("service error")
