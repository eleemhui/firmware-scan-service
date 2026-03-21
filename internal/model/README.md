# internal/model/

Shared data types used by both the API and worker.

## Types

### `FirmwareScan`

Stored in the `firmware_scans` MongoDB collection. One document per unique `(device_id, binary_hash)` pair.

| Field | BSON key | Description |
|-------|----------|-------------|
| `ID` | `_id` | UUID string, generated at registration |
| `DeviceID` | `device_id` | Reporting device identifier |
| `FirmwareVersion` | `firmware_version` | Version string |
| `BinaryHash` | `binary_hash` | Hash of the firmware binary — part of the idempotency key |
| `Metadata` | `metadata` | Arbitrary JSON object from the device |
| `Status` | `status` | `scheduled` → `started` → `complete` (or `failed`) |
| `DetectedVulns` | `detected_vulns` | CVE IDs found during analysis — omitted if none |
| `CreatedAt` | `created_at` | Document creation time |
| `UpdatedAt` | `updated_at` | Last modification time |
| `ScanStartedAt` | `scan_started_at` | When the worker claimed the scan |
| `ScanCompletedAt` | `scan_completed_at` | When the worker finished |

### `Vulnerability`

Stored in the `vulnerabilities` collection. One document per CVE ID.

| Field | BSON key | Description |
|-------|----------|-------------|
| `CveID` | `_id` | CVE identifier — unique by being the document `_id` |
| `DeviceIDs` | `device_ids` | Devices on which this CVE has been detected |

### `ScanJobMessage`

JSON payload published to the RabbitMQ queue.

| Field | Description |
|-------|-------------|
| `ScanID` | UUID of the scan document |
| `DeviceID` | Device that registered the scan — passed through so the worker can record it in vulnerability documents |

## Status constants

```
StatusScheduled = "scheduled"
StatusStarted   = "started"
StatusComplete  = "complete"
StatusFailed    = "failed"
```
