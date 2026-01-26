# Firebase Production Integration Verification Checklist

## Overview
This checklist provides a comprehensive verification process for Firebase Admin SDK integration in production using Docker secrets. It ensures that Firebase credentials are securely loaded without committing secrets to Git.

---

## Prerequisites

### 1. Firebase Service Account
- [ ] Download Firebase Admin SDK service account JSON from Firebase Console
  - Go to: https://console.firebase.google.com/project/_/settings/serviceaccounts/adminsdk
  - Click "Generate new private key"
  - Save as `firebase-service-account.json` (DO NOT commit to Git)

### 2. Docker Secrets Setup
- [ ] Ensure Docker Swarm is initialized (required for secrets)
  ```bash
  docker swarm init
  ```

---

## Phase 1: Docker Secrets Creation

### 1.1 Create Firebase Project ID Secret
```bash
# Command to create the secret
echo "your-firebase-project-id" | docker secret create firebase_project_id -

# Verification
docker secret ls | grep firebase_project_id
```
**PASS Criteria:** Secret `firebase_project_id` appears in `docker secret ls` output

### 1.2 Create Firebase Credentials Secret
```bash
# Command to create the secret (from file)
cat firebase-service-account.json | docker secret create firebase_credentials -

# Verification
docker secret ls | grep firebase_credentials
```
**PASS Criteria:** Secret `firebase_credentials` appears in `docker secret ls` output

### 1.3 Verify Secrets Content
```bash
# Verify project ID secret (should echo your project ID)
docker secret inspect firebase_project_id --format '{{.Spec.Name}}: {{.Spec.Data}}'

# Verify credentials secret is not empty
docker secret inspect firebase_credentials --format '{{len .Spec.Data}}'
```
**PASS Criteria:** Credentials secret shows non-zero byte count

---

## Phase 2: Docker Compose Configuration

### 2.1 Verify Secrets Definition in docker-compose.production.yml
- [ ] Check that secrets are defined at the top level:
  ```yaml
  secrets:
    firebase_project_id:
      external: true
    firebase_credentials:
      external: true
  ```
**PASS Criteria:** Both secrets defined with `external: true`

### 2.2 Verify video-service Configuration
- [ ] Check that video-service mounts the secrets:
  ```yaml
  video-service:
    secrets:
      - firebase_project_id
      - firebase_credentials
    environment:
      - FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id
      - FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials
      - PUSH_PROVIDER=firebase
  ```
**PASS Criteria:** All three environment variables present

---

## Phase 3: Go Code Verification

### 3.1 Verify Firebase Provider Fail-Fast Logic
- [ ] Check `pkg/push/firebase.go` for production mode checks:
  - [ ] Line ~32: `productionMode := os.Getenv("ENV") == "production"`
  - [ ] Line ~40-44: Log.Fatal if credentials path missing in production
  - [ ] Line ~56-58: Log.Fatal if file read fails in production
  - [ ] Line ~108-110: Log.Fatal if project ID empty in production
  - [ ] Line ~123-125: Log.Fatal if Firebase app init fails in production
  - [ ] Line ~137-139: Log.Fatal if messaging client init fails in production

**PASS Criteria:** All 6 fail-fast checks present

### 3.2 Verify Startup Validation
- [ ] Check `pkg/push/firebase.go` for `StartupCheck` function:
  - [ ] Function exists and validates provider state
  - [ ] Returns error in production if not initialized
  - [ ] Logs success message when validation passes

**PASS Criteria:** `StartupCheck` function implemented correctly

### 3.3 Verify video-service Initialization
- [ ] Check `cmd/video-service/main.go`:
  - [ ] Line ~150-154: Fail-fast if FIREBASE_PROJECT_ID missing in production
  - [ ] Line ~160-163: Fail-fast if credentials file missing in production
  - [ ] Line ~170-176: Calls `push.StartupCheck` after provider creation
  - [ ] Line ~180-184: Fail-fast if PUSH_PROVIDER=mock in production

**PASS Criteria:** All 4 validation checks present

---

## Phase 4: Build and Deploy

