# Auth Service Regression Test Suite
## Post-Login Fix Validation

**Date:** 2026-01-27
**Target:** auth-service
**Base URL:** `http://localhost:8080` (adjust as needed)
**Fix Applied:** Redis account lock serialization/deserialization fix in [`session_repo.go`](../internal/repository/redis/session_repo.go)

---

## Test Environment Setup

### Prerequisites
```bash
# Ensure services are running
docker ps | grep -E "(auth-service|redis|cockroach)"

# Check auth-service health
curl -s http://localhost:8080/health | jq .

# Check Prometheus metrics endpoint
curl -s http://localhost:8080/metrics | grep auth_
```

### Test User Credentials
```json
{
  "email": "regression.test@example.com",
  "username": "regression_test_user",
  "password": "TestPassword123!",
  "display_name": "Regression Test User"
}
```

### Cleanup (Run before each test suite)
```bash
# Delete test user from CockroachDB
docker exec -it cockroach cockroach sql --insecure \
  -d secureconnect_poc \
  -e "DELETE FROM users WHERE email = 'regression.test@example.com';"

# Clear Redis keys
docker exec -it redis redis-cli \
  KEYS "failed_login:regression.test@example.com*" | xargs redis-cli DEL

docker exec -it redis redis-cli \
  KEYS "session:*" | xargs redis-cli DEL

# Verify cleanup
docker exec -it redis redis-cli \
  KEYS "failed_login:*"
```

---

## Test Case 1: Register → Login (Success Path)

### Objective
Verify successful registration followed by successful login without 500 errors.

### Test Steps

#### Step 1: Register New User
```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "regression.test@example.com",
    "username": "regression_test_user",
    "password": "TestPassword123!",
    "display_name": "Regression Test User"
  }' | jq .
```

**Expected HTTP Code:** `201 Created`

**Expected Response:**
```json
{
  "success": true,
  "data": {
    "user": {
      "user_id": "<uuid>",
      "email": "regression.test@example.com",
      "username": "regression_test_user",
      "display_name": "Regression Test User",
      "status": "offline",
      "created_at": "<timestamp>",
      "updated_at": "<timestamp>"
    },
    "access_token": "<jwt_token>",
    "refresh_token": "<jwt_token>"
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

#### Step 2: Login with Correct Credentials
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "regression.test@example.com",
    "password": "TestPassword123!"
  }' | jq .
```

**Expected HTTP Code:** `200 OK`

**Expected Response:**
```json
{
  "success": true,
  "data": {
    "user": {
      "user_id": "<uuid>",
      "email": "regression.test@example.com",
      "username": "regression_test_user",
      "display_name": "Regression Test User",
      "status": "online",
      "created_at": "<timestamp>",
      "updated_at": "<timestamp>"
    },
    "access_token": "<jwt_token>",
    "refresh_token": "<jwt_token>"
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

### Expected Logs (auth-service)
```log
{"level":"info","service":"auth-service","msg":"User registered successfully","user_id":"<uuid>","email":"regression.test@example.com"}
{"level":"info","service":"auth-service","msg":"User logged in successfully","user_id":"<uuid>","email":"regression.test@example.com"}
```

### Expected Prometheus Metrics
```bash
curl -s http://localhost:8080/metrics | grep -E "auth_login_success_total|auth_login_failed_total"
```

**Expected Output:**
```
auth_login_success_total{service="auth-service"} 1
auth_login_failed_total{service="auth-service"} 0
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Register returns 201 | ✅ | ❌ |
| Login returns 200 (NOT 500) | ✅ | ❌ |
| Response contains access_token | ✅ | ❌ |
| Response contains refresh_token | ✅ | ❌ |
| User status = "online" | ✅ | ❌ |
| auth_login_success_total = 1 | ✅ | ❌ |
| No 500 error in logs | ✅ | ❌ |

---

## Test Case 2: Login Wrong Password (401)

### Objective
Verify incorrect password returns 401, not 500.

