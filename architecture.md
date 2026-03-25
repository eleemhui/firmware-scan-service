## System Overview

```
                  ┌─────────────────────────────────────────┐
                  │         firmware_scan_service           │
  HTTP Clients    │                                         │
  (concurrent)    │  ┌──────────┐      ┌─────────────────┐  │
       │          │  │ REST API │      │    Watchdog     │  │
       │  POST    │  │ :8080    │      │ (goroutine)     │  │
       └─────────►│  │          │      │                 │  │
                  │  └────┬─────┘      └────────┬────────┘  │
                  └───────┼─────────────────────┼───────────┘
               ┌──────────┘          ┌──────────┘
               │ INSERT              │ Re-enqueue
               │ (idempotent)        │ stale/orphan
               │                     ┼──────────────────────────┐
               │                     │                          ▼
               │                     │              ┌───────────────────────┐
               │                     │              │       MongoDB         │
               │                     │              │                       │
               ┼─────────────────────┼─────────────►│  ┌─────────────────┐  │
               │                     │              │  │ firmware_scans  │  │
               │ PUBLISH (on new)    │ REPUBLISH    │  └─────────────────┘  │
               │                     │ (stale/      │                       │
               ▼                     ▼  orphan)     │  ┌─────────────────┐  │
             ┌──────────────────────────┐           │  │ vulnerabilities │  │
             │       RabbitMQ           │           │  └─────────────────┘  │
             │  firmware_scan_jobs      │           └───────────────────────┘
             └──────────────────────────┘                      ▲
                       │                                       │
         ┌─────────────┼─────────────┐                         │
         │ CONSUME     │ CONSUME     │ CONSUME                 │
         ▼             ▼             ▼                         │
  ┌────────────┐ ┌────────────┐ ┌────────────┐                 │
  │  worker-1  │ │  worker-2  │ │  worker-3  │                 │
  │            │ │            │ │            │                 │
  │ ClaimScan  │ │ ClaimScan  │ │ ClaimScan  │                 │
  │ Scan Frmwr │ │ Scan Frmwr │ │ Scan Frmwr │                 │
  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘                 │
        └──────────────┼──────────────┘                        │
                       │ UPDATE status & vulnerabilities       │
                       └───────────────────────────────────────┘
```

## Main Architectural Decisions

### 1. MongoDB over a relational database
MongoDB was chosen for its flexible document model, which accommodates variable-size firmware metadata without schema migrations. It supports atomic inserts and updates that enable distributed deduplication without application-level locking. When scaling is needed, MongoDB offers built-in replica sets and native sharding as a straightforward upgrade path.

### 2. RabbitMQ over Kafka
RabbitMQ was chosen for simplicity and light-weight deployment. Performing firmware scans fit RabbitMQ's work-queue model using a round-robin, `prefetch=1`, and manual ack so that only one worker will process one scan. By setting `DeliveryMode: Persistent` most queued messages can survive broker restarts.

### 3. Separate API and worker binaries
The REST API and the analysis worker are compiled and deployed as independent processes. This allows each to scale independently: more API replicas to handle HTTP concurrency (if placed behind a load balancer), and more workers to drain the queue faster and perform scans.

### 4. Watchdog in the API
The Watchdog is a background thread (goroutine) that runs in the API process for design simplicity. The Watchdog will cover certain scenarios like orphaned/stale scans that can occur if a worker dies or the message queue fails and messages are lost. After a timeout period has passed, stale/orphaned scan jobs will be requeued. 

### 5. UUID strings as MongoDB `_id`
For better performance I used UUID strings generated in go for `_id`. This avoids a MongoDB round-trip when the document is inserted. In addition the firmware_scans collection will enforce uniqueness for the combination of `ObjectID`-`BinaryHash`.


### Duplicate and Repeated Request Handling

**Idempotency key:** `(device_id, binary_hash)`

`RegisterScan` calls `InsertOne` with a unique compound index on `(device_id, binary_hash)`. If MongoDB returns a duplicate key error, the existing document is fetched and returned to the caller. Exactly one queue message is ever published per unique pair.

- New scan → `201 Created` + message enqueued
- Duplicate → `200 OK` + existing record returned, no message published

The worker additionally guards against duplicate processing:

```
ClaimScan:
  FindOneAndUpdate(
    filter: { _id: scanID, status: "scheduled" },  ← conditional
    update: { $set: { status: "started" } }
  )

  If no document matched → scan was already claimed by another worker → skip
```

This means even if RabbitMQ delivers the same message twice (at-least-once), the second delivery is a no-op.