### 4.1 Build Production Images
```bash
cd secureconnect-backend
docker-compose -f docker-compose.production.yml build video-service
```
**PASS Criteria:** Build completes without errors

### 4.2 Start Services
```bash
docker-compose -f docker-compose.production.yml up -d video-service
```
**PASS Criteria:** Container starts successfully

### 4.3 Check Container Logs for Firebase Initialization
```bash
docker logs video-service --tail 50
```
**Expected Output:**
```
✅ Firebase Admin SDK initialized successfully: project_id=<your-project-id>, credentials_path=/run/secrets/firebase_credentials
✅ Firebase startup check passed: project_id=<your-project-id>, initialized=true
```
**PASS Criteria:** Both success messages appear in logs

---

## Phase 5: Runtime Verification

### 5.1 Verify Firebase Provider is Initialized
```bash
# Check container logs for initialization message
docker logs video-service | grep "Firebase Admin SDK initialized successfully"
```
**PASS Criteria:** Message found with correct project_id

### 5.2 Verify No Mock Provider in Production
```bash
# Ensure no mock provider warnings
docker logs video-service | grep -i "mock"
```
**PASS Criteria:** No output (or only development mode messages)

### 5.3 Verify Health Check
```bash
curl http://localhost:8083/health
```
**PASS Criteria:** Returns `{"status":"healthy","service":"video-service",...}`

---

## Phase 6: End-to-End Testing

### 6.1 Test Push Notification (Video Call)
```bash
# Trigger a video call via API (requires valid JWT)
curl -X POST http://localhost:8080/v1/calls/initiate \
  -H "Authorization: Bearer <valid-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"recipient_id": "<user-id>", "type": "video"}'
```
**PASS Criteria:** Video call initiated successfully

### 6.2 Verify Firebase Logs
```bash
# Check logs for Firebase send operations
docker logs video-service | grep "Firebase messages sent"
```
**PASS Criteria:** Logs show Firebase send attempts

---

## Phase 7: Security Validation

### 7.1 Verify No Credentials in Codebase
```bash
# Search for Firebase credentials in Go files
grep -r "firebase.*json" secureconnect-backend/pkg/push/
grep -r "type.*service_account" secureconnect-backend/pkg/push/
```
**PASS Criteria:** No hardcoded credentials found

### 7.2 Verify No Credentials in Git History
```bash
# Check if credentials were ever committed
git log --all --full-history --source -- "*firebase*" | grep -i "private.*key"
```
**PASS Criteria:** No Firebase private keys in Git history

### 7.3 Verify .gitignore Excludes Credentials
```bash
cat .gitignore | grep -i firebase
```
**PASS Criteria:** `.gitignore` contains patterns like `*.json` or `firebase*`

---

## Phase 8: Failure Scenario Testing

### 8.1 Test Missing Credentials (Production Mode)
```bash
# Remove secrets temporarily
docker secret rm firebase_credentials firebase_project_id

# Try to start service (should fail)
docker-compose -f docker-compose.production.yml up video-service
```
**Expected Output:**
```
❌ FIREBASE_CREDENTIALS_PATH not set. Required in production mode.
❌ Please create Docker secret: echo 'your-firebase-credentials' | docker secret create firebase_credentials -
❌ Fatal: Firebase credentials required in production mode
```
**PASS Criteria:** Container exits with error

### 8.2 Test Mock Provider Rejection (Production Mode)
```bash
# Set PUSH_PROVIDER=mock in docker-compose.production.yml
# Try to start service
docker-compose -f docker-compose.production.yml up video-service
```
**Expected Output:**
```
❌ ERROR: PUSH_PROVIDER=mock is not allowed in production mode!
❌ Please set PUSH_PROVIDER=firebase and configure Firebase credentials
❌ Fatal: Mock push provider not allowed in production
```
**PASS Criteria:** Container exits with error

### 8.3 Test Development Mode (Mock Provider)
```bash
# Set ENV=development and PUSH_PROVIDER=mock
docker-compose -f docker-compose.production.yml up video-service
```
**Expected Output:**
```
ℹ️  Using MockProvider for push notifications (development mode)
```
**PASS Criteria:** Container starts with mock provider