### Test Steps

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "regression.test@example.com",
    "password": "WrongPassword123!"
  }' -v
```

**Expected HTTP Code:** `401 Unauthorized`

**Expected Response:**
```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid email or password"
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

### Expected Logs (auth-service)
```log
{"level":"warn","service":"auth-service","msg":"Failed login attempt","email":"regression.test@example.com","ip":"<client_ip>","reason":"invalid credentials"}
```

### Expected Prometheus Metrics
```bash
curl -s http://localhost:8080/metrics | grep -E "auth_login_failed_total|auth_login_failed_by_ip"
```

**Expected Output:**
```
auth_login_failed_total{service="auth-service"} 1
auth_login_failed_by_ip{service="auth-service",ip="<client_ip>"} 1
```

### Expected Redis State
```bash
docker exec -it redis redis-cli GET "failed_login:regression.test@example.com"
```

**Expected Output:** JSON with attempts count
```json
{"user_id":"<uuid>","email":"regression.test@example.com","ip":"<client_ip>","attempts":1}
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Returns 401 (NOT 500) | ✅ | ❌ |
| Error message = "Invalid email or password" | ✅ | ❌ |
| auth_login_failed_total incremented | ✅ | ❌ |
| Redis has failed_login key | ✅ | ❌ |
| No panic in logs | ✅ | ❌ |

---

## Test Case 3: Login Non-Existent User (401)

### Objective
Verify login with non-existent email returns 401, not 500.

### Test Steps

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "nonexistent@example.com",
    "password": "TestPassword123!"
  }' -v
```

**Expected HTTP Code:** `401 Unauthorized`

**Expected Response:**
```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid email or password"
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

### Expected Logs (auth-service)
```log
{"level":"warn","service":"auth-service","msg":"Failed login attempt","email":"nonexistent@example.com","ip":"<client_ip>","reason":"user not found"}
```

### Expected Prometheus Metrics
```
auth_login_failed_total{service="auth-service"} 1
auth_login_failed_by_ip{service="auth-service",ip="<client_ip>"} 1
```

### Expected Redis State
```bash
docker exec -it redis redis-cli GET "failed_login:nonexistent@example.com"
```

**Expected Output:** JSON with attempts count (user_id = nil)
```json
{"user_id":"00000000-0000-0000-0000-000000000000","email":"nonexistent@example.com","ip":"<client_ip>","attempts":1}
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Returns 401 (NOT 500) | ✅ | ❌ |
| Error message = "Invalid email or password" | ✅ | ❌ |
| auth_login_failed_total incremented | ✅ | ❌ |
| Redis has failed_login key | ✅ | ❌ |
| No 500 error in logs | ✅ | ❌ |

---

## Test Case 4: Redis Down → Login Graceful Error

### Objective
Verify login handles Redis unavailability gracefully (degraded mode).

### Test Steps

#### Step 1: Stop Redis
```bash
docker stop redis
# Or simulate network failure
docker network disconnect bridge redis
```

#### Step 2: Attempt Login
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "regression.test@example.com",
    "password": "TestPassword123!"
  }' -v
