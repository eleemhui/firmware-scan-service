# internal/handler/

HTTP request handlers for the REST API. Each handler is constructed with its dependencies injected and returned as an `http.HandlerFunc`.

## Handlers

### `NewScanHandler(database, publisher)`

`POST /v1/firmware-scans`

Decodes the request body, validates required fields (`device_id`, `firmware_version`, `binary_hash`), and delegates to `service.RegisterScan`.

- Returns `201` if the scan is new
- Returns `200` if the `(device_id, binary_hash)` pair already exists

### `NewAddVulnsHandler(database)`

`PATCH /v1/findings/vulns`

Accepts `{"vulns": ["CVE-001", ...]}` and delegates to `service.AddVulns`. Returns the full sorted list of CVE IDs in the registry.

### `NewListVulnsHandler(database)`

`GET /v1/findings/vulns`

Returns all CVE IDs in the `vulnerabilities` collection, sorted.

## Helpers (`helpers.go`)

- `writeJSON(w, status, v)` — marshal and write a JSON response
- `writeError(w, status, message)` — write a `{"error": "..."}` response
