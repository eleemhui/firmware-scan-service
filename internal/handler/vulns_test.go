package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"firmware-scan-service/internal/model"
)

func TestNewAddVulnsHandler_Returns200WithUpdatedList(t *testing.T) {
	svc := &mockVulnService{
		vulns: []model.Vulnerability{
			{CveID: "CVE-001"},
			{CveID: "CVE-002"},
		},
	}
	handler := NewAddVulnsHandler(svc)

	body, _ := json.Marshal(map[string][]string{"vulns": {"CVE-001", "CVE-002"}})
	req := httptest.NewRequest(http.MethodPatch, "/v1/findings/vulns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp vulnsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !reflect.DeepEqual(resp.Vulns, []string{"CVE-001", "CVE-002"}) {
		t.Errorf("unexpected vulns: %v", resp.Vulns)
	}
}

func TestNewAddVulnsHandler_EmptyList_Returns400(t *testing.T) {
	svc := &mockVulnService{}
	handler := NewAddVulnsHandler(svc)

	body, _ := json.Marshal(map[string][]string{"vulns": {}})
	req := httptest.NewRequest(http.MethodPatch, "/v1/findings/vulns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestNewAddVulnsHandler_InvalidJSON_Returns400(t *testing.T) {
	svc := &mockVulnService{}
	handler := NewAddVulnsHandler(svc)

	req := httptest.NewRequest(http.MethodPatch, "/v1/findings/vulns", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestNewAddVulnsHandler_ServiceError_Returns500(t *testing.T) {
	svc := &mockVulnService{err: errService}
	handler := NewAddVulnsHandler(svc)

	body, _ := json.Marshal(map[string][]string{"vulns": {"CVE-001"}})
	req := httptest.NewRequest(http.MethodPatch, "/v1/findings/vulns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestNewListVulnsHandler_ReturnsSortedList(t *testing.T) {
	svc := &mockVulnService{
		vulns: []model.Vulnerability{
			{CveID: "CVE-001"},
			{CveID: "CVE-042"},
		},
	}
	handler := NewListVulnsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/findings/vulns", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp vulnsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if !reflect.DeepEqual(resp.Vulns, []string{"CVE-001", "CVE-042"}) {
		t.Errorf("unexpected vulns: %v", resp.Vulns)
	}
}

func TestNewListVulnsHandler_ServiceError_Returns500(t *testing.T) {
	svc := &mockVulnService{err: errService}
	handler := NewListVulnsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/findings/vulns", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}
