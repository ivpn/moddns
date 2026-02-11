#!/bin/bash
# MongoDB Replica Set Initialization Script

set -e

echo "Waiting for MongoDB instances to be ready..."
sleep 10

echo "Initiating replica set..."
mongosh --host mongodb-primary:27017 -u admin -p admin --authenticationDatabase admin <<EOFMONGO
rs.initiate({
  _id: "rs0",
  members: [
    { _id: 0, host: "mongodb-primary:27017", priority: 2 },
    { _id: 1, host: "mongodb-secondary1:27017", priority: 1 },
    { _id: 2, host: "mongodb-secondary2:27017", priority: 1 }
  ]
});
EOFMONGO

echo "Waiting for replica set to stabilize..."
sleep 15

echo "Checking replica set status..."
mongosh --host mongodb-primary:27017 -u admin -p admin --authenticationDatabase admin --eval "rs.status()"

echo "Creating database and users..."
mongosh --host mongodb-primary:27017 -u admin -p admin --authenticationDatabase admin <<EOFMONGO
use dns;
db.createUser({
  user: "admin",
  pwd: "admin",
  roles: [
    { role: "readWrite", db: "dns" },
    { role: "dbAdmin", db: "dns" }
  ]
});
EOFMONGO

echo "MongoDB replica set initialization complete!"
