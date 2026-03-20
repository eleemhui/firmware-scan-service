CREATE TABLE IF NOT EXISTS vulnerabilities (
    id         SERIAL      PRIMARY KEY,
    cve_id     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_vulnerabilities_cve_id UNIQUE (cve_id)
);
