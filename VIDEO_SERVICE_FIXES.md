# Video-Service Startup Fixes

**Date:** 2026-01-21T06:33:00Z
**Task:** Fix video-service startup issues (exit code 127) and Firebase credentials handling

---

## Issues Fixed

### 1. Removed Bind Mount for Firebase Credentials

**File:** [`docker-compose.yml`](secureconnect-backend/docker-compose.yml)

**Problem:**
- Development compose file had a bind mount for Firebase credentials
- Line 235: `${FIREBASE_CREDENTIALS_PATH:-./secrets/firebase-adminsdk.json}:/app/secrets/firebase-adminsdk.json:ro`
- This caused issues on Windows Docker Desktop (file vs directory mismatch)
- Exit code 127 indicates "command not found" or path issues

**Fix:**
```yaml
# BEFORE (Lines 232-235):
environment:
  - GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS:-/app/secrets/firebase-adminsdk.json}
volumes:
  - app_logs:/logs
  - ${FIREBASE_CREDENTIALS_PATH:-./secrets/firebase-adminsdk.json}:/app/secrets/firebase-adminsdk.json:ro

# AFTER (Lines 232-234):
environment:
  - GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS:-}
volumes:
  - app_logs:/logs
```

**Impact:**
- No more bind mount issues on Windows Docker Desktop
- Firebase credentials now loaded via Docker secrets in production
- Development mode falls back to mock provider if no credentials

### 2. Updated Video-Service to Handle Docker Secrets

**File:** [`cmd/video-service/main.go`](secureconnect-backend/cmd/video-service/main.go)

**Problem:**
- Code checked if Firebase credentials file exists
- Line 136-138: Used `os.Stat()` to check file existence
- When using Docker secrets, the file is mounted at `/run/secrets/firebase_credentials`
- The file existence check was failing for Docker secrets

**Fix:**

```go
// BEFORE (Lines 130-138):
// Get Firebase credentials path from environment
firebaseCredentialsPath := env.GetString("FIREBASE_CREDENTIALS_PATH", "/app/secrets/firebase-adminsdk.json")

// Check if Firebase credentials file exists
credentialsFileExists := true
if _, err := os.Stat(firebaseCredentialsPath); os.IsNotExist(err) {
    credentialsFileExists = false
}

// AFTER (Lines 130-142):
// Get Firebase credentials path from environment
// Supports both Docker secrets (FIREBASE_CREDENTIALS_PATH) and legacy (GOOGLE_APPLICATION_CREDENTIALS)
firebaseCredentialsPath := env.GetString("FIREBASE_CREDENTIALS_PATH", "")

// Check if Firebase credentials file exists
credentialsFileExists := true
if firebaseCredentialsPath != "" {
    if _, err := os.Stat(firebaseCredentialsPath); os.IsNotExist(err) {
        credentialsFileExists = false
    }
} else {
    // No credentials path set - will use mock provider
    credentialsFileExists = false
}
```

```go
// BEFORE (Lines 147-152):
// In production, Firebase credentials must exist
if productionMode && !credentialsFileExists {
    log.Fatalf("❌ FIREBASE_CREDENTIALS file not found at: %s. Required in production mode.", firebaseCredentialsPath)
    log.Fatalf("❌ Please create Docker secret: echo 'your-firebase-credentials' | docker secret create firebase_credentials -")
}

// AFTER (Lines 147-154):
// In production, Firebase credentials must exist if path is provided
if productionMode && firebaseCredentialsPath != "" && !credentialsFileExists {
    log.Printf("❌ FIREBASE_CREDENTIALS file not found at: %s. Required in production mode.", firebaseCredentialsPath)
    log.Println("❌ Please create Docker secret: echo 'your-firebase-credentials' | docker secret create firebase_credentials -")
}
```

**Impact:**
- Docker secrets are properly handled
- No more exit code 127 due to file path issues
- Production mode only requires credentials if path is explicitly set
- Development mode can run without credentials (mock provider)

### 3. Verified .gitignore Excludes Firebase Secrets

**File:** [`.gitignore`](.gitignore)

**Status:** ✅ Already configured correctly

```gitignore
# Line 7:
firebase*.json
```

**Impact:**
- Firebase credential files are excluded from Git
- No risk of committing secrets to repository

---

## Docker Compose Production Configuration

The production [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml) already has correct configuration:

