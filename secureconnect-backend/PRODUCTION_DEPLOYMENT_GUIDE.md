# SecureConnect Backend - Production Deployment Guide

**Version**: 1.0  
**Last Updated**: 2026-01-11  
**Auditor**: Senior Software Architect / Production Code Auditor

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Security Setup](#security-setup)
3. [Production Deployment](#production-deployment)
4. [Monitoring & Logging](#monitoring--logging)
5. [Backup & Restore](#backup--restore)
6. [Troubleshooting](#troubleshooting)
7. [Maintenance](#maintenance)

---

## Prerequisites

### System Requirements

| Resource | Minimum | Recommended |
|----------|-----------|--------------|
| CPU | 4 cores | 8+ cores |
| RAM | 8 GB | 16+ GB |
| Storage | 100 GB | 500+ GB SSD |
| OS | Linux (Ubuntu 20.04+) | Linux (Ubuntu 22.04+) |
| Docker | 20.10+ | 24.0+ |
| Docker Compose | 2.0+ | 2.20+ |

### Software Requirements

```bash
# Docker Engine
docker --version

# Docker Compose
docker compose version

# OpenSSL (for secret generation)
openssl version

# Git (for deployment scripts)
git version
```

---

## Security Setup

### 1. Docker Secrets Management

#### Option A: Using Docker Secrets (Recommended)

```bash
# Navigate to project directory
cd secureconnect-backend

# Generate and create secrets
chmod +x scripts/setup-secrets.sh
./scripts/setup-secrets.sh

# Verify secrets
docker secret ls
```

#### Option B: Using Environment Variables (Development Only)

```bash
# Copy secrets example
cp .env.secrets.example .env

# Edit with your values
nano .env
```

### 2. SSL/TLS Configuration

#### Generate Self-Signed Certificates (Development)

```bash
# Create SSL directory
mkdir -p configs/ssl

# Generate certificates
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout configs/ssl/server.key \
  -out configs/ssl/server.crt \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"
```

#### Use Let's Encrypt (Production)

```bash
# Install certbot
sudo apt-get install certbot

# Generate certificates
sudo certbot certonly --standalone -d api.yourdomain.com

# Copy certificates to configs/ssl/
sudo cp /etc/letsencrypt/live/api.yourdomain.com/fullchain.pem configs/ssl/server.crt
sudo cp /etc/letsencrypt/live/api.yourdomain.com/privkey.pem configs/ssl/server.key
```

### 3. Firewall Configuration

```bash
# Allow required ports
sudo ufw allow 80/tcp    # HTTP
sudo ufw allow 443/tcp   # HTTPS
sudo ufw allow 8080/tcp  # API Gateway (if direct access)
sudo ufw allow 26257/tcp # CockroachDB (if remote access)
sudo ufw allow 9042/tcp  # Cassandra (if remote access)
sudo ufw allow 6379/tcp  # Redis (if remote access)
sudo ufw allow 9000/tcp  # MinIO API (if remote access)
sudo ufw allow 9001/tcp  # MinIO Console (if remote access)
sudo ufw allow 3000/tcp  # Grafana (if remote access)

# Enable firewall
sudo ufw enable
```

---

## Production Deployment

### 1. Clone Repository

```bash
# Clone repository
git clone https://github.com/your-org/secureconnect-backend.git
cd secureconnect-backend
```

### 2. Configure Environment

```bash
# Create production environment file
cp .env.example .env.production

# Edit configuration
nano .env.production
```

### 3. Setup Docker Secrets

```bash
# Generate secrets
chmod +x scripts/setup-secrets.sh
./scripts/setup-secrets.sh
```

### 4. Initialize Databases

```bash
# CockroachDB
docker exec secureconnect_crdb ./cockroach sql --insecure \
  -e "CREATE DATABASE IF NOT EXISTS secureconnect_poc;"

# Cassandra
docker exec secureconnect_cassandra cqlsh -e "
  CREATE KEYSPACE IF NOT EXISTS secureconnect_ks 
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};
"

# Apply schema
docker exec secureconnect_cassandra cqlsh -f /dev/stdin < scripts/cassandra-schema.cql
```

### 5. Deploy Services

```bash
# Build and start all services
docker compose -f docker-compose.production.yml up -d

# Check status
docker compose -f docker-compose.production.yml ps

# View logs
docker compose -f docker-compose.production.yml logs -f
```

### 6. Verify Deployment

```bash
# Check all containers
docker ps

# Test health endpoints
curl http://localhost/health
curl http://localhost:8080/health

# Test databases
docker exec secureconnect_crdb ./cockroach sql --insecure -e "SELECT 1;"
docker exec secureconnect_redis redis-cli PING
docker exec secureconnect_cassandra cqlsh -e "SELECT now() FROM system.local;"
```

---

## Monitoring & Logging

### 1. Deploy Logging Stack

```bash
# Deploy Loki, Promtail, and Grafana
docker compose -f docker-compose.logging.yml up -d
```

### 2. Access Grafana

1. Open browser: `http://localhost:3000`
2. Login with: `admin` / `change-me-in-production`
3. Navigate to **Explore** to view logs
4. Filter by service: `app="api-gateway"`, `app="auth-service"`, etc.

### 3. Configure Log Retention

Edit [`configs/loki-config.yml`](configs/loki-config.yml):

```yaml
schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h
      # Retention settings
      retention:
        period: 720h  # 30 days
```

### 4. Create Grafana Dashboards

1. Navigate to **Dashboards** → **New Dashboard**
2. Add **Logs** panel
3. Query: `{app="api-gateway"}`
4. Save dashboard as "SecureConnect API Gateway"

---

## Backup & Restore

### 1. Automated Backups

#### Setup Cron Job

```bash
# Edit crontab
crontab -e

# Add backup job (daily at 2 AM)
0 2 * * * /path/to/secureconnect-backend/scripts/backup-databases.sh /path/to/backups >> /var/log/backup.log 2>&1
```

#### Backup to Cloud Storage

```bash
# Install rclone
sudo apt-get install rclone

# Configure rclone
rclone config

# Create backup script with cloud upload
cat > scripts/backup-to-cloud.sh << 'EOF'
#!/bin/bash
BACKUP_DIR="/path/to/backups"
RCLONE_REMOTE="s3:secureconnect-backups"

# Run local backup
./scripts/backup-databases.sh "$BACKUP_DIR"

# Upload to cloud
rclone sync "$BACKUP_DIR" "$RCLONE_REMOTE/$(date +%Y%m%d)" --progress
EOF

chmod +x scripts/backup-to-cloud.sh
```

### 2. Manual Backup

```bash
# Backup all databases
chmod +x scripts/backup-databases.sh
./scripts/backup-databases.sh ./backups

# List backups
ls -lh ./backups/
```

### 3. Restore from Backup

```bash
# Restore specific backup
chmod +x scripts/restore-databases.sh
./scripts/restore-databases.sh ./backups/cockroachdb_20260111_020000.sql.gz

# Verify restoration
docker exec secureconnect_crdb ./cockroach sql --insecure -e "SELECT COUNT(*) FROM users;"
```

---

## Troubleshooting

### Common Issues

#### Issue 1: Container Won't Start

```bash
# Check logs
docker logs <container_name>

# Check resource usage
docker stats

# Check disk space
df -h
```

#### Issue 2: Database Connection Failed

```bash
# Check database status
docker exec secureconnect_crdb ./cockroach sql --insecure -e "SELECT 1;"
docker exec secureconnect_cassandra cqlsh -e "SELECT now() FROM system.local;"
docker exec secureconnect_redis redis-cli PING

# Check network connectivity
docker network inspect secureconnect-net
```

#### Issue 3: Out of Memory

```bash
# Check memory usage
docker stats --no-stream

# Increase swap
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

#### Issue 4: Cassandra Slow Startup

```bash
# Check Cassandra logs
docker logs secureconnect_cassandra

# Increase startup timeout in docker-compose.yml
healthcheck:
  start_period: 300s  # Increase to 5 minutes
```

### Health Checks

```bash
# Check all services
for service in api-gateway auth-service chat-service video-service; do
  echo "Checking $service..."
  docker exec $service wget -q -O - http://localhost:8080/health && echo "OK" || echo "FAIL"
done
```

---

## Maintenance

### 1. Update Services

```bash
# Pull latest images
docker compose -f docker-compose.production.yml pull

# Recreate containers
docker compose -f docker-compose.production.yml up -d --force-recreate
```

### 2. Cleanup Old Images

```bash
# Remove unused images
docker image prune -a

# Remove unused volumes
docker volume prune

# Remove unused networks
docker network prune
```

### 3. Rotate Secrets

```bash
# Generate new secrets
./scripts/setup-secrets.sh

# Update services
docker compose -f docker-compose.production.yml up -d --force-recreate
```

### 4. Monitor Disk Space

```bash
# Check volume usage
docker system df

# Clean up
docker system prune -a --volumes
```

---

## Security Best Practices

### 1. Regular Updates

```bash
# Update Docker
sudo apt-get update && sudo apt-get install docker-ce docker-ce-cli containerd.io

# Update base images
docker pull cockroachdb/cockroach:latest
docker pull cassandra:latest
docker pull redis:7-alpine
docker pull minio/minio:latest
```

### 2. Access Control

```bash
# Restrict Docker socket access
sudo chmod 600 /var/run/docker.sock
sudo chown root:docker /var/run/docker.sock

# Use non-root user in containers
```

### 3. Network Isolation

```bash
# Create separate networks for different services
docker network create secureconnect-app
docker network create secureconnect-db
docker network create secureconnect-public

# Connect services to appropriate networks
docker network connect secureconnect-app api-gateway
docker network connect secureconnect-db secureconnect_crdb
```

---

## Performance Tuning

### 1. CockroachDB Tuning

```yaml
# In docker-compose.yml
environment:
  - COCKROACH_MAX_OFFSET_MEMORY=4GB
  - COCKROACH_CACHE=2GB
  - COCKROACH_MAX_SQL_MEMORY=1GB
```

### 2. Redis Tuning

```yaml
# In docker-compose.yml
command: >
  redis-server
  --maxmemory 2gb
  --maxmemory-policy allkeys-lru
  --appendonly yes
  --save 900 1
```

### 3. Cassandra Tuning

```yaml
# In docker-compose.yml
environment:
  - MAX_HEAP_SIZE=2048M
  - HEAP_NEWSIZE=200M
```

---

## Disaster Recovery

### 1. Backup Strategy

- **Daily**: Full database backups
- **Hourly**: Incremental backups (if supported)
- **Off-site**: Cloud storage replication
- **Retention**: 30 days local, 90 days cloud

### 2. Recovery Procedure

```bash
# 1. Stop services
docker compose -f docker-compose.production.yml down

# 2. Restore databases
./scripts/restore-databases.sh <backup_file>

# 3. Start services
docker compose -f docker-compose.production.yml up -d

# 4. Verify
curl http://localhost/health
```

---

## Support

### Documentation

- [API Documentation](../docs/API_DOCUMENTATION.md)
- [Deployment Architecture](../docs/deployment-architecture-diagrams.md)
- [Troubleshooting Guide](../docs/maintenance/troubleshooting-guide.md)

### Contact

- **Email**: ops@secureconnect.com
- **Slack**: #secureconnect-ops
- **On-Call**: +1-XXX-XXX-XXXX

---

## Appendix

### A. Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|-----------|---------|
| `ENV` | Environment (development/production) | Yes | development |
| `DB_HOST` | CockroachDB host | Yes | localhost |
| `DB_PORT` | CockroachDB port | No | 26257 |
| `DB_NAME` | Database name | Yes | secureconnect_poc |
| `REDIS_HOST` | Redis host | Yes | localhost |
| `REDIS_PORT` | Redis port | No | 6379 |
| `CASSANDRA_HOST` | Cassandra host | Yes | localhost |
| `CASSANDRA_PORT` | Cassandra port | No | 9042 |
| `MINIO_ENDPOINT` | MinIO endpoint | Yes | http://localhost:9000 |
| `MINIO_ACCESS_KEY` | MinIO access key | Yes | minioadmin |
| `MINIO_SECRET_KEY` | MinIO secret key | Yes | minioadmin |
| `JWT_SECRET` | JWT signing secret | Yes | - |
| `PORT` | Service port | No | 8080 |

### B. Port Reference

| Port | Service | Purpose | External |
|-------|----------|---------|----------|
| 80 | Nginx | HTTP | Yes |
| 443 | Nginx | HTTPS | Yes |
| 8080 | API Gateway | API | Yes |
| 26257 | CockroachDB | SQL | No |
| 8081 | CockroachDB | Admin UI | No |
| 9042 | Cassandra | CQL | No |
| 6379 | Redis | Redis Protocol | No |
| 9000 | MinIO | S3 API | No |
| 9001 | MinIO | Web Console | No |
| 3000 | Grafana | Web UI | No |
| 3100 | Loki | HTTP API | No |

---

**End of Document**
