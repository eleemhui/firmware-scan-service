#!/bin/bash
# Initiates the MongoDB replica set and waits for a primary to be elected.
# Runs once via the mongo-setup service in docker-compose.
set -e

echo "mongo-setup: waiting for mongo-primary..."
until mongosh --host mongo-primary --quiet --eval "db.adminCommand('ping').ok" 2>/dev/null; do
    sleep 2
done

echo "mongo-setup: waiting for mongo-secondary..."
until mongosh --host mongo-secondary --quiet --eval "db.adminCommand('ping').ok" 2>/dev/null; do
    sleep 2
done

mongosh --host mongo-primary --quiet --eval "
  try {
    rs.status();
    print('mongo-setup: replica set already initialised');
  } catch(e) {
    print('mongo-setup: initiating replica set...');
    rs.initiate({
      _id: 'rs0',
      members: [
        { _id: 0, host: 'mongo-primary:27017',   priority: 2 },
        { _id: 1, host: 'mongo-secondary:27017', priority: 1 }
      ]
    });
  }

  // Wait for primary election (up to 30s).
  var attempts = 0;
  while (rs.status().myState !== 1 && attempts < 30) {
    sleep(1000);
    attempts++;
  }
  if (rs.status().myState !== 1) {
    print('mongo-setup: primary election timed out');
    quit(1);
  }
  print('mongo-setup: primary elected, replica set ready');
"