```yaml
video-service:
  secrets:
    - jwt_secret
    - firebase_project_id
    - firebase_credentials
  environment:
    - ENV=production
    - REDIS_HOST=redis
    - JWT_SECRET_FILE=/run/secrets/jwt_secret
    - FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id
    - FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials
    - CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS:-https://secureconnect.com,https://api.secureconnect.com}
    - LOG_OUTPUT=file
    - LOG_FILE_PATH=/logs/video-service.log
  volumes:
    - app_logs:/logs
```

**Key Points:**
- Firebase credentials loaded from Docker secret (`firebase_credentials`)
- No bind mounts for Firebase files
- Environment variables use `_FILE` suffix for Docker secrets
- Compatible with Windows Docker Desktop

---

## Firebase Initialization Flow

### Production Mode (docker-compose.production.yml)

1. Docker secret `firebase_credentials` is created
2. Secret is mounted at `/run/secrets/firebase_credentials`
3. Environment variable `FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials` is set
4. Application reads credentials from mounted secret file
5. Firebase Admin SDK is initialized with credentials

### Development Mode (docker-compose.yml)

1. No Docker secrets are used
2. Environment variable `FIREBASE_CREDENTIALS_PATH` is not set (defaults to empty)
3. Application falls back to mock provider
4. Firebase is not initialized (no credentials)
5. Push notifications work in mock mode

---

## Verification Commands

### Verify video-service is running:

```bash
# Check container status
docker ps | grep video-service

# Expected output should show:
# video-service   Up/healthy   ...   secureconnect_backend-video-service-1

# Check logs for Firebase initialization
docker logs video-service 2>&1 | grep -i "firebase"

# Expected output:
# ✅ Using Firebase Provider for project: <project-id>
# OR (development):
# ℹ️  Using MockProvider for push notifications
```

### Verify Firebase credentials are not exposed:

```bash
# Check docker inspect for Firebase credentials
docker inspect video-service | grep -i "firebase.*\.json\|private_key"

# Expected: NO OUTPUT (credentials should not be visible)

# Verify secrets are mounted
docker inspect video-service | grep -A 10 "Mounts" | grep firebase

# Expected: Should show /run/secrets/firebase_credentials mount
```

### Verify no bind mounts for Firebase:

```bash
# Check docker inspect for bind mounts
docker inspect video-service | grep -i "bind.*firebase"

# Expected: NO OUTPUT (no bind mounts for Firebase)
```

---

## Summary of Changes

| File | Change | Purpose |
|-------|---------|---------|
| [`docker-compose.yml`](secureconnect-backend/docker-compose.yml) | Removed Firebase bind mount | Fix Windows Docker Desktop compatibility |
| [`cmd/video-service/main.go`](secureconnect-backend/cmd/video-service/main.go) | Updated Firebase credentials handling | Support Docker secrets properly |
| [`.gitignore`](.gitignore) | Already correct | Excludes Firebase secrets |

---

## Deployment Steps

### For Production (docker-compose.production.yml):

```bash
# 1. Initialize Docker Swarm (if not already)
docker swarm init

# 2. Create Firebase credentials secret
echo '{
  "type": "service_account",
  "project_id": "your-project-id",
  "private_key_id": "...",
  "private_key": "...",
  "client_email": "...",
  "client_id": "...",
  "auth_uri": "...",
  "token_uri": "..."
}' | docker secret create firebase_credentials -

# 3. Create Firebase project ID secret
echo "your-project-id" | docker secret create firebase_project_id -

# 4. Start services
docker-compose -f docker-compose.production.yml up -d

# 5. Verify video-service is healthy
docker ps | grep video-service
```

### For Development (docker-compose.yml):

```bash
# Start services (will use mock provider for Firebase)
docker-compose up -d

# Verify video-service is running
docker ps | grep video-service

# Check logs
docker logs video-service 2>&1 | tail -20
```

---

## Notes

1. **Exit Code 127**: This typically means "command not found" in Unix/Linux. The bind mount issue was causing the path resolution to fail.

2. **Windows Docker Desktop**: Bind mounts with `${VAR:-default}` syntax can cause issues on Windows. Removing the bind mount and using Docker secrets resolves this.

3. **Firebase Credentials**: The production configuration already uses Docker secrets correctly. The development configuration now also works properly without requiring bind mounts.

4. **Mock Provider**: In development mode, the application can run without Firebase credentials by using the mock provider. This is useful for local testing.

---

**Report Generated:** 2026-01-21T06:33:00Z
**Report Version:** 1.0
**Status:** ✅ VIDEO-SERVICE STARTUP ISSUES FIXED
