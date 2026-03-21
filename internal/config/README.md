# internal/config/

Loads configuration from environment variables. Both the API and worker call `config.Load()` at startup.

## Variables

| Env var | Required | Default | Description |
|---------|----------|---------|-------------|
| `MONGO_URI` | yes | — | MongoDB connection URI (replica set format) |
| `MONGO_DB` | no | `firmware_db` | MongoDB database name |
| `AMQP_URL` | yes | — | RabbitMQ AMQP connection string |
| `QUEUE_NAME` | no | `firmware_scan_jobs` | Queue name for scan jobs |
| `PORT` | no | `8080` | HTTP listen port (API only) |

`Load()` returns an error if any required variable is missing.
