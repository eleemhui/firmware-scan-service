# cmd/

Contains the two runnable binaries. Each has its own `main.go` and is built into a separate Docker image.

| Directory | Binary | Docker image |
|-----------|--------|--------------|
| `api/` | `firmware_scan_service` | `Dockerfile.api` |
| `worker/` | `firmware_analysis_service` | `Dockerfile.worker` |

Both binaries share the same `internal/` packages and connect to the same MongoDB replica set and RabbitMQ instance.