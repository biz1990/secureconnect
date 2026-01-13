#!/bin/bash
# =============================================================================
# SecureConnect Backend - Database Restore Script
# =============================================================================
# This script restores databases from backup files
#
# Usage: ./scripts/restore-databases.sh [backup_file]
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check arguments
if [ $# -eq 0 ]; then
    echo -e "${RED}Error: Backup file not specified${NC}"
    echo ""
    echo "Usage: $0 [backup_file]"
    echo ""
    echo "Available backups:"
    ls -lh ./backups/ 2>/dev/null || echo "No backups found"
    exit 1
fi

BACKUP_FILE="$1"

if [ ! -f "$BACKUP_FILE" ]; then
    echo -e "${RED}Error: Backup file not found: $BACKUP_FILE${NC}"
    exit 1
fi

echo -e "${GREEN}=== SecureConnect Backend - Database Restore ===${NC}"
echo -e "${BLUE}Backup File: $BACKUP_FILE${NC}"
echo ""
echo -e "${RED}WARNING: This will replace existing data!${NC}"
read -p "Are you sure? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Restore cancelled"
    exit 0
fi

# =============================================================================
# DETECT BACKUP TYPE
# =============================================================================

BASENAME=$(basename "$BACKUP_FILE")

if [[ "$BASENAME" == cockroachdb_*.sql.gz ]]; then
    RESTORE_TYPE="cockroachdb"
elif [[ "$BASENAME" == cockroachdb_*.sql ]]; then
    RESTORE_TYPE="cockroachdb"
elif [[ "$BASENAME" == cassandra_*.tar.gz ]]; then
    RESTORE_TYPE="cassandra"
elif [[ "$BASENAME" == redis_*.rdb.gz ]]; then
    RESTORE_TYPE="redis"
elif [[ "$BASENAME" == redis_*.rdb ]]; then
    RESTORE_TYPE="redis"
elif [[ "$BASENAME" == minio_*.tar.gz ]]; then
    RESTORE_TYPE="minio"
else
    echo -e "${RED}Error: Unknown backup type${NC}"
    exit 1
fi

# =============================================================================
# RESTORE COCKROACHDB
# =============================================================================

if [ "$RESTORE_TYPE" = "cockroachdb" ]; then
    echo -e "${YELLOW}Restoring CockroachDB...${NC}"
    
    COCKROACH_CONTAINER=$(docker ps -q -f name=secureconnect_crdb)
    
    if [ -z "$COCKROACH_CONTAINER" ]; then
        echo -e "${RED}Error: CockroachDB container not found${NC}"
        exit 1
    fi
    
    # Decompress if needed
    TEMP_FILE="$BACKUP_FILE"
    if [[ "$BACKUP_FILE" == *.gz ]]; then
        TEMP_FILE="/tmp/restore_cockroachdb_$$.sql"
        gunzip -c "$BACKUP_FILE" > "$TEMP_FILE"
    fi
    
    # Restore database
    docker exec -i "$COCKROACH_CONTAINER" ./cockroach sql --insecure < "$TEMP_FILE"
    
    # Cleanup
    if [ -f "$TEMP_FILE" ] && [ "$TEMP_FILE" != "$BACKUP_FILE" ]; then
        rm -f "$TEMP_FILE"
    fi
    
    echo -e "${GREEN}✓ CockroachDB restored successfully${NC}"
fi

# =============================================================================
# RESTORE CASSANDRA
# =============================================================================

if [ "$RESTORE_TYPE" = "cassandra" ]; then
    echo -e "${YELLOW}Restoring Cassandra...${NC}"
    
    CASSANDRA_CONTAINER=$(docker ps -q -f name=secureconnect_cassandra)
    
    if [ -z "$CASSANDRA_CONTAINER" ]; then
        echo -e "${RED}Error: Cassandra container not found${NC}"
        exit 1
    fi
    
    # Stop Cassandra
    echo "Stopping Cassandra..."
    docker stop "$CASSANDRA_CONTAINER"
    
    # Backup existing data
    docker exec "$CASSANDRA_CONTAINER" tar -czf /tmp/backup_before_restore.tar.gz -C /var/lib/cassandra/data . || true
    
    # Clear existing data
    docker exec "$CASSANDRA_CONTAINER" rm -rf /var/lib/cassandra/data/* || true
    
    # Extract backup
    echo "Extracting backup..."
    gunzip -c "$BACKUP_FILE" | docker exec -i "$CASSANDRA_CONTAINER" tar -xzf - -C /var/lib/cassandra/data
    
    # Start Cassandra
    echo "Starting Cassandra..."
    docker start "$CASSANDRA_CONTAINER"
    
    # Wait for Cassandra to be ready
    echo "Waiting for Cassandra to be ready..."
    for i in {1..60}; do
        docker exec "$CASSANDRA_CONTAINER" cqlsh -e "describe cluster" > /dev/null 2>&1 && break
        echo -n "."
        sleep 2
    done
    echo ""
    
    echo -e "${GREEN}✓ Cassandra restored successfully${NC}"
fi

# =============================================================================
# RESTORE REDIS
# =============================================================================

if [ "$RESTORE_TYPE" = "redis" ]; then
    echo -e "${YELLOW}Restoring Redis...${NC}"
    
    REDIS_CONTAINER=$(docker ps -q -f name=secureconnect_redis)
    
    if [ -z "$REDIS_CONTAINER" ]; then
        echo -e "${RED}Error: Redis container not found${NC}"
        exit 1
    fi
    
    # Decompress if needed
    TEMP_FILE="$BACKUP_FILE"
    if [[ "$BACKUP_FILE" == *.gz ]]; then
        TEMP_FILE="/tmp/restore_redis_$$.rdb"
        gunzip -c "$BACKUP_FILE" > "$TEMP_FILE"
    fi
    
    # Stop Redis
    echo "Stopping Redis..."
    docker stop "$REDIS_CONTAINER"
    
    # Backup existing data
    docker cp "$REDIS_CONTAINER:/data/dump.rdb" "/tmp/redis_backup_before_restore.rdb" 2>/dev/null || true
    
    # Copy backup file
    docker cp "$TEMP_FILE" "$REDIS_CONTAINER:/data/dump.rdb"
    
    # Start Redis
    echo "Starting Redis..."
    docker start "$REDIS_CONTAINER"
    
    # Wait for Redis to be ready
    echo "Waiting for Redis to be ready..."
    for i in {1..30}; do
        docker exec "$REDIS_CONTAINER" redis-cli PING > /dev/null 2>&1 && break
        echo -n "."
        sleep 1
    done
    echo ""
    
    # Cleanup
    if [ -f "$TEMP_FILE" ] && [ "$TEMP_FILE" != "$BACKUP_FILE" ]; then
        rm -f "$TEMP_FILE"
    fi
    
    echo -e "${GREEN}✓ Redis restored successfully${NC}"
fi

# =============================================================================
# RESTORE MINIO
# =============================================================================

if [ "$RESTORE_TYPE" = "minio" ]; then
    echo -e "${YELLOW}Restoring MinIO...${NC}"
    
    MINIO_CONTAINER=$(docker ps -q -f name=secureconnect_minio)
    
    if [ -z "$MINIO_CONTAINER" ]; then
        echo -e "${RED}Error: MinIO container not found${NC}"
        exit 1
    fi
    
    # Stop MinIO
    echo "Stopping MinIO..."
    docker stop "$MINIO_CONTAINER"
    
    # Backup existing data
    docker exec "$MINIO_CONTAINER" tar -czf /tmp/backup_before_restore.tar.gz -C /data . || true
    
    # Clear existing data
    docker exec "$MINIO_CONTAINER" rm -rf /data/* || true
    
    # Extract backup
    echo "Extracting backup..."
    gunzip -c "$BACKUP_FILE" | docker exec -i "$MINIO_CONTAINER" tar -xzf - -C /data
    
    # Start MinIO
    echo "Starting MinIO..."
    docker start "$MINIO_CONTAINER"
    
    # Wait for MinIO to be ready
    echo "Waiting for MinIO to be ready..."
    for i in {1..30}; do
        curl -f http://localhost:9000/minio/health/live > /dev/null 2>&1 && break
        echo -n "."
        sleep 2
    done
    echo ""
    
    echo -e "${GREEN}✓ MinIO restored successfully${NC}"
fi

# =============================================================================
# SUMMARY
# =============================================================================

echo ""
echo -e "${GREEN}=== Restore Complete ===${NC}"
echo ""
echo "Restored: $BACKUP_FILE"
echo ""
echo "Please verify your data and restart services if needed"
