#!/bin/bash
# Start mongod and suppress INFO-level log lines ("s":"I").
# Only WARNING, ERROR, and FATAL messages reach docker logs.

# Ensure the data directory is owned by the mongodb user
# (mirrors what docker-entrypoint.sh does, which we are bypassing).
chown -R mongodb:mongodb /data/db

gosu mongodb mongod "$@" 2>&1 | grep --line-buffered -v '"s":"I"'

# Exit with mongod's exit code, not grep's.
exit "${PIPESTATUS[0]}"
