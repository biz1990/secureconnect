# SecureConnect - Environment Variables Reference

## Overview

This document lists all environment variables used across SecureConnect services in production deployment.

---

## 🔐 Secrets (via Docker Secrets)

These are **never** set as environment variables. They are mounted as files at `/run/secrets/`.

| Secret Name | Used By | Purpose | Creation Command |
|------------|---------|---------|------------------|
| `jwt_secret` | All services | JWT token signing | `openssl rand -base64 32 \| docker secret create jwt_secret -` |
| `db_password` | auth-service, api-gateway | CockroachDB authentication | User-provided |
| `redis_password` | All services | Redis authentication | `openssl rand -base64 24 \| docker secret create redis_password -` |
| `minio_access_key` | storage-service, api-gateway, chat-service | MinIO access key | User-provided |
| `minio_secret_key` | storage-service, api-gateway, chat-service | MinIO secret key | `openssl rand -base64 32 \| docker secret create minio_secret_key -` |
| `smtp_username` | auth-service, api-gateway | SMTP authentication | User-provided (email address) |
| `smtp_password` | auth-service, api-gateway | SMTP authentication | User-provided (app password) |
| `firebase_project_id` | video-service | Firebase project identifier | User-provided |
| `firebase_credentials` | video-service | Firebase service account JSON | `docker secret create firebase_credentials ./firebase.json` |
| `turn_user` | turn-server | TURN server username | `echo "turn-prod-user" \| docker secret create turn_user -` |
| `turn_password` | turn-server | TURN server password | `openssl rand -base64 24 \| docker secret create turn_password -` |
| `grafana_admin_password` | grafana | Grafana admin password | `openssl rand -base64 24 \| docker secret create grafana_admin_password -` |

**Total Secrets:** 12

---

## 🌍 Environment Variables

### Core Configuration

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `ENV` | `development` | ✅ | All | Environment (development/staging/production) |
| `PORT` | Service-specific | ❌ | All services | Service HTTP port |
| `SERVICE_NAME` | Service-specific | ❌ | All services | Service identifier for logging |

### Application URLs

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `APP_URL` | `https://secureconnect.com` | ✅ | auth-service, api-gateway | Frontend application URL |
| `CORS_ALLOWED_ORIGINS` | `https://secureconnect.com` | ✅ | All services | Comma-separated CORS origins |

### Database - CockroachDB

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `DB_HOST` | `cockroachdb` | ✅ | auth-service, api-gateway | CockroachDB hostname |
| `DB_PORT` | `26257` | ❌ | auth-service, api-gateway | CockroachDB port |
| `DB_USER` | `root` | ❌ | auth-service, api-gateway | Database username |
| `DB_PASSWORD_FILE` | `/run/secrets/db_password` | ✅ | auth-service, api-gateway | Path to DB password secret |
| `DB_NAME` | `secureconnect` | ❌ | auth-service, api-gateway | Database name |
| `DB_SSL_MODE` | `require` | ✅ | auth-service, api-gateway | SSL mode (disable/require/verify-ca/verify-full) |
| `DB_MAX_CONNS` | `25` | ❌ | auth-service, api-gateway | Max connection pool size |
| `DB_MIN_CONNS` | `5` | ❌ | auth-service, api-gateway | Min connection pool size |

### Database - Cassandra

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `CASSANDRA_HOSTS` | `cassandra` | ✅ | chat-service, api-gateway | Comma-separated Cassandra hosts |
| `CASSANDRA_KEYSPACE` | `secureconnect` | ❌ | chat-service, api-gateway | Cassandra keyspace |
| `CASSANDRA_CONSISTENCY` | `QUORUM` | ❌ | chat-service, api-gateway | Consistency level |
| `CASSANDRA_TIMEOUT` | `600ms` | ❌ | chat-service, api-gateway | Query timeout |

### Cache - Redis

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `REDIS_HOST` | `redis` | ✅ | All services | Redis hostname |
| `REDIS_PORT` | `6379` | ❌ | All services | Redis port |
| `REDIS_PASSWORD_FILE` | `/run/secrets/redis_password` | ✅ | All services | Path to Redis password secret |
| `REDIS_DB` | `0` | ❌ | All services | Redis database number |
| `REDIS_POOL_SIZE` | `10` | ❌ | All services | Connection pool size |
| `REDIS_TIMEOUT` | `5s` | ❌ | All services | Command timeout |

### Storage - MinIO

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `MINIO_ENDPOINT` | `http://minio:9000` | ✅ | storage-service, api-gateway, chat-service | MinIO server endpoint |
| `MINIO_ACCESS_KEY_FILE` | `/run/secrets/minio_access_key` | ✅ | storage-service, api-gateway, chat-service | Path to MinIO access key |
| `MINIO_SECRET_KEY_FILE` | `/run/secrets/minio_secret_key` | ✅ | storage-service, api-gateway, chat-service | Path to MinIO secret key |
| `MINIO_USE_SSL` | `false` | ❌ | storage-service, api-gateway, chat-service | Enable SSL for MinIO |
| `MINIO_BUCKET` | `secureconnect` | ❌ | storage-service, api-gateway, chat-service | Default bucket name |

