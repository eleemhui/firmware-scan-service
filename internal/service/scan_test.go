package service

import (
	"context"
	"testing"
	"time"

	"firmware-scan-service/internal/model"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── registerScan ──────────────────────────────────────────────────────────────

func TestRegisterScan_NewScan_IsNew(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{insertID: id}
	req := RegisterScanRequest{DeviceID: "dev-1", FirmwareVersion: "1.0", BinaryHash: "abc"}

	scan, isNew, err := registerScan(context.Background(), repo, &mockPublisher{}, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for new scan")
	}
	if scan.DeviceID != "dev-1" {
		t.Errorf("unexpected DeviceID: %s", scan.DeviceID)
	}
}

func TestRegisterScan_PublishError_ReturnsError(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{insertID: id}
	req := RegisterScanRequest{DeviceID: "dev-1", FirmwareVersion: "1.0", BinaryHash: "abc"}

	_, _, err := registerScan(context.Background(), repo, &mockPublisher{err: errRepo}, req)
	if err == nil {
		t.Error("expected error when publish fails")
	}
}

func TestRegisterScan_Duplicate_ReturnsFalseIsNew(t *testing.T) {
	existing := &model.FirmwareScan{ID: primitive.NewObjectID(), DeviceID: "dev-1"}
	repo := &mockScanRepo{insertErr: makeDupeError(), findScan: existing}
	req := RegisterScanRequest{DeviceID: "dev-1", FirmwareVersion: "1.0", BinaryHash: "abc"}

	scan, isNew, err := registerScan(context.Background(), repo, &mockPublisher{}, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNew {
		t.Error("expected isNew=false for duplicate")
	}
	if scan.DeviceID != "dev-1" {
		t.Errorf("unexpected device id: %s", scan.DeviceID)
	}
}

func TestRegisterScan_InsertError_ReturnsError(t *testing.T) {
	repo := &mockScanRepo{insertErr: errRepo}
	req := RegisterScanRequest{DeviceID: "dev-1", FirmwareVersion: "1.0", BinaryHash: "abc"}

	_, _, err := registerScan(context.Background(), repo, &mockPublisher{}, req)
	if err == nil {
		t.Error("expected error for non-duplicate insert failure")
	}
}

func TestRegisterScan_DuplicateFindError_ReturnsError(t *testing.T) {
	repo := &mockScanRepo{insertErr: makeDupeError(), findErr: errRepo}
	req := RegisterScanRequest{DeviceID: "dev-1", FirmwareVersion: "1.0", BinaryHash: "abc"}

	_, _, err := registerScan(context.Background(), repo, &mockPublisher{}, req)
	if err == nil {
		t.Error("expected error when FindByDeviceHash fails")
	}
}

// ── claimScan ─────────────────────────────────────────────────────────────────

func TestClaimScan_InvalidID_ReturnsError(t *testing.T) {
	_, err := claimScan(context.Background(), &mockScanRepo{}, "not-an-objectid")
	if err == nil {
		t.Error("expected error for invalid ObjectID")
	}
}

func TestClaimScan_Claimed_ReturnsTrue(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{claimResult: true}

	ok, err := claimScan(context.Background(), repo, id.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected claimed=true")
	}
}

func TestClaimScan_AlreadyClaimed_ReturnsFalse(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{claimResult: false}

	ok, err := claimScan(context.Background(), repo, id.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected claimed=false")
	}
}

func TestClaimScan_RepoError_ReturnsError(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{claimErr: errRepo}

	_, err := claimScan(context.Background(), repo, id.Hex())
	if err == nil {
		t.Error("expected error from repo")
	}
}

// ── completeScan ──────────────────────────────────────────────────────────────

func TestCompleteScan_InvalidID_ReturnsError(t *testing.T) {
	err := completeScan(context.Background(), &mockScanRepo{}, "bad-id")
	if err == nil {
		t.Error("expected error for invalid ObjectID")
	}
}

func TestCompleteScan_Success(t *testing.T) {
	id := primitive.NewObjectID()
	err := completeScan(context.Background(), &mockScanRepo{}, id.Hex())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompleteScan_RepoError_ReturnsError(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{completeErr: errRepo}
	err := completeScan(context.Background(), repo, id.Hex())
	if err == nil {
		t.Error("expected error from repo")
	}
}

// ── recordVulnerabilities ─────────────────────────────────────────────────────

func TestRecordVulnerabilities_InvalidID_ReturnsError(t *testing.T) {
	err := recordVulnerabilities(context.Background(), &mockScanRepo{}, &mockVulnRepo{}, "bad-id", []string{"CVE-001"})
	if err == nil {
		t.Error("expected error for invalid ObjectID")
	}
}

func TestRecordVulnerabilities_SetVulnsError_ReturnsError(t *testing.T) {
	id := primitive.NewObjectID()
	scanRepo := &mockScanRepo{setVulnsErr: errRepo}
	err := recordVulnerabilities(context.Background(), scanRepo, &mockVulnRepo{}, id.Hex(), []string{"CVE-001"})
	if err == nil {
		t.Error("expected error when SetVulns fails")
	}
}

func TestRecordVulnerabilities_UpsertError_ReturnsError(t *testing.T) {
	id := primitive.NewObjectID()
	vulnRepo := &mockVulnRepo{upsertErr: errRepo}
	err := recordVulnerabilities(context.Background(), &mockScanRepo{}, vulnRepo, id.Hex(), []string{"CVE-001"})
	if err == nil {
		t.Error("expected error when UpsertCVE fails")
	}
}

func TestRecordVulnerabilities_Success(t *testing.T) {
	id := primitive.NewObjectID()
	err := recordVulnerabilities(context.Background(), &mockScanRepo{}, &mockVulnRepo{}, id.Hex(), []string{"CVE-001", "CVE-002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── addVulnsToRegistry ────────────────────────────────────────────────────────

func TestAddVulnsToRegistry_EmptyList_NoError(t *testing.T) {
	err := addVulnsToRegistry(context.Background(), &mockVulnRepo{}, []string{}, "scan-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddVulnsToRegistry_UpsertError_ReturnsError(t *testing.T) {
	repo := &mockVulnRepo{upsertErr: errRepo}
	err := addVulnsToRegistry(context.Background(), repo, []string{"CVE-001"}, "scan-1")
	if err == nil {
		t.Error("expected error from UpsertCVE")
	}
}

// ── requeueStaleScan ──────────────────────────────────────────────────────────

func TestRequeueStaleScan_NoStaleScans_ReturnsEmpty(t *testing.T) {
	msgs, err := requeueStaleScan(context.Background(), &mockScanRepo{}, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestRequeueStaleScan_FindError_ReturnsError(t *testing.T) {
	repo := &mockScanRepo{staleErr: errRepo}
	_, err := requeueStaleScan(context.Background(), repo, time.Minute)
	if err == nil {
		t.Error("expected error from FindStale")
	}
}

func TestRequeueStaleScan_ResetError_ReturnsError(t *testing.T) {
	repo := &mockScanRepo{
		staleScans: []model.FirmwareScan{{ID: primitive.NewObjectID(), DeviceID: "dev-1"}},
		resetErr:   errRepo,
	}
	_, err := requeueStaleScan(context.Background(), repo, time.Minute)
	if err == nil {
		t.Error("expected error from ResetToScheduled")
	}
}

func TestRequeueStaleScan_ReturnsMessages(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{
		staleScans: []model.FirmwareScan{{ID: id, DeviceID: "dev-1"}},
	}
	msgs, err := requeueStaleScan(context.Background(), repo, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ScanID != id.Hex() {
		t.Errorf("unexpected ScanID: %s", msgs[0].ScanID)
	}
	if msgs[0].DeviceID != "dev-1" {
		t.Errorf("unexpected DeviceID: %s", msgs[0].DeviceID)
	}
}

// ── requeueOrphanedScheduled ──────────────────────────────────────────────────

func TestRequeueOrphanedScheduled_NoOrphans_ReturnsEmpty(t *testing.T) {
	msgs, err := requeueOrphanedScheduled(context.Background(), &mockScanRepo{}, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestRequeueOrphanedScheduled_FindError_ReturnsError(t *testing.T) {
	repo := &mockScanRepo{orphanErr: errRepo}
	_, err := requeueOrphanedScheduled(context.Background(), repo, time.Minute)
	if err == nil {
		t.Error("expected error from FindOrphaned")
	}
}

func TestRequeueOrphanedScheduled_StampError_ReturnsError(t *testing.T) {
	repo := &mockScanRepo{
		orphans:  []model.FirmwareScan{{ID: primitive.NewObjectID()}},
		stampErr: errRepo,
	}
	_, err := requeueOrphanedScheduled(context.Background(), repo, time.Minute)
	if err == nil {
		t.Error("expected error from StampRequeued")
	}
}

func TestRequeueOrphanedScheduled_ReturnsMessages(t *testing.T) {
	id := primitive.NewObjectID()
	repo := &mockScanRepo{
		orphans: []model.FirmwareScan{{ID: id, DeviceID: "dev-2"}},
	}
	msgs, err := requeueOrphanedScheduled(context.Background(), repo, time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].DeviceID != "dev-2" {
		t.Errorf("unexpected DeviceID: %s", msgs[0].DeviceID)
	}
}