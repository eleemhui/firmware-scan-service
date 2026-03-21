# Firmware Scan Service

A scalable firmware scan registration platform built in Go. Devices report their firmware via REST API; scans are deduplicated, persisted in MongoDB, and processed asynchronously by a worker that consumes from a RabbitMQ queue.

## Services

| Service | Description |
|---------|-------------|
| `api` (`cmd/api`) | REST API — accepts scan registrations and CVE reports |
| `worker` (`cmd/worker`) | Queue consumer — performs firmware analysis, detects vulnerabilities |
| `mongo-primary` | MongoDB 7 primary — source of truth |
| `mongo-secondary` | MongoDB 7 secondary — streaming replica set member |
| `mongo-setup` | One-shot container that initiates the replica set, then exits |
| `rabbitmq` | RabbitMQ 3 — job queue with management UI |

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose v2](https://docs.docker.com/compose/install/)
- `curl` or any HTTP client for testing

## Quick Start

```bash
cd firmware-scan-service

# Build and start all services
docker compose up --build

# Subsequent starts (no rebuild needed)
docker compose up
```

The API will be available at `http://localhost:8080` once healthy.
RabbitMQ management UI: `http://localhost:15672` (guest / guest)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MONGO_URI` | _(required)_ | MongoDB connection string (replica set URI) |
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

Returns the complete deduplicated list of CVE IDs after the call.

---

### GET /v1/findings/vulns

Return all unique CVE IDs in the system, sorted.

```bash
curl http://localhost:8080/v1/findings/vulns
# → {"vulns":["CVE-001","CVE-002","CVE-042"]}
```

---

## Scaling

```bash
# Run 3 API replicas behind Docker's built-in round-robin load balancer
docker compose up --scale api=3

# Run 3 workers consuming from the same queue in parallel
docker compose up --scale worker=3
```

Both are safe to scale without code changes.

---

## Architecture

```
HTTP clients
    │
    ▼
firmware_scan_service (cmd/api)
    ├── MongoDB replica set (primary + secondary)
    └── RabbitMQ (firmware_scan_jobs queue)
              │
              ▼
    firmware_analysis_service (cmd/worker)
              │
              └── MongoDB primary (status + vulnerability updates)
```

### Duplicate and Repeated Request Handling

**Idempotency key:** `(device_id, binary_hash)`

`RegisterScan` calls `InsertOne` with a unique compound index on `(device_id, binary_hash)`. If MongoDB returns a duplicate key error, the existing document is fetched and returned to the caller. Exactly one queue message is ever published per unique pair.

- New scan → `201 Created` + message enqueued
- Duplicate → `200 OK` + existing record returned, no message published

### Asynchronous Processing

1. **API** publishes `{"scan_id": "<uuid>", "device_id": "<id>"}` to the durable `firmware_scan_jobs` queue.
2. **Worker** consumes messages with `prefetch=1`:
   - Atomically transitions status `scheduled` → `started` via `FindOneAndUpdate`
   - Simulates analysis (`time.Sleep` 2–5 s)
   - With 1-in-10 probability, detects 1–3 vulnerabilities (CVE-001 to CVE-100)
   - If vulnerabilities are detected: updates `firmware_scans.detected_vulns` and upserts each CVE into the `vulnerabilities` collection
   - Transitions status → `complete`
   - Acks the message

Messages are `Persistent` and the queue is `durable`. If the worker crashes mid-scan, RabbitMQ redelivers the message. A watchdog goroutine resets scans stuck in `started` for more than 5 minutes back to `scheduled`.

### MongoDB Collections

**`firmware_scans`** — one document per unique `(device_id, binary_hash)` pair

```
_id              string   UUID
device_id        string
firmware_version string
binary_hash      string
metadata         object   arbitrary hardware/component data
status           string   scheduled | started | complete | failed
detected_vulns   []string CVE IDs found during this scan (omitted if none)
created_at       date
updated_at       date
scan_started_at  date     set when worker claims the scan
scan_completed_at date    set when worker completes the scan

Index: { device_id: 1, binary_hash: 1 }  unique
```

**`vulnerabilities`** — one document per CVE ID

```
_id        string   CVE ID (e.g. "CVE-042")  — unique by being the _id
device_ids []string devices on which this CVE has been detected
```

### Behaviour Under Load

| Concern | Design response |
|---------|----------------|
| Burst of registrations | `POST /v1/firmware-scans` is one DB write + one queue publish — lightweight |
| Slow analysis | Decoupled via queue — API returns immediately; workers process at their own pace |
| Multiple API replicas | Deduplication enforced by MongoDB index, not the application — any number of replicas are safe |
| Multiple workers | RabbitMQ delivers each message to exactly one consumer; `prefetch=1` distributes evenly |
| Database failover | MongoDB replica set streams all changes in real time; promote secondary and update `MONGO_URI` to recover |

### Scaling Further

| Change | Benefit |
|--------|---------|
| Point reads at the secondary | Offload `GET /v1/findings/vulns` and scan status reads from the primary |
| Replace classic RabbitMQ queue with a quorum queue | Durable across broker restarts, tolerates broker node failures |
| Horizontal API scaling | Stateless — add replicas freely |
| Horizontal worker scaling | Increases analysis throughput linearly |
| Kafka for very high volume | Consumer groups allow per-partition parallelism for millions of devices |