```

**Expected HTTP Code:** `200 OK` (degraded mode allows login)

**Expected Response:**
```json
{
  "success": true,
  "data": {
    "user": {
      "user_id": "<uuid>",
      "email": "regression.test@example.com",
      "username": "regression_test_user",
      "display_name": "Regression Test User",
      "status": "online",
      "created_at": "<timestamp>",
      "updated_at": "<timestamp>"
    },
    "access_token": "<jwt_token>",
    "refresh_token": "<jwt_token>"
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

### Expected Logs (auth-service)
```log
{"level":"warn","service":"auth-service","msg":"Session storage skipped (Redis degraded)","user_id":"<uuid>"}
{"level":"warn","service":"auth-service","msg":"Failed to clear failed login attempts","email":"regression.test@example.com","error":"redis is in degraded mode"}
```

### Expected Prometheus Metrics
```bash
curl -s http://localhost:8080/metrics | grep redis_degraded_mode
```

**Expected Output:**
```
redis_degraded_mode 1
```

#### Step 3: Restore Redis
```bash
docker start redis
# Or reconnect network
docker network connect bridge redis

# Wait for health check to clear degraded mode
sleep 15

# Verify Redis is healthy
docker exec -it redis redis-cli PING
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Login returns 200 (NOT 500) | ✅ | ❌ |
| Tokens are generated | ✅ | ❌ |
| redis_degraded_mode = 1 | ✅ | ❌ |
| Warning logs about degraded mode | ✅ | ❌ |
| No panic or crash | ✅ | ❌ |
| Service recovers after Redis restart | ✅ | ❌ |

---

## Test Case 5: Multiple Failed Login Attempts → Account Lock

### Objective
Verify account locks after 5 failed attempts and returns 401.

### Test Steps

#### Step 1: Execute 5 Failed Logins
```bash
for i in {1..5}; do
  echo "Attempt $i"
  curl -X POST http://localhost:8080/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{
      "email": "regression.test@example.com",
      "password": "WrongPassword123!"
    }' -o /dev/null -w "HTTP %{http_code}\n"
  sleep 0.5
done
```

**Expected Output:**
```
Attempt 1
HTTP 401
Attempt 2
HTTP 401
Attempt 3
HTTP 401
Attempt 4
HTTP 401
Attempt 5
HTTP 401
```

#### Step 2: Verify Account is Locked
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "regression.test@example.com",
    "password": "TestPassword123!"
  }' -v
```

**Expected HTTP Code:** `401 Unauthorized`

**Expected Response:**
```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Account temporarily locked due to too many failed attempts. Please try again later."
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

### Expected Logs (auth-service)
```log
{"level":"warn","service":"auth-service","msg":"Failed login attempt","email":"regression.test@example.com","attempts":5}
{"level":"info","service":"auth-service","msg":"Account locked","email":"regression.test@example.com","locked_until":"<timestamp>"}
```

### Expected Prometheus Metrics
```bash
curl -s http://localhost:8080/metrics | grep -E "auth_login_failed_total|auth_account_locked_total"
```

**Expected Output:**
```
auth_login_failed_total{service="auth-service"} 5
auth_account_locked_total{service="auth-service"} 1
```

### Expected Redis State
```bash
docker exec -it redis redis-cli GET "failed_login:regression.test@example.com" | jq .
```

**Expected Output:** JSON with lock timestamp
```json
{
  "user_id":"<uuid>",
  "email":"regression.test@example.com",
  "ip":"<client_ip>",
  "attempts":5,
  "locked_until":"<iso8601_timestamp>"
}
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| 5 failed attempts = 401 each | ✅ | ❌ |
| 6th attempt (correct password) = 401 | ✅ | ❌ |
| Error message mentions "locked" | ✅ | ❌ |
| auth_account_locked_total = 1 | ✅ | ❌ |
| Redis has locked_until timestamp | ✅ | ❌ |
| No 500 error | ✅ | ❌ |

---

## Test Case 6: Login After Lock Expiry

### Objective
Verify account unlocks after 15 minutes (lock duration).

### Test Steps

#### Step 1: Manually Set Expired Lock
```bash
# Set lock time to 1 minute ago (simulating expired lock)
EXPIRED_TIME=$(date -u -d "1 minute ago" +"%Y-%m-%dT%H:%M:%SZ")

docker exec -it redis redis-cli SET "failed_login:regression.test@example.com" \
  '{"user_id":"<uuid>","email":"regression.test@example.com","ip":"<client_ip>","attempts":5,"locked_until":"'$EXPIRED_TIME'"}' \
  EX 900
```

#### Step 2: Attempt Login (Should Succeed)
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "regression.test@example.com",
    "password": "TestPassword123!"
  }' | jq .
