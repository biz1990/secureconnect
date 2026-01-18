#!/bin/bash
# =============================================================================
# SecureConnect - CockroachDB TLS Certificate Generation
# =============================================================================
# Generates TLS certificates for secure CockroachDB deployment
# Usage: ./scripts/generate-certs.sh
# =============================================================================

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

CERTS_DIR="./certs"
CA_KEY="./ca.key"

echo -e "${BLUE}=== CockroachDB TLS Certificate Generation ===${NC}"
echo ""

# Check if cockroach binary is available
if ! command -v cockroach &> /dev/null; then
    echo -e "${RED}✗ CockroachDB binary not found${NC}"
    echo "Please install CockroachDB: https://www.cockroachlabs.com/docs/stable/install-cockroachdb.html"
    echo ""
    echo "Or use Docker:"
    echo "  docker run -it --rm -v \$(pwd):/tmp/certs cockroachdb/cockroach:v23.1.0 cert create-ca --certs-dir=/tmp/certs --ca-key=/tmp/ca.key"
    exit 1
fi

# Create certs directory
mkdir -p "$CERTS_DIR"

echo -e "${GREEN}Creating TLS certificates in: $CERTS_DIR${NC}"
echo ""

# 1. Create Certificate Authority (CA)
echo -e "${BLUE}1. Creating Certificate Authority (CA)...${NC}"
if [ -f "$CERTS_DIR/ca.crt" ]; then
    echo -e "${YELLOW}CA certificate already exists (skipping)${NC}"
else
    cockroach cert create-ca \
        --certs-dir="$CERTS_DIR" \
        --ca-key="$CA_KEY" \
        --overwrite
    echo -e "${GREEN}✓ CA certificate created${NC}"
fi

# 2. Create node certificate
echo ""
echo -e "${BLUE}2. Creating node certificate...${NC}"
if [ -f "$CERTS_DIR/node.crt" ]; then
    echo -e "${YELLOW}Node certificate already exists (skipping)${NC}"
else
    cockroach cert create-node \
        localhost \
        cockroachdb \
        secureconnect_crdb \
        127.0.0.1 \
        ::1 \
        --certs-dir="$CERTS_DIR" \
        --ca-key="$CA_KEY" \
        --overwrite
    echo -e "${GREEN}✓ Node certificate created${NC}"
fi

# 3. Create client certificate for root user
echo ""
echo -e "${BLUE}3. Creating client certificate for 'root' user...${NC}"
if [ -f "$CERTS_DIR/client.root.crt" ]; then
    echo -e "${YELLOW}Client certificate already exists (skipping)${NC}"
else
    cockroach cert create-client \
        root \
        --certs-dir="$CERTS_DIR" \
        --ca-key="$CA_KEY" \
        --overwrite
    echo -e "${GREEN}✓ Client certificate created for 'root'${NC}"
fi

# Set proper permissions
chmod 700 "$CERTS_DIR"
chmod 600 "$CA_KEY"
chmod 600 "$CERTS_DIR"/*.key 2>/dev/null || true

echo ""
echo -e "${GREEN}=== Certificate Generation Complete ===${NC}"
echo ""
echo "Created certificates:"
ls -lh "$CERTS_DIR"
echo ""
echo -e "${GREEN}CA Certificate:${NC} $CERTS_DIR/ca.crt"
echo -e "${GREEN}Node Certificate:${NC} $CERTS_DIR/node.crt"
echo -e "${GREEN}Client Certificate:${NC} $CERTS_DIR/client.root.crt"
echo ""
echo -e "${YELLOW}⚠️  SECURITY REMINDERS:${NC}"
echo "1. Keep '$CA_KEY' secure and DO NOT commit to git"
echo "2. Add to .gitignore: certs/, ca.key, *.key, *.pem"
echo "3. Certificates are valid for 1 year by default"
echo "4. Back up certificates to a secure location"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo "1. Update docker-compose.production.yml to mount certs directory"
echo "2. Change CockroachDB command from --insecure to --certs-dir=/cockroach/certs"
echo "3. Update application connection strings to use sslmode=require"
echo ""
