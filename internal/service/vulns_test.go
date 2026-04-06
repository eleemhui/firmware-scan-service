package service

import (
	"context"
	"testing"

	"firmware-scan-service/internal/model"
)

func TestAddVulns_Success_ReturnsList(t *testing.T) {
	repo := &mockVulnRepo{
		vulns: []model.Vulnerability{{CveID: "CVE-001"}, {CveID: "CVE-002"}},
	}
	vulns, err := addVulns(context.Background(), repo, []string{"CVE-001", "CVE-002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vulns) != 2 {
		t.Errorf("expected 2 vulns, got %d", len(vulns))
	}
}

func TestAddVulns_UpsertError_ReturnsError(t *testing.T) {
	repo := &mockVulnRepo{upsertErr: errRepo}
	_, err := addVulns(context.Background(), repo, []string{"CVE-001"})
	if err == nil {
		t.Error("expected error when upsert fails")
	}
}

func TestAddVulns_ListError_ReturnsError(t *testing.T) {
	repo := &mockVulnRepo{listErr: errRepo}
	_, err := addVulns(context.Background(), repo, []string{"CVE-001"})
	if err == nil {
		t.Error("expected error when list fails")
	}
}

func TestAddVulns_EmptyList_ReturnsAll(t *testing.T) {
	repo := &mockVulnRepo{
		vulns: []model.Vulnerability{{CveID: "CVE-001"}},
	}
	vulns, err := addVulns(context.Background(), repo, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vulns) != 1 {
		t.Errorf("expected 1 vuln, got %d", len(vulns))
	}
}