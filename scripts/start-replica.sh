#!/bin/bash
set -e

PGDATA=${PGDATA:-/var/lib/postgresql/data}

# Wait until the primary is accepting connections.
until pg_isready -h postgres-primary -p 5432 -U firmware; do
    echo "replica: waiting for primary..."
    sleep 2
done

# Only run pg_basebackup if this is a fresh volume.
if [ ! -f "$PGDATA/PG_VERSION" ]; then
    echo "replica: running pg_basebackup from primary..."
    PGPASSWORD=replicator_pass pg_basebackup \
        -h postgres-primary \
        -p 5432 \
        -U replicator \
        -D "$PGDATA" \
        -Xs -R -P   # -R writes standby.signal + primary_conninfo automatically
    echo "replica: base backup complete"
fi

exec postgres -D "$PGDATA"