```

**Expected HTTP Code:** `200 OK`

**Expected Response:** Same as Test Case 1 (successful login)

### Expected Logs (auth-service)
```log
{"level":"info","service":"auth-service","msg":"Account lock expired, allowing login","email":"regression.test@example.com"}
{"level":"info","service":"auth-service","msg":"User logged in successfully","user_id":"<uuid>"}
```

### Expected Redis State
```bash
docker exec -it redis redis-cli GET "failed_login:regression.test@example.com"
```

**Expected Output:** `(nil)` - lock cleared on successful login

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Login returns 200 (NOT 401) | ✅ | ❌ |
| Tokens generated | ✅ | ❌ |
| Failed login attempts cleared | ✅ | ❌ |
| No lock error in logs | ✅ | ❌ |
| auth_login_success_total incremented | ✅ | ❌ |

---

## Test Case 7: JWT Validity & Expiration

### Objective
Verify JWT tokens are valid and have correct expiration.

### Test Steps

#### Step 1: Login and Extract Tokens
```bash
# Store tokens in variables
RESPONSE=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "regression.test@example.com",
    "password": "TestPassword123!"
  }')

ACCESS_TOKEN=$(echo $RESPONSE | jq -r '.data.access_token')
REFRESH_TOKEN=$(echo $RESPONSE | jq -r '.data.refresh_token')

echo "Access Token: $ACCESS_TOKEN"
echo "Refresh Token: $REFRESH_TOKEN"
```

#### Step 2: Verify Access Token Structure
```bash
# Decode JWT header (without signature)
echo $ACCESS_TOKEN | cut -d'.' -f1 | base64 -d 2>/dev/null | jq .
```

**Expected Output:**
```json
{
  "alg": "HS256",
  "typ": "JWT"
}
```

#### Step 3: Verify Access Token Claims
```bash
# Decode JWT payload (without signature)
echo $ACCESS_TOKEN | cut -d'.' -f2 | base64 -d 2>/dev/null | jq .
```

**Expected Output:**
```json
{
  "user_id": "<uuid>",
  "email": "regression.test@example.com",
  "username": "regression_test_user",
  "role": "user",
  "aud": "secureconnect-api",
  "exp": <future_timestamp>,
  "iat": <current_timestamp>,
  "nbf": <current_timestamp>,
  "iss": "secureconnect-auth",
  "sub": "<user_id>",
  "jti": "<unique_id>"
}
```

#### Step 4: Verify Token Expiration (Access Token = 15 min)
```bash
# Get exp timestamp
EXP=$(echo $ACCESS_TOKEN | cut -d'.' -f2 | base64 -d 2>/dev/null | jq -r '.exp')
CURRENT=$(date +%s)
DIFF=$((EXP - CURRENT))

echo "Access token expires in $DIFF seconds"

# Verify ~15 minutes (900 seconds)
if [ $DIFF -gt 800 ] && [ $DIFF -lt 1000 ]; then
  echo "✅ Access token expiration correct (~15 min)"
else
  echo "❌ Access token expiration incorrect: $DIFF seconds"
fi
```

#### Step 5: Verify Refresh Token Expiration (30 days)
```bash
EXP=$(echo $REFRESH_TOKEN | cut -d'.' -f2 | base64 -d 2>/dev/null | jq -r '.exp')
CURRENT=$(date +%s)
DIFF=$((EXP - CURRENT))
DAYS=$((DIFF / 86400))

echo "Refresh token expires in $DAYS days"

# Verify ~30 days
if [ $DAYS -gt 28 ] && [ $DAYS -lt 32 ]; then
  echo "✅ Refresh token expiration correct (~30 days)"
else
  echo "❌ Refresh token expiration incorrect: $DAYS days"
fi
```

#### Step 6: Test Protected Endpoint with Valid Token
```bash
curl -X GET http://localhost:8080/v1/auth/profile \
  -H "Authorization: Bearer $ACCESS_TOKEN" | jq .