---

## How Asynchronous Processing Is Coordinated

```
  1. API receives POST
        │
        ▼
  2. MongoDB INSERT  ──► status: "scheduled"
        │
        ▼
  3. Publish to RabbitMQ (only on successful new insert)
        │
        ▼
  4. Worker receives message (prefetch=1 — one in-flight per worker)
        │
        ▼
  5. ClaimScan: atomic FindOneAndUpdate  ──► status: "started" only if status = "scheduled"
        │
        ▼
  6. Simulate / perform analysis (`time.Sleep` 2–5 s)
        │
        ├──► (10% chance) RecordVulnerabilities
        │         writes detected_vulns to vulnerabilities collection (CVE-001 to CVE-100)
        │     (1% chance) Worker "crashes" and never sets status to "complete"
        ▼
  7. CompleteScan  ──► status: "complete"
        │
        ▼
  8. ack message — RabbitMQ discards it

  Failure paths:
    Worker crash (os.Exit)      → TCP close → RabbitMQ redelivers (unacked)
    Scan stuck in "started"     → Watchdog resets to "scheduled" after staleThreshold
    Message lost from queue     → Watchdog re-publishes after orphanedThreshold
```

Status transitions are strictly `scheduled → started → complete`. The `ClaimScan` condition prevents backwards transitions and race conditions. Only the watchdog can reschedule a scan by setting it back to `scheduled` and updating `last_requeued_at`. 

---


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
_id               string    CVE ID (e.g. "CVE-042") — unique by being the _id
first_detected    date      timestamp of first detection
last_detected     date      timestamp of most recent detection
first_detected_by string    scan ID that first detected this CVE
last_detected_by  string    scan ID that most recently detected this CVE
detected_count    int64     total number of times this CVE has been detected
```

The CVE ID is used as `_id`, enforcing uniqueness via MongoDB's built-in primary key index. Each detection performs a single atomic upsert using `$setOnInsert` (first_detected, first_detected_by), `$set` (last_detected, last_detected_by), and `$inc` (detected_count). When submitted via the HTTP API with no scan context, `first_detected_by` and `last_detected_by` are set to an empty string.

## Behaviour Under High Load

| Component | Behaviour |
|-----------|-----------|
| **API** | Stateless — scale horizontally with `--scale api=N`. Each replica independently inserts and publishes. MongoDB's unique index serialises duplicate requests without API coordination. Requires a load-balancer. |
| **RabbitMQ** | Queue depth grows under burst load. Workers drain it at their own pace. `prefetch=1` ensures no single worker is overwhelmed. |
| **Workers** | Scale with `--scale worker=N` (default: 3). Each worker processes one message at a time. More workers = higher parallel throughput of scans. |
| **MongoDB** | Single node handles all reads and writes. Under sustained high load, adding a replica set and routing GET endpoints to a secondary is the first scaling step. |
| **Watchdog** | Runs once per 1–5 minute random interval — negligible overhead regardless of load. |

The two weakest links under extreme load is the MongoDB primary for writes and RabbitMQ can grow without limits. See scaling section below.

---

## What Changes Would Be Needed for Significantly More Devices

### Near-term (10× current load)

- **Add a MongoDB replica set** — promote the single node to a primary with one or more secondaries. Route read-only GET endpoints to a secondary connection pool to offload the primary.
- **Increase worker replicas** — raise `deploy.replicas` in `docker-compose.yml` or use the Kubernetes `Deployment` replica count.
- **RabbitMQ quorum queues** — replace the classic queue with a quorum queue (`x-queue-type: quorum`) for stronger HA guarantees across a multi-node RabbitMQ cluster. Use flow control to apply back-pressure on producers.

### Medium-term (100× current load)

- **MongoDB sharding** — shard the `firmware_scans` collection on `device_id`. Queries and writes for the same device route to the same shard, keeping the unique index local to one shard.
- **Horizontal RabbitMQ cluster** — run a 3-node RabbitMQ cluster with quorum queues mirrored across all nodes.


### Long-term (fleet-scale, millions of devices)

- **Kafka instead of RabbitMQ** — partition `firmware_scan_jobs` by `device_id` for ordered, replayable processing per device. Consumer groups map to worker pools.
- **API Gateway + rate limiting** — fronting the API with a gateway adds rate limiting, authentication, and TLS termination before requests reach the Go service.
- **Kubernetes Horizontal Pod Autoscaler (HPA)** — replace fixed replica counts with Horizontal Pod Autoscalers (HPA) that scales workers based on load or queue depth.