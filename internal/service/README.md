# internal/service/

Business logic. All functions are pure (no global state) and take their dependencies as parameters.

## scan.go

### `RegisterScan(ctx, database, publisher, req) (scan, isNew, error)`

Attempts `InsertOne` on `firmware_scans`. If MongoDB returns a duplicate key error (unique index on `device_id + binary_hash`), the existing document is fetched and returned with `isNew=false`. On success, publishes `{"scan_id": "...", "device_id": "..."}` to the queue.

### `ClaimScan(ctx, database, id) (bool, error)`

Atomically transitions a scan from `scheduled` → `started` using `FindOneAndUpdate`. Returns `false` if another worker already claimed it — callers must skip and ack in that case.

### `CompleteScan(ctx, database, id) error`

Sets status → `complete` and records `scan_completed_at`.

### `RecordVulnerabilities(ctx, database, scanID, deviceID, cveIDs) error`

1. Sets `detected_vulns` on the scan document
2. Calls `AddVulnsToRegistry` to upsert each CVE into the `vulnerabilities` collection

### `AddVulnsToRegistry(ctx, database, cveIDs, deviceID) error`

Upserts one document per CVE ID into `vulnerabilities`:
- If `deviceID` is non-empty: `$addToSet` the device ID into `device_ids`
- If `deviceID` is empty (API-originated call): `$setOnInsert` initialises `device_ids: []` only on new documents

### `RequeueStaleScan(ctx, database, staleAfter) ([]string, error)`

Finds all scans in `started` status with `updated_at` older than `staleAfter`, resets them to `scheduled`, and returns their IDs for re-publishing to the queue.

## vulns.go

### `AddVulns(ctx, database, cveIDs) ([]Vulnerability, error)`

Called by the HTTP handler. Delegates to `AddVulnsToRegistry` with an empty device ID, then returns the full updated list via `ListVulns`.

### `ListVulns(ctx, database) ([]Vulnerability, error)`

Returns all documents from the `vulnerabilities` collection sorted by CVE ID (`_id`).