```

**Expected HTTP Code:** `200 OK`

**Expected Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "<uuid>",
    "email": "regression.test@example.com",
    "username": "regression_test_user",
    "role": "user"
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

#### Step 7: Test Protected Endpoint Without Token
```bash
curl -X GET http://localhost:8080/v1/auth/profile -v
```

**Expected HTTP Code:** `401 Unauthorized`

#### Step 8: Test Refresh Token
```bash
curl -X POST http://localhost:8080/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "'$REFRESH_TOKEN'"
  }' | jq .
```

**Expected HTTP Code:** `200 OK`

**Expected Response:**
```json
{
  "success": true,
  "data": {
    "access_token": "<new_jwt_token>",
    "refresh_token": "<new_jwt_token>"
  },
  "meta": {
    "timestamp": "<iso8601>"
  }
}
```

### Expected Prometheus Metrics
```bash
curl -s http://localhost:8080/metrics | grep -E "auth_refresh_token_success_total|auth_refresh_token_invalid_total"
```

**Expected Output:**
```
auth_refresh_token_success_total{service="auth-service"} 1
auth_refresh_token_invalid_total{service="auth-service"} 0
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Access token has HS256 algorithm | ✅ | ❌ |
| Access token has user_id claim | ✅ | ❌ |
| Access token has aud = "secureconnect-api" | ✅ | ❌ |
| Access token expires ~15 min | ✅ | ❌ |
| Refresh token expires ~30 days | ✅ | ❌ |
| Protected endpoint works with token | ✅ | ❌ |
| Protected endpoint fails without token | ✅ | ❌ |
| Refresh token generates new tokens | ✅ | ❌ |
| auth_refresh_token_success_total = 1 | ✅ | ❌ |

---

## Test Case 8: Backward Compatibility - Stale Unix Timestamp Data

### Objective
Verify fix handles old Unix timestamp format in Redis.

### Test Steps

#### Step 1: Manually Insert Stale Unix Timestamp Data
```bash
# Get current Unix timestamp
UNIX_TIME=$(date +%s)

# Set old format data (plain Unix timestamp string)
docker exec -it redis redis-cli SET "failed_login:backwards.compat@example.com" "$UNIX_TIME" EX 900

# Verify data is old format
docker exec -it redis redis-cli GET "failed_login:backwards.compat@example.com"
```

**Expected Output:** Plain number (e.g., `1737933600`)

#### Step 2: Create User
```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "backwards.compat@example.com",
    "username": "backwards_compat_user",
    "password": "TestPassword123!",
    "display_name": "Backwards Compat User"
  }' | jq .
```

#### Step 3: Attempt Login (Should Not Crash)
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "backwards.compat@example.com",
    "password": "TestPassword123!"
  }' -v
```

**Expected HTTP Code:** `200 OK` (login succeeds)

### Expected Logs (auth-service)
```log
{"level":"info","service":"auth-service","msg":"Parsed stale Unix timestamp format","email":"backwards.compat@example.com"}
{"level":"info","service":"auth-service","msg":"User logged in successfully","user_id":"<uuid>"}
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Login returns 200 (NOT 500) | ✅ | ❌ |
| No JSON unmarshal error in logs | ✅ | ❌ |
| Tokens generated | ✅ | ❌ |
| Old data handled gracefully | ✅ | ❌ |

---

## Test Case 9: Missing Redis Key (First-Time Login)

### Objective
Verify login works when Redis key doesn't exist (no lock data).

### Test Steps

#### Step 1: Ensure No Redis Data
```bash
docker exec -it redis redis-cli DEL "failed_login:firsttime@example.com"

# Verify key doesn't exist
docker exec -it redis redis-cli EXISTS "failed_login:firsttime@example.com"
```

**Expected Output:** `0`

#### Step 2: Create User
```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "firsttime@example.com",
    "username": "firsttime_user",
    "password": "TestPassword123!",
    "display_name": "First Time User"
  }' | jq .
```

