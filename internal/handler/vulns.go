package handler

import (
	"encoding/json"
	"net/http"

	"firmware-scan-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type vulnsRequest struct {
	Vulns []string `json:"vulns"`
}

type vulnsResponse struct {
	Vulns []string `json:"vulns"`
}

// NewAddVulnsHandler returns a handler for PATCH /v1/findings/vulns.
func NewAddVulnsHandler(pool *pgxpool.Pool) http.HandlerFunc {
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

		vulns, err := service.AddVulns(r.Context(), pool, req.Vulns)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add vulns")
			return
		}

		ids := make([]string, len(vulns))
		for i, v := range vulns {
			ids[i] = v.CveID
		}
		writeJSON(w, http.StatusOK, vulnsResponse{Vulns: ids})
	}
}

// NewListVulnsHandler returns a handler for GET /v1/findings/vulns.
func NewListVulnsHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vulns, err := service.ListVulns(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list vulns")
			return
		}

		ids := make([]string, len(vulns))
		for i, v := range vulns {
			ids[i] = v.CveID
		}
		writeJSON(w, http.StatusOK, vulnsResponse{Vulns: ids})
	}
}
