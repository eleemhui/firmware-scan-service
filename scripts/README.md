# scripts/

Shell scripts used by Docker Compose services.

## mongod-filter.sh

Wrapper entrypoint for `mongo-primary` and `mongo-secondary`. Replaces the default `docker-entrypoint.sh` to:

1. Set correct ownership on `/data/db`
2. Start `mongod` via `gosu mongodb` and pipe output through `grep`, suppressing `"s":"I"` (INFO) log lines — only WARNING, ERROR, and FATAL messages are shown in `docker compose logs`

## mongo-setup.sh

One-shot script run by the `mongo-setup` service after both MongoDB nodes are healthy. Initiates the replica set (`rs.initiate`) with `mongo-primary` as priority-2 primary and `mongo-secondary` as priority-1 secondary. Waits for a primary to be elected before exiting so that the API and worker containers only start once the replica set is fully ready.

## init-primary.sh / start-replica.sh

Legacy PostgreSQL replication scripts — no longer used. The system now uses MongoDB.
