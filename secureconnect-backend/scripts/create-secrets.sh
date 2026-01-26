#!/bin/bash
# =============================================================================
# SecureConnect - Docker Secrets Creation Script
# =============================================================================
# Creates all required Docker secrets for production deployment
# Usage: ./scripts/create-secrets.sh
# =============================================================================

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== SecureConnect Production Secrets Setup ===${NC}"
echo ""

# Check if running in Docker Swarm mode
if ! docker info 2>/dev/null | grep -q "Swarm: active"; then
    echo -e "${YELLOW}⚠️  Warning: Docker Swarm is not active${NC}"
    echo "Initializing Docker Swarm..."
    docker swarm init || true
fi

echo -e "${GREEN}Creating Docker secrets for production deployment...${NC}"
echo ""

# Function to create secret from user input
create_secret() {
    local secret_name=$1
    local prompt_message=$2
    local is_password=${3:-false}
    
    # Check if secret already exists
    if docker secret inspect "$secret_name" >/dev/null 2>&1; then
        echo -e "${YELLOW}✓ Secret '$secret_name' already exists (skipping)${NC}"
        return 0
    fi
    
    echo -e "${BLUE}Creating secret: $secret_name${NC}"
    echo "$prompt_message"
    
    if [ "$is_password" = true ]; then
        # Read password without echo
        read -s -p "Enter value: " secret_value
        echo ""
        read -s -p "Confirm value: " secret_value_confirm
        echo ""
        
        if [ "$secret_value" != "$secret_value_confirm" ]; then
            echo -e "${RED}✗ Values don't match. Skipping $secret_name${NC}"
            return 1
        fi
    else
        read -p "Enter value: " secret_value
    fi
    
    if [ -z "$secret_value" ]; then
        echo -e "${RED}✗ Empty value. Skipping $secret_name${NC}"
        return 1
    fi
    
    # Create secret
    echo "$secret_value" | docker secret create "$secret_name" - >/dev/null
    echo -e "${GREEN}✓ Created secret: $secret_name${NC}"
    echo ""
}

# Function to create secret from file
create_secret_from_file() {
    local secret_name=$1
    local file_path=$2
    
    # Check if secret already exists
    if docker secret inspect "$secret_name" >/dev/null 2>&1; then
        echo -e "${YELLOW}✓ Secret '$secret_name' already exists (skipping)${NC}"
        return 0
    fi
    
    if [ ! -f "$file_path" ]; then
        echo -e "${RED}✗ File not found: $file_path${NC}"
        return 1
    fi
    
    docker secret create "$secret_name" "$file_path" >/dev/null
    echo -e "${GREEN}✓ Created secret from file: $secret_name${NC}"
}

# Function to generate random secret
generate_random_secret() {
    local secret_name=$1
    local length=${2:-32}
    
    # Check if secret already exists
    if docker secret inspect "$secret_name" >/dev/null 2>&1; then
        echo -e "${YELLOW}✓ Secret '$secret_name' already exists (skipping)${NC}"
        return 0
    fi
    
    openssl rand -base64 "$length" | docker secret create "$secret_name" - >/dev/null
    echo -e "${GREEN}✓ Generated random secret: $secret_name${NC}"
}

# =============================================================================
# Create all secrets
# =============================================================================

echo -e "${BLUE}--- Critical Secrets ---${NC}"

# JWT Secret
echo ""
echo -e "${YELLOW}JWT_SECRET: Used for signing authentication tokens${NC}"
echo "Recommendation: Auto-generate a 32-byte random string"
read -p "Auto-generate JWT secret? (Y/n): " auto_jwt
if [ "$auto_jwt" != "n" ] && [ "$auto_jwt" != "N" ]; then
    generate_random_secret "jwt_secret" 32
else
    create_secret "jwt_secret" "Enter JWT secret (min 32 characters):" true
fi

# Database Password (CockroachDB)
echo ""
echo -e "${YELLOW}DB_PASSWORD: CockroachDB root password${NC}"
create_secret "db_password" "Enter database password:" true

# Cassandra Credentials
echo ""
echo -e "${YELLOW}CASSANDRA_USER: Cassandra username${NC}"
create_secret "cassandra_user" "Enter Cassandra username:" false

echo ""
echo -e "${YELLOW}CASSANDRA_PASSWORD: Cassandra password${NC}"
create_secret "cassandra_password" "Enter Cassandra password:" true

# Redis Password
echo ""
echo -e "${YELLOW}REDIS_PASSWORD: Redis authentication password${NC}"
read -p "Auto-generate Redis password? (Y/n): " auto_redis
if [ "$auto_redis" != "n" ] && [ "$auto_redis" != "N" ]; then
    generate_random_secret "redis_password" 32
else
    create_secret "redis_password" "Enter Redis password:" true
fi

echo ""
echo -e "${BLUE}--- MinIO Credentials ---${NC}"

# MinIO Access Key
echo ""
create_secret "minio_access_key" "Enter MinIO access key (username):" false

# MinIO Secret Key
echo ""
read -p "Auto-generate MinIO secret key? (Y/n): " auto_minio
if [ "$auto_minio" != "n" ] && [ "$auto_minio" != "N" ]; then
    generate_random_secret "minio_secret_key" 32
else
    create_secret "minio_secret_key" "Enter MinIO secret key:" true
fi

echo ""
echo -e "${BLUE}--- SMTP Credentials ---${NC}"

# SMTP Username
echo ""
create_secret "smtp_username" "Enter SMTP username (email address):" false

# SMTP Password
echo ""
create_secret "smtp_password" "Enter SMTP password:" true

echo ""
echo -e "${BLUE}--- Firebase Credentials ---${NC}"

# Firebase Project ID
echo ""
create_secret "firebase_project_id" "Enter Firebase project ID:" false

# Firebase Credentials File
echo ""
echo -e "${YELLOW}Firebase credentials JSON file${NC}"
echo "Please provide the path to your Firebase service account JSON file"
echo "Example: ./secrets/firebase-new-credentials.json"
read -p "Enter file path: " firebase_file

if [ -n "$firebase_file" ] && [ -f "$firebase_file" ]; then
    create_secret_from_file "firebase_credentials" "$firebase_file"
else
    echo -e "${RED}✗ Invalid file path. Skipping firebase_credentials${NC}"
    echo -e "${YELLOW}⚠️  You can create this secret later with:${NC}"
    echo "    docker secret create firebase_credentials ./path/to/firebase.json"
fi

# =============================================================================
# Summary
# =============================================================================

echo ""
echo -e "${GREEN}=== Secret Creation Complete ===${NC}"
echo ""
echo "Created secrets:"
docker secret ls
echo ""
echo -e "${YELLOW}⚠️  IMPORTANT SECURITY REMINDERS:${NC}"
echo "1. Delete any plaintext credential files from your repository"
echo "2. Ensure .gitignore includes: secrets/, certs/, *.key, *.pem"
echo "3. Rotate Firebase credentials at: https://console.firebase.google.com"
echo "4. Never commit secret values to version control"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "1. Run: ./scripts/generate-certs.sh (generate CockroachDB TLS certs)"
echo "2. Run: docker-compose -f docker-compose.production.yml up -d"
echo ""
