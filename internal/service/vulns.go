package service

import (
	"context"
	"fmt"

	"firmware-scan-service/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AddVulns inserts CVE IDs into the registry, ignoring duplicates.
// Uses a pgx Batch to send all inserts in a single round-trip.
// PostgreSQL's UNIQUE constraint on cve_id serializes concurrent writes
// across all replicas, preventing race conditions and duplicates.
func AddVulns(ctx context.Context, pool *pgxpool.Pool, cveIDs []string) ([]model.Vuln, error) {
	if len(cveIDs) == 0 {
		return ListVulns(ctx, pool)
	}

	batch := &pgx.Batch{}
	for _, id := range cveIDs {
		batch.Queue(
			`INSERT INTO vulnerabilities (cve_id) VALUES ($1) ON CONFLICT (cve_id) DO NOTHING`,
			id,
		)
	}

	results := pool.SendBatch(ctx, batch)
	for range cveIDs {
		if _, err := results.Exec(); err != nil {
			results.Close()
			return nil, fmt.Errorf("insert vuln: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return nil, fmt.Errorf("close batch: %w", err)
	}

	return ListVulns(ctx, pool)
}

// ListVulns returns all unique CVE IDs ordered by insertion time.
func ListVulns(ctx context.Context, pool *pgxpool.Pool) ([]model.Vuln, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, cve_id, created_at FROM vulnerabilities ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query vulns: %w", err)
	}
	defer rows.Close()

	var vulns []model.Vuln
	for rows.Next() {
		var v model.Vuln
		if err := rows.Scan(&v.ID, &v.CveID, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vuln: %w", err)
		}
		vulns = append(vulns, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if vulns == nil {
		vulns = []model.Vuln{}
	}
	return vulns, nil
}
