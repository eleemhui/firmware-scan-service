CREATE TABLE IF NOT EXISTS firmware_scans (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id        TEXT        NOT NULL,
    firmware_version TEXT        NOT NULL,
    binary_hash      TEXT        NOT NULL,
    metadata         JSONB,
    status           TEXT        NOT NULL DEFAULT 'scheduled',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_firmware_scans_device_hash UNIQUE (device_id, binary_hash)
);
