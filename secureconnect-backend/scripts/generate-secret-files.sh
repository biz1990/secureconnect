#!/bin/bash
# =============================================================================
# SecureConnect - Generate Secret Files for Docker Compose
# =============================================================================
# Creates all required secret files for production deployment
# Usage: ./scripts/generate-secret-files.sh
# =============================================================================

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== SecureConnect Production Secret Files Generation ===${NC}"
echo ""

# Create secrets directory
SECRETS_DIR="./secrets"
if [ ! -d "$SECRETS_DIR" ]; then
    mkdir -p "$SECRETS_DIR"
    echo -e "${GREEN}✓ Created secrets directory${NC}"
else
    echo -e "${YELLOW}✓ Secrets directory already exists${NC}"
fi

# Function to generate random secret
generate_random_secret() {
    if command -v openssl &> /dev/null; then
        openssl rand -base64 32 | tr -d "=+/" | cut -c1-32
    else
        cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1
    fi
}

# Function to create secret file
create_secret_file() {
    local secret_name=$1
    local secret_value=$2
    local secret_file="$SECRETS_DIR/${secret_name}.txt"
    
    if [ -f "$secret_file" ]; then
        echo -e "${YELLOW}✓ Secret file '$secret_name' already exists (skipping)${NC}"
        return 0
    fi
    
    echo "$secret_value" > "$secret_file"
    chmod 600 "$secret_file"
    echo -e "${GREEN}✓ Created secret file: $secret_name${NC}"
}

# Function to create secret file from user input
create_secret_file_from_input() {
    local secret_name=$1
    local prompt_message=$2
    local is_password=${3:-false}
    local secret_file="$SECRETS_DIR/${secret_name}.txt"
    
    if [ -f "$secret_file" ]; then
        echo -e "${YELLOW}✓ Secret file '$secret_name' already exists (skipping)${NC}"
        return 0
    fi
    
    echo -e "${BLUE}Creating secret file: $secret_name${NC}"
    echo "$prompt_message"
    
    if [ "$is_password" = true ]; then
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
    
    echo "$secret_value" > "$secret_file"
    chmod 600 "$secret_file"
    echo -e "${GREEN}✓ Created secret file: $secret_name${NC}"
    echo ""
}

# =============================================================================
# Generate all secrets
# =============================================================================

echo -e "${BLUE}--- Critical Secrets ---${NC}"

# JWT Secret
echo ""
echo -e "${YELLOW}JWT_SECRET: Used for signing authentication tokens${NC}"
read -p "Auto-generate JWT secret? (Y/n): " auto_jwt
if [ "$auto_jwt" != "n" ] && [ "$auto_jwt" != "N" ]; then
    JWT_SECRET=$(generate_random_secret)
    create_secret_file "jwt_secret" "$JWT_SECRET"
else
    create_secret_file_from_input "jwt_secret" "Enter JWT secret (min 32 characters):" true
fi

# Database Password (CockroachDB)
echo ""
echo -e "${YELLOW}DB_PASSWORD: CockroachDB root password${NC}"
read -p "Auto-generate DB password? (Y/n): " auto_db
if [ "$auto_db" != "n" ] && [ "$auto_db" != "N" ]; then
    DB_PASSWORD=$(generate_random_secret)
    create_secret_file "db_password" "$DB_PASSWORD"
else
    create_secret_file_from_input "db_password" "Enter database password:" true
fi

# Cassandra Credentials
echo ""
echo -e "${YELLOW}CASSANDRA_USER: Cassandra username${NC}"
read -p "Use default username 'cassandra'? (Y/n): " use_default_cassandra_user
if [ "$use_default_cassandra_user" != "n" ] && [ "$use_default_cassandra_user" != "N" ]; then
    create_secret_file "cassandra_user" "cassandra"
else
    create_secret_file_from_input "cassandra_user" "Enter Cassandra username:" false
fi

echo ""
echo -e "${YELLOW}CASSANDRA_PASSWORD: Cassandra password${NC}"
read -p "Auto-generate Cassandra password? (Y/n): " auto_cassandra
if [ "$auto_cassandra" != "n" ] && [ "$auto_cassandra" != "N" ]; then
    CASSANDRA_PASSWORD=$(generate_random_secret)
    create_secret_file "cassandra_password" "$CASSANDRA_PASSWORD"
else
    create_secret_file_from_input "cassandra_password" "Enter Cassandra password:" true
fi

# Redis Password
echo ""
echo -e "${YELLOW}REDIS_PASSWORD: Redis authentication password${NC}"
read -p "Auto-generate Redis password? (Y/n): " auto_redis
if [ "$auto_redis" != "n" ] && [ "$auto_redis" != "N" ]; then
    REDIS_PASSWORD=$(generate_random_secret)
    create_secret_file "redis_password" "$REDIS_PASSWORD"
else
    create_secret_file_from_input "redis_password" "Enter Redis password:" true
fi

