# Firmware Scan Service

A scalable firmware scan registration platform built in Go. Devices report their firmware via REST API; scans are deduplicated, persisted in MongoDB, and processed asynchronously by a worker that consumes from a RabbitMQ queue.

## Services

| Service | Description |
|---------|-------------|
| `api` (`cmd/api`) | REST API — accepts scan registrations and CVE reports |
| `worker` (`cmd/worker`) | Queue consumer — performs firmware analysis, detects vulnerabilities |
| `mongodb` | MongoDB 8 — persistent store for scans and vulnerabilities |
| `rabbitmq` | RabbitMQ 4.2.5 — job queue with management UI |

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose v2](https://docs.docker.com/compose/install/)
- `curl` or any HTTP client for testing

## Quick Start

```bash
cd firmware-scan-service

# Build and start all services
docker compose up --build

# Subsequent starts (if no rebuild needed)
docker compose up

# Run a quick load test
pip install aiohttp
python load_test.py --requests 100

# View one scan record
docker compose exec mongodb mongosh --quiet firmware_db --eval "
  db.firmware_scans.aggregate([{ \$limit: 1 },
    { \$project: {device_id: 1,firmware_version: 1,binary_hash: 1,status: 1,detected_vulns: 1,
        created_at: 1,updated_at: 1,scan_started_at: 1,scan_completed_at: 1,last_requeued_at: 1,
        metadata_bytes: { \$bsonSize: '\$metadata' }
    }}])"

# View detected vulnerabilities, CVEs
curl http://localhost:8080/v1/findings/vulns

# Check status counts of scan records
docker compose exec mongodb mongosh --quiet firmware_db --eval "
  var counts = {};
  ['scheduled','started','complete','failed'].forEach(s => counts[s] = 0);
  db.firmware_scans.aggregate([
    { \$group: { _id: '\$status', scan_count: { \$sum: 1 } } }
  ]).forEach(r => counts[r._id] = r.scan_count);
  Object.entries(counts).sort().forEach(([s,c]) => print(s + ': ' + c));
"
```

The API will be available at `http://localhost:8080` once healthy.
RabbitMQ management UI: `http://localhost:15672` (guest / guest)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MONGO_URI` | _(required)_ | MongoDB connection string |
| `MONGO_DB` | `firmware_db` | MongoDB database name |
| `AMQP_URL` | _(required)_ | RabbitMQ connection string |
| `QUEUE_NAME` | `firmware_scan_jobs` | Queue name for scan jobs |
| `PORT` | `8080` | API listen port (api only) |

## API Reference

### POST /v1/firmware-scans

Register a device's firmware and trigger an asynchronous scan.

```bash
curl -X POST http://localhost:8080/v1/firmware-scans \
  -H 'Content-Type: application/json' \
  -d '{
    "device_id": "device-123",
    "firmware_version": "2.4.1",
    "binary_hash": "ab34c9fdeadbeef",
    "metadata": {
      "hardware_model": "X1000",
      "components": []
    }
  }'
```

- **201 Created** — new scan registered, job enqueued
- **200 OK** — scan already registered for this `(device_id, binary_hash)` pair — idempotent retry
- **400 Bad Request** — missing required fields

---

### PATCH /v1/findings/vulns

Append CVE IDs to the vulnerability registry. Duplicates are ignored.

```bash
curl -X PATCH http://localhost:8080/v1/findings/vulns \
  -H 'Content-Type: application/json' \
  -d '{"vulns": ["CVE-001", "CVE-002"]}'
```

Returns the complete deduplicated list of CVE IDs after the call. CVEs submitted via this endpoint will have empty `first_detected_by` / `last_detected_by` fields since there is no scan context.

---

### GET /v1/findings/vulns

Return all CVEs in the system, sorted by CVE ID, including detection metadata.

```bash
curl http://localhost:8080/v1/findings/vulns
# → {"vulns":[
#     {"cve_id":"CVE-001","first_detected":"...","last_detected":"...","first_detected_by":"<scanID>","last_detected_by":"<scanID>","detected_count":3},
#     ...
#   ]}
```

---

## Scaling

Workers can be scaled easily without code changes.

```bash
# Run 3 workers consuming from the same queue in parallel
docker compose up --scale worker=3
```
---

## Architecture

For architectural decisions, design patterns, and scaling considerations see [architecture.md](architecture.md).


