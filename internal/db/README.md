# internal/db/

MongoDB client construction and index management.

## Functions

### `NewClient(ctx, uri) (*mongo.Client, error)`

Connects to MongoDB using the provided URI and verifies the connection with a ping. Returns an error if the connection or ping fails.

### `CreateIndexes(ctx, database) error`

Creates all required indexes on startup. Safe to call on every start — MongoDB is a no-op if an index already exists.

**Indexes created:**

| Collection | Keys | Options |
|------------|------|---------|
| `firmware_scans` | `{ device_id: 1, binary_hash: 1 }` | unique, name: `uq_device_hash` |

The `vulnerabilities` collection uses `_id` (the CVE ID) as its natural unique key — no additional index needed.
