package handler

import (
	"encoding/json"
	"net/http"

	"firmware-scan-service/internal/model"
	"firmware-scan-service/internal/service"
)

type vulnsRequest struct {
	Vulns []string `json:"vulns"`
}

type vulnsResponse struct {
	Vulns []string `json:"vulns"`
}

func cveIDs(vulns []model.Vulnerability) []string {
	ids := make([]string, len(vulns))
	for i, v := range vulns {
		ids[i] = v.CveID
	}
	return ids
}

// NewAddVulnsHandler returns a handler for PATCH /v1/findings/vulns.
func NewAddVulnsHandler(svc service.VulnManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req vulnsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.Vulns) == 0 {
			writeError(w, http.StatusBadRequest, "vulns must be a non-empty array")
			return
		}

		vulns, err := svc.AddVulns(r.Context(), req.Vulns)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add vulns")
			return
		}
		writeJSON(w, http.StatusOK, vulnsResponse{Vulns: cveIDs(vulns)})
	}
}

// NewListVulnsHandler returns a handler for GET /v1/findings/vulns.
func NewListVulnsHandler(svc service.VulnManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vulns, err := svc.ListVulns(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list vulns")
			return
		}
		writeJSON(w, http.StatusOK, vulnsResponse{Vulns: cveIDs(vulns)})
	}
}
