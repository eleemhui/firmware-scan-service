# cmd/worker/

Entry point for the `firmware_analysis_service` queue consumer.

## Startup sequence

1. Load config from environment (`MONGO_URI`, `AMQP_URL`, etc.)
2. Connect to MongoDB (`internal/db`)
3. Connect to RabbitMQ consumer and publisher (`internal/queue`)
4. Start the watchdog goroutine
5. Begin consuming messages from `firmware_scan_jobs`

## Message processing (`processScan`)

Each RabbitMQ message carries `{"scan_id": "...", "device_id": "..."}`.

1. **Claim** — atomically transition `scheduled` → `started` via `FindOneAndUpdate`. If another worker already claimed it, skip and ack.
2. **Analyse** — simulate firmware analysis (`time.Sleep` 2–5 s).
3. **Vulnerability detection** — 1-in-10 chance of detecting 1–3 CVEs (CVE-001 to CVE-100). If detected:
   - Set `detected_vulns` on the scan document
   - Upsert each CVE into the `vulnerabilities` collection, adding `device_id` to its `device_ids` set
4. **Complete** — transition status → `complete`, set `scan_completed_at`
5. **Ack** — remove the message from the queue

If `processScan` returns an error the message is nacked with `requeue=false` and routed to the dead-letter queue.

## Scaling

Run multiple worker instances with `docker compose up --scale worker=N`. RabbitMQ delivers each message to exactly one consumer; `prefetch=1` distributes work evenly across all instances.