echo ""
echo -e "${BLUE}--- MinIO Credentials ---${NC}"

# MinIO Access Key
echo ""
echo -e "${YELLOW}MINIO_ACCESS_KEY: MinIO access key (username)${NC}"
read -p "Auto-generate MinIO access key? (Y/n): " auto_minio_access
if [ "$auto_minio_access" != "n" ] && [ "$auto_minio_access" != "N" ]; then
    MINIO_ACCESS_KEY=$(generate_random_secret | cut -c1-20)
    create_secret_file "minio_access_key" "$MINIO_ACCESS_KEY"
else
    create_secret_file_from_input "minio_access_key" "Enter MinIO access key (20 chars):" false
fi

# MinIO Secret Key
echo ""
echo -e "${YELLOW}MINIO_SECRET_KEY: MinIO secret key${NC}"
read -p "Auto-generate MinIO secret key? (Y/n): " auto_minio_secret
if [ "$auto_minio_secret" != "n" ] && [ "$auto_minio_secret" != "N" ]; then
    MINIO_SECRET_KEY=$(generate_random_secret)
    create_secret_file "minio_secret_key" "$MINIO_SECRET_KEY"
else
    create_secret_file_from_input "minio_secret_key" "Enter MinIO secret key:" true
fi

echo ""
echo -e "${BLUE}--- SMTP Credentials ---${NC}"

# SMTP Username
echo ""
echo -e "${YELLOW}SMTP_USERNAME: SMTP username (email address)${NC}"
create_secret_file_from_input "smtp_username" "Enter SMTP username:" false

# SMTP Password
echo ""
echo -e "${YELLOW}SMTP_PASSWORD: SMTP password${NC}"
create_secret_file_from_input "smtp_password" "Enter SMTP password:" true

echo ""
echo -e "${BLUE}--- Firebase Credentials ---${NC}"

# Firebase Project ID
echo ""
echo -e "${YELLOW}FIREBASE_PROJECT_ID: Firebase project ID${NC}"
create_secret_file_from_input "firebase_project_id" "Enter Firebase project ID:" false

# Firebase Credentials File
echo ""
echo -e "${YELLOW}FIREBASE_CREDENTIALS: Firebase service account JSON file${NC}"
echo "Please provide the path to your Firebase service account JSON file"
echo "Example: ./firebase-service-account.json"
read -p "Enter file path: " firebase_file

if [ -n "$firebase_file" ] && [ -f "$firebase_file" ]; then
    if [ ! -f "$SECRETS_DIR/firebase_credentials.json" ]; then
        cp "$firebase_file" "$SECRETS_DIR/firebase_credentials.json"
        chmod 600 "$SECRETS_DIR/firebase_credentials.json"
        echo -e "${GREEN}✓ Created secret file: firebase_credentials.json${NC}"
    else
        echo -e "${YELLOW}✓ Secret file 'firebase_credentials.json' already exists (skipping)${NC}"
    fi
else
    echo -e "${RED}✗ Invalid file path. Skipping firebase_credentials${NC}"
    echo -e "${YELLOW}⚠️  You can create this file later by copying your Firebase JSON to:${NC}"
    echo "    ./secrets/firebase_credentials.json"
fi

echo ""
echo -e "${BLUE}--- TURN Server Credentials ---${NC}"

# TURN User
echo ""
echo -e "${YELLOW}TURN_USER: TURN server username${NC}"
read -p "Use default username 'turnuser'? (Y/n): " use_default_turn_user
if [ "$use_default_turn_user" != "n" ] && [ "$use_default_turn_user" != "N" ]; then
    create_secret_file "turn_user" "turnuser"
else
    create_secret_file_from_input "turn_user" "Enter TURN server username:" false
fi

# TURN Password
echo ""
echo -e "${YELLOW}TURN_PASSWORD: TURN server password${NC}"
read -p "Auto-generate TURN password? (Y/n): " auto_turn
if [ "$auto_turn" != "n" ] && [ "$auto_turn" != "N" ]; then
    TURN_PASSWORD=$(generate_random_secret)
    create_secret_file "turn_password" "$TURN_PASSWORD"
else
    create_secret_file_from_input "turn_password" "Enter TURN server password:" true
fi

# =============================================================================
# Summary
# =============================================================================

echo ""
echo -e "${GREEN}=== Secret Files Generation Complete ===${NC}"
echo ""
echo "Created secret files in: $SECRETS_DIR/"
ls -la "$SECRETS_DIR" | tail -n +2
echo ""
echo -e "${YELLOW}⚠️  IMPORTANT SECURITY REMINDERS:${NC}"
echo "1. Add '$SECRETS_DIR/' to .gitignore"
echo "2. Never commit these files to version control"
echo "3. Store backup copies in a secure password manager"
echo "4. Set appropriate file permissions (600)"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "1. Run: ./scripts/generate-certs.sh (generate CockroachDB TLS certs)"
echo "2. Run: docker compose -f docker-compose.production.yml up -d --build"
echo ""
