package handler

import (
	"encoding/json"
	"net/http"

	"firmware-scan-service/internal/service"

	"go.mongodb.org/mongo-driver/mongo"
)

type vulnsRequest struct {
	Vulns []string `json:"vulns"`
}

type vulnsResponse struct {
	Vulns []string `json:"vulns"`
}

// NewAddVulnsHandler returns a handler for PATCH /v1/findings/vulns.
func NewAddVulnsHandler(database *mongo.Database) http.HandlerFunc {
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

		ids, err := service.AddVulns(r.Context(), database, req.Vulns)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add vulns")
			return
		}
		writeJSON(w, http.StatusOK, vulnsResponse{Vulns: ids})
	}
}

// NewListVulnsHandler returns a handler for GET /v1/findings/vulns.
func NewListVulnsHandler(database *mongo.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, err := service.ListVulns(r.Context(), database)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list vulns")
			return
		}
		writeJSON(w, http.StatusOK, vulnsResponse{Vulns: ids})
	}
}
