ALTER TABLE firmware_scans
    ADD COLUMN IF NOT EXISTS scan_started_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS scan_completed_at TIMESTAMPTZ;