#### Step 3: Login (Should Succeed)
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "firsttime@example.com",
    "password": "TestPassword123!"
  }' -v
```

**Expected HTTP Code:** `200 OK`

### Expected Logs (auth-service)
```log
{"level":"info","service":"auth-service","msg":"No account lock found","email":"firsttime@example.com"}
{"level":"info","service":"auth-service","msg":"User logged in successfully","user_id":"<uuid>"}
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Login returns 200 (NOT 500) | ✅ | ❌ |
| No "failed to get account lock" error | ✅ | ❌ |
| Tokens generated | ✅ | ❌ |
| No redis.Nil error in logs | ✅ | ❌ |

---

## Test Case 10: New Lock Data Format (JSON)

### Objective
Verify new account locks use JSON format.

### Test Steps

#### Step 1: Trigger Account Lock (5 Failed Attempts)
```bash
for i in {1..5}; do
  curl -s -X POST http://localhost:8080/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{
      "email": "newformat@example.com",
      "password": "WrongPassword123!"
    }' > /dev/null
  sleep 0.5
done
```

#### Step 2: Verify Redis Data is JSON Format
```bash
docker exec -it redis redis-cli GET "failed_login:newformat@example.com" | jq .
```

**Expected Output:** JSON object (not plain number)
```json
{
  "user_id":"<uuid>",
  "email":"newformat@example.com",
  "ip":"<client_ip>",
  "attempts":5,
  "locked_until":"<iso8601_timestamp>"
}
```

### PASS/FAIL Criteria

| Check | PASS | FAIL |
|-------|------|------|
| Redis data is JSON format | ✅ | ❌ |
| Has locked_until field | ✅ | ❌ |
| locked_until is ISO8601 format | ✅ | ❌ |
| Account is locked (401 on next attempt) | ✅ | ❌ |

---

## Summary Checklist

### All Tests Must Pass

| Test Case | Description | Status |
|-----------|-------------|--------|
| 1 | Register → Login (Success) | ⬜ |
| 2 | Login Wrong Password (401) | ⬜ |
| 3 | Login Non-Existent User (401) | ⬜ |
| 4 | Redis Down → Graceful Error | ⬜ |
| 5 | Multiple Failed Attempts → Lock | ⬜ |
| 6 | Login After Lock Expiry | ⬜ |
| 7 | JWT Validity & Expiration | ⬜ |
| 8 | Backward Compatibility (Unix Timestamp) | ⬜ |
| 9 | Missing Redis Key (First-Time Login) | ⬜ |
| 10 | New Lock Data Format (JSON) | ⬜ |

### Critical Success Criteria

- [ ] **NO 500 errors on login** (primary fix validation)
- [ ] All 401 responses for invalid credentials
- [ ] Account lock works correctly
- [ ] JWT tokens are valid and have correct expiration
- [ ] Redis degraded mode handled gracefully
- [ ] Backward compatibility maintained

### Run All Tests

```bash
# Run all tests sequentially
./secureconnect-backend/qa/run_auth_regression_tests.sh

# Or run individual tests
# See each test case above for curl commands
```

---

## Appendix: Helper Scripts

### View Auth Service Logs
```bash
docker logs auth-service --tail 100 -f
```

### View Redis Keys
```bash
docker exec -it redis redis-cli KEYS "failed_login:*"
```

### View Specific Redis Key
```bash
docker exec -it redis redis-cli GET "failed_login:regression.test@example.com" | jq .
```

### Clear All Failed Login Data
```bash
docker exec -it redis redis-cli KEYS "failed_login:*" | xargs redis-cli DEL
```

### Check Prometheus Metrics
```bash
curl -s http://localhost:8080/metrics | grep -E "auth_|redis_"
```

### Reset Test Environment
```bash
# Stop all services
docker-compose down

# Remove volumes (optional - destroys data)
docker-compose down -v

# Restart services
docker-compose up -d

# Wait for services to be healthy
sleep 30

# Verify health
curl http://localhost:8080/health
```
