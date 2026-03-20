# Firmware Scan Service

A scalable firmware scan registration platform built in Go. Devices report their firmware via REST API; scans are deduplicated, persisted in PostgreSQL, and processed asynchronously by a worker that consumes from a RabbitMQ queue.

## Services

| Service | Description |
|---------|-------------|
| `api` (`cmd/api`) | REST API — accepts scan registrations and CVE reports |
| `worker` (`cmd/worker`) | Queue consumer — performs (simulated) firmware analysis |
| `postgres-primary` | PostgreSQL 16 primary — source of truth |
| `postgres-replica` | PostgreSQL 16 streaming replica — demonstrates HA replication |
| `rabbitmq` | RabbitMQ 3 — job queue with management UI |

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose v2](https://docs.docker.com/compose/install/)
- `curl` or any HTTP client for testing

## Quick Start

```bash
# Clone / enter the directory
cd firmware_scan_service

# Build and start all services
docker compose up --build
```

The API will be available at `http://localhost:8080` once healthy.
RabbitMQ management UI: `http://localhost:15672` (guest / guest)

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
- **200 OK** — scan already registered (idempotent retry)
- **400 Bad Request** — missing required fields

Sending the exact same request again returns `200` with the existing record — safe for device retries on unreliable networks.

---

### PATCH /v1/findings/vulns

Append CVE IDs to the global vulnerability registry. Duplicates are ignored automatically.

```bash
curl -X PATCH http://localhost:8080/v1/findings/vulns \
  -H 'Content-Type: application/json' \
  -d '{"vulns": ["CVE-2024-001", "CVE-2024-002"]}'

curl -X PATCH http://localhost:8080/v1/findings/vulns \
  -H 'Content-Type: application/json' \
  -d '{"vulns": ["CVE-2024-002", "CVE-2024-003"]}'
```

Returns the complete deduplicated registry after each call.

---

### GET /v1/findings/vulns

Return all unique CVE IDs in the system.

```bash
curl http://localhost:8080/v1/findings/vulns
# → {"vulns":["CVE-2024-001","CVE-2024-002","CVE-2024-003"]}
```

---

## Scaling

```bash
# Run 3 API replicas behind Docker's built-in round-robin load balancer
docker compose up --scale api=3

# Run 3 workers consuming from the same queue in parallel
docker compose up --scale worker=3
```

Both are safe to scale without code changes — see the architecture notes below.

---

## Architecture Notes

### Overview

```
HTTP clients
    │
    ▼
firmware_scan_service (cmd/api)
    ├── PostgreSQL primary ◄── streaming replication ──► PostgreSQL replica
    └── RabbitMQ (firmware_scan_jobs queue)
              │
              ▼
    firmware_analysis_service (cmd/worker)
              │
              └── PostgreSQL primary (status updates)
```

### Duplicate and Repeated Request Handling

**Idempotency key:** `(device_id, binary_hash)`

The core mechanism is a single atomic SQL statement:

```sql
INSERT INTO firmware_scans (device_id, firmware_version, binary_hash, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (device_id, binary_hash) DO NOTHING
RETURNING id, ...
```

- If a row is returned → the scan is new. The API publishes one message to RabbitMQ and responds `201`.
- If no row is returned (conflict) → the scan already exists. The API fetches and returns the existing record with `200`. No message is published.

PostgreSQL's row-level locking during `INSERT` means that if 100 concurrent requests arrive for the same device, exactly one will insert the row and get it back via `RETURNING`. The rest see zero rows. No application-level distributed lock is needed.

Devices that retry due to network failures receive the same response shape (just `200` instead of `201`) and the scan is not processed twice.

### Asynchronous Processing

1. **API** publishes `{"scan_id": "<uuid>"}` to the durable `firmware_scan_jobs` queue after a successful insert.
2. **Worker** consumes messages one at a time (`prefetch=1`):
   - Updates status → `started`
   - Simulates analysis (`time.Sleep` 2–5 s)
   - Updates status → `complete`
   - Acks the message

Messages are `Persistent` (survive broker restart) and the queue is `durable`. Manual acknowledgement (`autoAck=false`) ensures a message is only removed from the queue once processing completes. If the worker crashes mid-scan, RabbitMQ redelivers the message to another worker.

Failed messages are nacked with `requeue=false`, routing them to a dead-letter queue rather than looping forever.

### Behaviour Under High Load

| Concern | Design response |
|---------|----------------|
| Burst of registrations | `POST /v1/firmware-scans` is lightweight: one DB write + one queue publish. pgx connection pool absorbs concurrency. |
| Slow analysis | Decoupled via queue — API returns immediately; worker processes at its own pace. Queue depth acts as a natural buffer. |
| Multiple API replicas | Deduplication is enforced by PostgreSQL, not the application. Any number of replicas can run safely behind a load balancer. |
| Multiple workers | RabbitMQ delivers each message to exactly one consumer. `prefetch=1` distributes work evenly. |
| Database failover | PostgreSQL replica streams all changes from the primary in real time. Promote the replica and update `DATABASE_URL` to recover. |

### Scaling to Significantly More Devices

| Change | Benefit |
|--------|---------|
| Promote PostgreSQL replica for reads | Offload `GET /v1/findings/vulns` and scan status reads |
| Replace classic RabbitMQ queue with a **quorum queue** | Durable across broker restarts, tolerate broker node failures |
| Horizontal API scaling | Stateless — add replicas freely |
| Horizontal worker scaling | Increases analysis throughput linearly |
| Partitioned message routing (Kafka) | For millions of devices, Kafka consumer groups allow per-partition parallelism with better throughput than AMQP |
| Separate read replica pool in the API | Route non-transactional reads to the replica DSN |

### Database Schema

**`firmware_scans`** — one row per unique `(device_id, binary_hash)` pair

```
id               UUID  PK
device_id        TEXT
firmware_version TEXT
binary_hash      TEXT
metadata         JSONB   -- arbitrary hardware/component data
status           TEXT    -- scheduled | started | complete | failed
created_at       TIMESTAMPTZ
updated_at       TIMESTAMPTZ
UNIQUE (device_id, binary_hash)
```

**`vulns`** — global CVE registry

```
id         SERIAL PK
cve_id     TEXT UNIQUE
created_at TIMESTAMPTZ
```
