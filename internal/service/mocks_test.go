package service

import (
	"context"
	"errors"
	"time"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ── Scan repo mock ────────────────────────────────────────────────────────────

type mockScanRepo struct {
	insertID    primitive.ObjectID
	insertErr   error
	findScan    *model.FirmwareScan
	findErr     error
	claimResult bool
	claimErr    error
	completeErr error
	setVulnsErr error
	staleScans  []model.FirmwareScan
	staleErr    error
	resetErr    error
	orphans     []model.FirmwareScan
	orphanErr   error
	stampErr    error
}

func (m *mockScanRepo) Insert(_ context.Context, _ *model.FirmwareScan) (primitive.ObjectID, error) {
	return m.insertID, m.insertErr
}
func (m *mockScanRepo) FindByDeviceHash(_ context.Context, _, _ string) (*model.FirmwareScan, error) {
	return m.findScan, m.findErr
}
func (m *mockScanRepo) Claim(_ context.Context, _ primitive.ObjectID) (bool, error) {
	return m.claimResult, m.claimErr
}
func (m *mockScanRepo) Complete(_ context.Context, _ primitive.ObjectID) error {
	return m.completeErr
}
func (m *mockScanRepo) SetVulns(_ context.Context, _ primitive.ObjectID, _ []string) error {
	return m.setVulnsErr
}
func (m *mockScanRepo) FindStale(_ context.Context, _ time.Time) ([]model.FirmwareScan, error) {
	return m.staleScans, m.staleErr
}
func (m *mockScanRepo) ResetToScheduled(_ context.Context, _ primitive.ObjectID) error {
	return m.resetErr
}
func (m *mockScanRepo) FindOrphaned(_ context.Context, _, _ time.Time, _ int64) ([]model.FirmwareScan, error) {
	return m.orphans, m.orphanErr
}
func (m *mockScanRepo) StampRequeued(_ context.Context, _ primitive.ObjectID, _ time.Time) error {
	return m.stampErr
}

// ── Vuln repo mock ────────────────────────────────────────────────────────────

type mockVulnRepo struct {
	upsertErr error
	vulns     []model.Vulnerability
	listErr   error
}

func (m *mockVulnRepo) UpsertCVE(_ context.Context, _, _ string, _ time.Time) error {
	return m.upsertErr
}
func (m *mockVulnRepo) ListAll(_ context.Context) ([]model.Vulnerability, error) {
	return m.vulns, m.listErr
}

// duplicateKeyError satisfies mongo.IsDuplicateKeyError
type duplicateKeyError struct{}

func (duplicateKeyError) Error() string { return "E11000 duplicate key error" }

func makeDupeError() error {
	return mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 11000, Message: "duplicate key error"}}}
}

// ── Publisher mock ────────────────────────────────────────────────────────────

type mockPublisher struct {
	err error
}

func (m *mockPublisher) Publish(_ context.Context, _ []byte) error {
	return m.err
}

var errRepo = errors.New("repository error")