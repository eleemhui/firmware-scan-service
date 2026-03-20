#!/bin/bash
set -e

# Create replication user.
# This script runs once inside the postgres container's
# /docker-entrypoint-initdb.d/ directory when PGDATA is first initialised.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'replicator_pass';
EOSQL

# Enable streaming replication on the primary.
cat >> "$PGDATA/postgresql.conf" <<-EOF

# Streaming replication settings (appended by init-primary.sh)
wal_level = replica
max_wal_senders = 5
wal_keep_size = 64
EOF

# Allow the replicator user to connect from any host in the Docker network.
echo "host replication replicator all md5" >> "$PGDATA/pg_hba.conf"
