#!/bin/bash
# =============================================================================
# SecureConnect Backend - Database Backup Script
# =============================================================================
# This script creates backups for CockroachDB, Cassandra, and Redis
#
# Usage: ./scripts/backup-databases.sh [backup_dir]
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BACKUP_DIR="${1:-./backups}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RETENTION_DAYS=7

# Create backup directory
mkdir -p "$BACKUP_DIR"

echo -e "${GREEN}=== SecureConnect Backend - Database Backup ===${NC}"
echo -e "${BLUE}Backup Directory: $BACKUP_DIR${NC}"
echo -e "${BLUE}Timestamp: $TIMESTAMP${NC}"
echo ""

# =============================================================================
# COCKROACHDB BACKUP
# =============================================================================

echo -e "${YELLOW}Backing up CockroachDB...${NC}"

COCKROACH_CONTAINER=$(docker ps -q -f name=secureconnect_crdb)

if [ -n "$COCKROACH_CONTAINER" ]; then
    BACKUP_FILE="$BACKUP_DIR/cockroachdb_$TIMESTAMP.sql"
    
    docker exec "$COCKROACH_CONTAINER" ./cockroach sql --insecure \
        -e "SHOW DATABASES;" > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        docker exec "$COCKROACH_CONTAINER" ./cockroach dump \
            --insecure \
            --dump-mode=schema \
            secureconnect_poc > "$BACKUP_FILE" 2>/dev/null || true
        
        if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
            gzip "$BACKUP_FILE"
            echo -e "${GREEN}✓ CockroachDB backup created: ${BACKUP_FILE}.gz${NC}"
        else
            echo -e "${RED}✗ CockroachDB backup failed${NC}"
        fi
    else
        echo -e "${RED}✗ CockroachDB not ready for backup${NC}"
    fi
else
    echo -e "${RED}✗ CockroachDB container not found${NC}"
fi

# =============================================================================
# CASSANDRA BACKUP
# =============================================================================

echo -e "${YELLOW}Backing up Cassandra...${NC}"

CASSANDRA_CONTAINER=$(docker ps -q -f name=secureconnect_cassandra)

if [ -n "$CASSANDRA_CONTAINER" ]; then
    BACKUP_FILE="$BACKUP_DIR/cassandra_$TIMESTAMP.tar.gz"
    
    # Check if Cassandra is ready
    docker exec "$CASSANDRA_CONTAINER" cqlsh -e "describe cluster" > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        # Create snapshot
        SNAPSHOT_NAME="backup_$TIMESTAMP"
        docker exec "$CASSANDRA_CONTAINER" nodetool snapshot "$SNAPSHOT_NAME" > /dev/null 2>&1 || true
        
        # Find snapshot directory
        SNAPSHOT_DIR=$(docker exec "$CASSANDRA_CONTAINER" find /var/lib/cassandra/data -name "$SNAPSHOT_NAME" -type d | head -n 1)
        
        if [ -n "$SNAPSHOT_DIR" ]; then
            # Copy snapshot data to backup
            docker exec "$CASSANDRA_CONTAINER" tar -czf - -C /var/lib/cassandra/data . > "$BACKUP_FILE"
            
            if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
                echo -e "${GREEN}✓ Cassandra backup created: $BACKUP_FILE${NC}"
            else
                echo -e "${RED}✗ Cassandra backup failed${NC}"
            fi
            
            # Cleanup snapshot
            docker exec "$CASSANDRA_CONTAINER" nodetool clearsnapshot "$SNAPSHOT_NAME" > /dev/null 2>&1 || true
        else
            echo -e "${YELLOW}⚠ Cassandra snapshot not created, skipping backup${NC}"
        fi
    else
        echo -e "${RED}✗ Cassandra not ready for backup${NC}"
    fi
else
    echo -e "${RED}✗ Cassandra container not found${NC}"
fi

# =============================================================================
# REDIS BACKUP
# =============================================================================

echo -e "${YELLOW}Backing up Redis...${NC}"

REDIS_CONTAINER=$(docker ps -q -f name=secureconnect_redis)

if [ -n "$REDIS_CONTAINER" ]; then
    BACKUP_FILE="$BACKUP_DIR/redis_$TIMESTAMP.rdb"
    
    # Trigger Redis SAVE
    docker exec "$REDIS_CONTAINER" redis-cli SAVE > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        # Copy RDB file
        docker cp "$REDIS_CONTAINER:/data/dump.rdb" "$BACKUP_FILE" 2>/dev/null || true
        
        if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
            gzip "$BACKUP_FILE"
            echo -e "${GREEN}✓ Redis backup created: ${BACKUP_FILE}.gz${NC}"
        else
            echo -e "${RED}✗ Redis backup failed${NC}"
        fi
    else
        echo -e "${RED}✗ Redis not ready for backup${NC}"
    fi
else
    echo -e "${RED}✗ Redis container not found${NC}"
fi

# =============================================================================
# MINIO BACKUP
# =============================================================================

echo -e "${YELLOW}Backing up MinIO...${NC}"

MINIO_CONTAINER=$(docker ps -q -f name=secureconnect_minio)

if [ -n "$MINIO_CONTAINER" ]; then
    BACKUP_FILE="$BACKUP_DIR/minio_$TIMESTAMP.tar.gz"
    
    # Backup MinIO data directory
    docker exec "$MINIO_CONTAINER" tar -czf - -C /data . > "$BACKUP_FILE" 2>/dev/null || true
    
    if [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
        echo -e "${GREEN}✓ MinIO backup created: $BACKUP_FILE${NC}"
    else
        echo -e "${YELLOW}⚠ MinIO backup skipped (no data or permission issue)${NC}"
    fi
else
    echo -e "${RED}✗ MinIO container not found${NC}"
fi

# =============================================================================
# CLEANUP OLD BACKUPS
# =============================================================================

echo ""
echo -e "${YELLOW}Cleaning up old backups (older than $RETENTION_DAYS days)...${NC}"

find "$BACKUP_DIR" -name "*.gz" -type f -mtime +$RETENTION_DAYS -delete 2>/dev/null || true
find "$BACKUP_DIR" -name "*.tar.gz" -type f -mtime +$RETENTION_DAYS -delete 2>/dev/null || true

echo -e "${GREEN}✓ Cleanup complete${NC}"

# =============================================================================
# SUMMARY
# =============================================================================

echo ""
echo -e "${GREEN}=== Backup Complete ===${NC}"
echo ""
echo "Backup files created in: $BACKUP_DIR"
echo ""
ls -lh "$BACKUP_DIR" | grep "$TIMESTAMP"
echo ""
echo -e "${YELLOW}To restore backups, see: ./scripts/restore-databases.sh${NC}"
