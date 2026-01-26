# Firebase Docker Secrets Migration Guide

## Overview

This guide documents the secure migration of Firebase Admin SDK credentials to Docker secrets. Firebase credentials are now loaded from `/run/secrets/firebase_credentials` using Docker secrets, ensuring sensitive data is never committed to version control.

## Architecture

### Docker Secrets Pattern

```
/run/secrets/
├── firebase_credentials        # Firebase Admin SDK JSON credentials
└── firebase_project_id          # Firebase project ID
```

### Environment Variables (FILE Pattern)

```yaml
# Production (Docker secrets)
FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id
FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials

# Local development (file path only)
FIREBASE_PROJECT_ID=your-firebase-project-id
FIREBASE_CREDENTIALS_PATH=./firebase-adminsdk.json
```

## Docker Secret Creation Commands

### 1. Create Firebase Project ID Secret

```bash
# Create the Firebase project ID secret
echo "your-firebase-project-id" | docker secret create firebase_project_id -

# Verify the secret was created
docker secret ls | grep firebase_project_id
```

### 2. Create Firebase Credentials Secret

```bash
# Create the Firebase credentials secret from JSON file
cat firebase-adminsdk.json | docker secret create firebase_credentials -

# Verify the secret was created
docker secret ls | grep firebase_credentials
```

### 3. Create All Required Secrets (Complete Set)

```bash
# JWT Secret
openssl rand -base64 32 | docker secret create jwt_secret -

# Database Password
openssl rand -base64 24 | docker secret create db_password -

# Cassandra Credentials
echo "cassandra" | docker secret create cassandra_user -
openssl rand -base64 24 | docker secret create cassandra_password -

# Redis Password
openssl rand -base64 24 | docker secret create redis_password -

# MinIO Credentials
echo "minioadmin" | docker secret create minio_access_key -
openssl rand -base64 24 | docker secret create minio_secret_key -

# SMTP Credentials
echo "your-email@gmail.com" | docker secret create smtp_username -
echo "your-app-password" | docker secret create smtp_password -

# Firebase Credentials
echo "your-firebase-project-id" | docker secret create firebase_project_id -
cat firebase-adminsdk.json | docker secret create firebase_credentials -

# TURN Server Credentials
echo "turnuser" | docker secret create turn_user -
openssl rand -base64 16 | docker secret create turn_password -

# List all secrets
docker secret ls
```

## Docker Compose Configuration

The [`docker-compose.production.yml`](docker-compose.production.yml) already has the correct configuration:

```yaml
secrets:
  firebase_project_id:
    external: true
  firebase_credentials:
    external: true

services:
  video-service:
    secrets:
      - firebase_project_id
      - firebase_credentials
    environment:
      - FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id
      - FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials
```

## Code Implementation

The [`pkg/push/firebase.go`](pkg/push/firebase.go) supports Docker secrets:

1. **Priority Order for Credentials Path:**
   - `FIREBASE_CREDENTIALS_PATH` (Docker secret path)
   - `GOOGLE_APPLICATION_CREDENTIALS` (legacy fallback)

2. **Priority Order for Project ID:**
   - `FIREBASE_PROJECT_ID_FILE` (Docker secret file)
   - Extracted from credentials JSON
   - Provided as parameter

3. **Security Features:**
   - Credentials read into memory (not passed as file path)
   - No reference to `/app/secrets/*.json`
   - Mock provider for local development

## Validation Steps

### 1. Verify Secrets Exist

```bash
# List all Docker secrets
docker secret ls

# Expected output:
# ID                          NAME                      DRIVER    CREATED          UPDATED
# abc123xyz789               firebase_credentials                1 minute ago     1 minute ago
# def456uvw012               firebase_project_id                 1 minute ago     1 minute ago
```

### 2. Verify Secret Content (Development Only)

```bash
# Inspect secret content (use with caution in production)
docker secret inspect firebase_project_id --format '{{.Spec.Name}}: {{range .Spec.Data}}{{printf "%s" .}}{{end}}'

# Or read from a running container (for debugging)
docker exec secureconnect_video-service cat /run/secrets/firebase_project_id
```

### 3. Verify Container Mount Points

```bash
# Start the service
docker-compose -f docker-compose.production.yml up -d video-service

# Check if secrets are mounted correctly
docker exec secureconnect_video-service ls -la /run/secrets/

# Expected output:
# total 8
# drwxr-xr-x    2 root     root          4096 Jan 22 03:00 .
# drwxr-xr-x    1 root     root          4096 Jan 22 03:00 ..
# lrwxrwxrwx    1 root     root            17 Jan 22 03:00 firebase_credentials -> ..data/firebase_credentials
# lrwxrwxrwx    1 root     root            20 Jan 22 03:00 firebase_project_id -> ..data/firebase_project_id
```

### 4. Verify Application Logs

```bash
# Check if Firebase initialized successfully
docker-compose -f docker-compose.production.yml logs video-service | grep Firebase

# Expected output:
# Firebase Admin SDK initialized successfully: project_id=your-firebase-project-id
```

### 5. Verify No Reference to /app/secrets

```bash
# Search for any remaining references to /app/secrets in the codebase
grep -r "/app/secrets" --include="*.go" --include="*.yml" --include="*.yaml" .

# Expected output: No results (empty)
```

### 6. Test Push Notification (Optional)

```bash
# Send a test push notification via API
curl -X POST http://localhost:8083/api/v1/push/test \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"title":"Test","body":"Docker secrets working!"}'
```

## Security Checklist

- [ ] Firebase JSON file added to `.gitignore` (already done: `firebase*.json`)
- [ ] No Firebase JSON files committed to GitHub
- [ ] Docker secrets created for `firebase_credentials`
- [ ] Docker secrets created for `firebase_project_id`
- [ ] `FIREBASE_CREDENTIALS_PATH` set to `/run/secrets/firebase_credentials`
- [ ] `FIREBASE_PROJECT_ID_FILE` set to `/run/secrets/firebase_project_id`
- [ ] No references to `/app/secrets/*.json` in codebase
- [ ] Secrets are external (not defined in docker-compose file)
- [ ] Secrets are mounted at `/run/secrets/` in containers

## Local Development

For local development, use the `.env.local` file with direct file paths:

```env
# .env.local
FIREBASE_PROJECT_ID=your-firebase-project-id
FIREBASE_CREDENTIALS_PATH=./firebase-adminsdk.json
```

The application will automatically detect the environment and use the appropriate configuration.

## Troubleshooting

### Issue: "FIREBASE_CREDENTIALS_PATH not set, creating mock provider"

**Cause:** The environment variable is not set correctly.

**Solution:**
1. Verify the secret exists: `docker secret ls | grep firebase_credentials`
2. Verify the service has the secret in docker-compose.yml
3. Verify the environment variable is set correctly

### Issue: "Failed to read Firebase credentials file"

**Cause:** The secret file cannot be read.

**Solution:**
1. Check the secret content: `docker secret inspect firebase_credentials`
2. Verify the secret was created from a valid JSON file
3. Check container logs for more details

### Issue: "Failed to parse Firebase credentials"

**Cause:** The JSON file is malformed.

**Solution:**
1. Validate the JSON file locally: `cat firebase-adminsdk.json | jq .`
2. Recreate the secret with valid JSON

## References

- [Docker Secrets Documentation](https://docs.docker.com/engine/swarm/secrets/)
- [Firebase Admin SDK Documentation](https://firebase.google.com/docs/admin/setup)
- [Go Firebase SDK](https://firebase.google.com/docs/admin/setup#initialize-sdk)
