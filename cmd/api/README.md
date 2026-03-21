# cmd/api/

Entry point for the `firmware_scan_service` REST API.

## Startup sequence

1. Load config from environment (`MONGO_URI`, `AMQP_URL`, etc.)
2. Connect to MongoDB and create indexes (`internal/db`)
3. Connect to RabbitMQ publisher (`internal/queue`)
4. Register HTTP routes and start listening on `PORT` (default `8080`)

## Routes

| Method | Path | Handler |
|--------|------|---------|
| `POST` | `/v1/firmware-scans` | `handler.NewScanHandler` |
| `PATCH` | `/v1/findings/vulns` | `handler.NewAddVulnsHandler` |
| `GET` | `/v1/findings/vulns` | `handler.NewListVulnsHandler` |

## Concurrency

Multiple API replicas can run simultaneously. Deduplication is enforced by a unique MongoDB index on `(device_id, binary_hash)`, not by the application — so any number of replicas are safe behind a load balancer.