### Email - SMTP

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `SMTP_HOST` | `smtp.gmail.com` | ✅ | auth-service, api-gateway | SMTP server hostname |
| `SMTP_PORT` | `587` | ❌ | auth-service, api-gateway | SMTP server port |
| `SMTP_USERNAME_FILE` | `/run/secrets/smtp_username` | ✅ | auth-service, api-gateway | Path to SMTP username |
| `SMTP_PASSWORD_FILE` | `/run/secrets/smtp_password` | ✅ | auth-service, api-gateway | Path to SMTP password |
| `SMTP_FROM` | `noreply@secureconnect.com` | ❌ | auth-service, api-gateway | Default sender email |

### Push Notifications - Firebase

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `FIREBASE_PROJECT_ID_FILE` | `/run/secrets/firebase_project_id` | ✅ | video-service | Path to Firebase project ID |
| `FIREBASE_CREDENTIALS_PATH` | `/run/secrets/firebase_credentials` | ✅ | video-service | Path to Firebase service account JSON |
| `PUSH_PROVIDER` | `firebase` | ❌ | video-service | Push provider (firebase/mock) |

### JWT Configuration

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `JWT_SECRET_FILE` | `/run/secrets/jwt_secret` | ✅ | All services | Path to JWT signing secret |
| `JWT_ACCESS_EXPIRY` | `15` | ❌ | auth-service | Access token expiry (minutes) |
| `JWT_REFRESH_EXPIRY` | `720` | ❌ | auth-service | Refresh token expiry (hours) |

### Monitoring - Grafana

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `GRAFANA_ADMIN_USER` | `admin` | ❌ | grafana | Admin username |
| `GRAFANA_ADMIN_PASSWORD__FILE` | `/run/secrets/grafana_admin_password` | ✅ | grafana | Path to admin password secret |
| `GRAFANA_URL` | `http://localhost:3000` | ❌ | grafana | Public Grafana URL |

### Monitoring - AlertManager

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `SLACK_WEBHOOK_URL` | None | ⚠️ | alertmanager | Slack webhook for alerts (used in config file) |

### Backup

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `BACKUP_RETENTION_DAYS` | `7` | ❌ | backup-scheduler | Days to retain backups |
| `BACKUP_SCHEDULE` | `0 2 * * *` | ❌ | backup-scheduler | Cron schedule for backups |

### Logging

| Variable | Default | Required | Services | Description |
|----------|---------|----------|----------|-------------|
| `LOG_LEVEL` | `info` | ❌ | All services | Log level (debug/info/warn/error) |
| `LOG_FORMAT` | `json` | ❌ | All services | Log format (json/text) |
| `LOG_OUTPUT` | `stdout` | ❌ | All services | Log output (stdout/file) |
| `LOG_FILE_PATH` | `/logs/app.log` | ❌ | All services | Log file path if output=file |

---

## 📝 Configuration Templates

### .env.production Template

Create this file for production deployment:

```bash
# Core
ENV=production

# URLs
APP_URL=https://secureconnect.com
CORS_ALLOWED_ORIGINS=https://secureconnect.com,https://app.secureconnect.com

# Database
DB_HOST=cockroachdb
DB_SSL_MODE=require

# SMTP
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_FROM=noreply@secureconnect.com

# Monitoring
GRAFANA_URL=https://metrics.secureconnect.com
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL

# Backup
BACKUP_RETENTION_DAYS=30
```

### Secret Creation Script

```bash
#!/bin/bash
# Create all production secrets

# JWT Secret (32 bytes)
openssl rand -base64 32 | docker secret create jwt_secret -

# Database Password
echo "your-secure-db-password" | docker secret create db_password -

# Redis Password (24 bytes)
openssl rand -base64 24 | docker secret create redis_password -

# MinIO Credentials
echo "minio-admin-user" | docker secret create minio_access_key -
openssl rand -base64 32 | docker secret create minio_secret_key -

# SMTP Credentials
echo "your-email@gmail.com" | docker secret create smtp_username -
echo "your-app-password" | docker secret create smtp_password -

# Firebase
echo "your-project-id" | docker secret create firebase_project_id -
docker secret create firebase_credentials ./firebase-service-account.json

# TURN Server
echo "turn-production-user" | docker secret create turn_user -
openssl rand -base64 24 | docker secret create turn_password -

# Grafana
openssl rand -base64 24 | docker secret create grafana_admin_password -
```

---

## ⚠️ Security Notes

1. **Never commit `.env.production`** to git
2. **All passwords use Docker secrets**, not environment variables
3. **Rotate secrets regularly** (every 90 days minimum)
4. **Use strong passwords** (24+ characters, random)
5. **SMTP requires app passwords** for Gmail (not account password)
6. **Firebase credentials must be rotated** after any exposure

---

## 🔍 Validation

Verify all secrets exist before deployment:

```bash
docker secret ls

# Should show all 12 secrets:
# - jwt_secret
# - db_password
# - redis_password
# - minio_access_key
# - minio_secret_key
# - smtp_username
# - smtp_password
# - firebase_project_id
# - firebase_credentials
# - turn_user
# - turn_password
# - grafana_admin_password
```

Verify environment variables:

```bash
# Check a service's environment
docker inspect api-gateway | grep -A 20 "Env"

# Should NOT contain any password values
# Should contain *_FILE paths for secrets
```