---

## Phase 9: CI/CD Integration

### 9.1 Automated Verification Script
```bash
#!/bin/bash
# verify-firebase-integration.sh

set -e

echo "=== Firebase Production Integration Verification ==="

# Check secrets exist
docker secret ls | grep -q firebase_project_id || { echo "❌ firebase_project_id secret missing"; exit 1; }
docker secret ls | grep -q firebase_credentials || { echo "❌ firebase_credentials secret missing"; exit 1; }

# Check docker-compose configuration
grep -q "firebase_project_id" docker-compose.production.yml || { echo "❌ firebase_project_id not in compose"; exit 1; }
grep -q "firebase_credentials" docker-compose.production.yml || { echo "❌ firebase_credentials not in compose"; exit 1; }
grep -q "PUSH_PROVIDER=firebase" docker-compose.production.yml || { echo "❌ PUSH_PROVIDER not set to firebase"; exit 1; }

# Check Go code for fail-fast logic
grep -q "productionMode := os.Getenv" pkg/push/firebase.go || { echo "❌ Missing production mode check"; exit 1; }
grep -q "log.Fatal" pkg/push/firebase.go || { echo "❌ Missing fail-fast logging"; exit 1; }

echo "✅ All verification checks passed!"
```

### 9.2 Run Automated Verification
```bash
chmod +x verify-firebase-integration.sh
./verify-firebase-integration.sh
```
**PASS Criteria:** Script exits with code 0 and prints "✅ All verification checks passed!"

---

## Troubleshooting Guide

### Issue: Container exits immediately
**Symptoms:** `docker logs video-service` shows "Fatal: Firebase credentials required"
**Solution:**
1. Verify secrets exist: `docker secret ls`
2. Recreate secrets if missing
3. Restart service: `docker-compose -f docker-compose.production.yml restart video-service`

### Issue: Firebase initialization fails
**Symptoms:** "Failed to initialize Firebase app" in logs
**Solution:**
1. Verify credentials file format is valid JSON
2. Check project ID matches Firebase project
3. Verify service account has proper permissions

### Issue: Push notifications not working
**Symptoms:** Video calls initiated but no notifications sent
**Solution:**
1. Check logs for Firebase send errors
2. Verify device tokens are registered
3. Test with Firebase Console to verify project setup

---

## Summary

### PASS/FAIL Criteria Summary

| Phase | Check | PASS Criteria |
|-------|-------|--------------|
| 1 | Secrets Created | Both secrets appear in `docker secret ls` |
| 2 | Compose Config | All required env vars present in video-service |
| 3 | Go Code | All 6 fail-fast checks present |
| 4 | Build/Deploy | Container starts without errors |
| 5 | Runtime | Firebase initialization message in logs |
| 6 | E2E Test | Video call initiates successfully |
| 7 | Security | No credentials in codebase or Git |
| 8 | Failure Tests | Container fails as expected |
| 9 | CI/CD | Automated script passes |

### Final Checklist
- [ ] All 9 phases completed
- [ ] All PASS criteria met
- [ ] No security issues found
- [ ] Documentation updated

---

## Appendix: Docker Secret Creation Commands

```bash
# Complete setup script
#!/bin/bash
# setup-firebase-secrets.sh

echo "Creating Firebase Docker secrets..."

# 1. Create project ID secret
read -p "Enter Firebase Project ID: " PROJECT_ID
echo "$PROJECT_ID" | docker secret create firebase_project_id -

# 2. Create credentials secret
read -p "Enter path to service account JSON: " CREDENTIALS_PATH
cat "$CREDENTIALS_PATH" | docker secret create firebase_credentials -

# 3. Verify secrets
echo "Created secrets:"
docker secret ls | grep firebase

echo "✅ Firebase secrets created successfully!"
```

---

## References
- Firebase Admin SDK: https://firebase.google.com/docs/admin/setup
- Docker Secrets: https://docs.docker.com/engine/swarm/secrets/
- Docker Compose: https://docs.docker.com/compose/compose-file/compose-file-v3/#secrets
