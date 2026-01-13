#!/bin/bash
# =============================================================================
# SecureConnect Database Initialization Script
# =============================================================================
# This script initializes all required databases and creates schemas
# Usage: ./init-db.sh

set -e  # Exit on error

echo "🚀 Starting SecureConnect Database Initialization..."

# --- Configuration ---
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-26257}"
DB_USER="${DB_USER:-root}"
DB_NAME="${DB_NAME:-secureconnect}"
CASSANDRA_HOST="${CASSANDRA_HOSTS:-localhost}"
CASSANDRA_KEYSPACE="${CASSANDRA_KEYSPACE:-secureconnect}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# --- 1. Initialize CockroachDB ---
echo -e "${YELLOW}📊 Initializing CockroachDB...${NC}"

# Wait for CockroachDB to be ready
echo "Waiting for CockroachDB to be ready..."
until cockroach sql --host=$DB_HOST:$DB_PORT --insecure -e "SELECT 1" > /dev/null 2>&1; do
    echo "CockroachDB is unavailable - sleeping"
    sleep 2
done

echo -e "${GREEN}✅ CockroachDB is ready${NC}"

# Create database if not exists
echo "Creating database: $DB_NAME"
cockroach sql --host=$DB_HOST:$DB_PORT --insecure <<EOF
CREATE DATABASE IF NOT EXISTS $DB_NAME;
EOF

# Run main schema
echo "Executing main schema (cockroach-init.sql)..."
cockroach sql --host=$DB_HOST:$DB_PORT --insecure --database=$DB_NAME < ./cockroach-init.sql

# Run calls schema
echo "Executing calls schema (calls-schema.sql)..."
cockroach sql --host=$DB_HOST:$DB_PORT --insecure --database=$DB_NAME < ./calls-schema.sql

echo -e "${GREEN}✅ CockroachDB schema created successfully${NC}"

# --- 2. Initialize Cassandra ---
echo -e "${YELLOW}📊 Initializing Cassandra...${NC}"

# Wait for Cassandra to be ready
echo "Waiting for Cassandra to be ready..."
until cqlsh $CASSANDRA_HOST -e "DESC KEYSPACES" > /dev/null 2>&1; do
    echo "Cassandra is unavailable - sleeping"
    sleep 2
done

echo -e "${GREEN}✅ Cassandra is ready${NC}"

# Create keyspace and tables
echo "Creating Cassandra keyspace and tables..."
cqlsh $CASSANDRA_HOST < ./cassandra-schema.cql

echo -e "${GREEN}✅ Cassandra schema created successfully${NC}"

# --- 3. Verify Redis ---
echo -e "${YELLOW}📊 Verifying Redis connection...${NC}"
if redis-cli ping > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Redis is ready${NC}"
else
    echo -e "${RED}❌ Redis is not available${NC}"
    exit 1
fi

# --- 4. Verify MinIO ---
echo -e "${YELLOW}📊 Verifying MinIO connection...${NC}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-localhost:9000}"
if curl -f http://$MINIO_ENDPOINT/minio/health/live > /dev/null 2>&1; then
    echo -e "${GREEN}✅ MinIO is ready${NC}"
else
    echo -e "${RED}❌ MinIO is not available${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}🎉 All databases initialized successfully!${NC}"
echo ""
echo "Next steps:"
echo "1. Review the created schemas"
echo "2. Start the microservices: make run-all"
echo "3. Check health endpoints: curl http://localhost:8080/health"
echo ""
