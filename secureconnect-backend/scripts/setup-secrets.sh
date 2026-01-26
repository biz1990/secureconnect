#!/bin/bash
# =============================================================================
# SecureConnect Backend - Docker Secrets Setup Script
# =============================================================================
# This script creates Docker secrets for production deployment
#
# Usage: ./scripts/setup-secrets.sh
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== SecureConnect Backend - Docker Secrets Setup ===${NC}"
echo ""

# Function to generate random secret
generate_secret() {
    if command -v openssl &> /dev/null; then
        openssl rand -base64 32 | tr -d "=+/" | cut -c1-32
    else
        echo "WARNING: openssl not found, using fallback method" >&2
        cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1
    fi
}

# Function to create secret if it doesn't exist
create_secret() {
    local secret_name=$1
    local secret_value=$2
    local description=$3
    
    if docker secret inspect "$secret_name" &> /dev/null; then
        echo -e "${YELLOW}Secret '$secret_name' already exists. Skipping...${NC}"
    else
        echo "$secret_value" | docker secret create "$secret_name" -
        echo -e "${GREEN}✓ Created secret: $secret_name${NC} - $description"
    fi
}

# =============================================================================
# CREATE SECRETS
# =============================================================================

echo -e "${YELLOW}Creating Docker secrets...${NC}"
echo ""

# JWT Secret
JWT_SECRET=$(generate_secret)
create_secret "jwt_secret" "$JWT_SECRET" "JWT signing key (32+ characters)"

# Database Password (CockroachDB)
DB_PASSWORD=$(generate_secret)
create_secret "db_password" "$DB_PASSWORD" "CockroachDB password"

# Cassandra User
CASSANDRA_USER="cassandra"
create_secret "cassandra_user" "$CASSANDRA_USER" "Cassandra username"

# Cassandra Password
CASSANDRA_PASSWORD=$(generate_secret)
create_secret "cassandra_password" "$CASSANDRA_PASSWORD" "Cassandra password"

# Redis Password
REDIS_PASSWORD=$(generate_secret)
create_secret "redis_password" "$REDIS_PASSWORD" "Redis password"

# MinIO Access Key
MINIO_ACCESS_KEY=$(generate_secret | cut -c1-20)
create_secret "minio_access_key" "$MINIO_ACCESS_KEY" "MinIO access key (20 chars)"

# MinIO Secret Key
MINIO_SECRET_KEY=$(generate_secret)
create_secret "minio_secret_key" "$MINIO_SECRET_KEY" "MinIO secret key (32 chars)"

# =============================================================================
# DISPLAY SECRETS (for backup purposes)
# =============================================================================

echo ""
echo -e "${YELLOW}=== Generated Secrets (SAVE THESE SECURELY!) ===${NC}"
echo ""
echo "JWT_SECRET=$JWT_SECRET"
echo "DB_PASSWORD=$DB_PASSWORD"
echo "CASSANDRA_USER=$CASSANDRA_USER"
echo "CASSANDRA_PASSWORD=$CASSANDRA_PASSWORD"
echo "REDIS_PASSWORD=$REDIS_PASSWORD"
echo "MINIO_ACCESS_KEY=$MINIO_ACCESS_KEY"
echo "MINIO_SECRET_KEY=$MINIO_SECRET_KEY"
echo ""
echo -e "${RED}WARNING: Save these values in a secure password manager!${NC}"
echo ""

# =============================================================================
# VERIFY SECRETS
# =============================================================================

echo -e "${YELLOW}Verifying secrets...${NC}"
docker secret ls

echo ""
echo -e "${GREEN}=== Secrets setup complete! ===${NC}"
echo ""
echo "To use these secrets, run:"
echo "  docker compose -f docker-compose.production.yml up -d"
echo ""
