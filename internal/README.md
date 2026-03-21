# internal/

Shared packages used by both the API and worker binaries. Nothing in here is intended for import outside this module.

| Package | Purpose |
|---------|---------|
| `config/` | Load and validate environment variables |
| `db/` | MongoDB client construction and index creation |
| `handler/` | HTTP request handlers (API only) |
| `model/` | Shared data types with BSON and JSON tags |
| `queue/` | RabbitMQ publisher and consumer wrappers |
| `service/` | Business logic — scan registration, status transitions, vulnerability recording |
