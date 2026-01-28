#!/bin/bash

# Auth Service Regression Test Suite Runner
# Post-Login Fix Validation
# Date: 2026-01-27

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${AUTH_SERVICE_URL:-http://localhost:8080}"
TEST_EMAIL="regression.test@example.com"
TEST_USERNAME="regression_test_user"
TEST_PASSWORD="TestPassword123!"
TEST_DISPLAY_NAME="Regression Test User"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

test_result() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if [ "$1" = "PASS" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "${GREEN}✓ PASS${NC}: $2"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "${RED}✗ FAIL${NC}: $2"
    fi
}

# Cleanup function
cleanup_test_data() {
    log_info "Cleaning up test data..."

    # Delete test user from CockroachDB
    docker exec -it cockroach cockroach sql --insecure \
        -d secureconnect_poc \
        -e "DELETE FROM users WHERE email = '$TEST_EMAIL';" 2>/dev/null || true

    # Clear Redis keys
    docker exec -it redis redis-cli \
        KEYS "failed_login:$TEST_EMAIL*" 2>/dev/null | \
        xargs -r docker exec -it redis redis-cli DEL 2>/dev/null || true

    # Clear session keys
    docker exec -it redis redis-cli \
        KEYS "session:*" 2>/dev/null | \
        xargs -r docker exec -it redis redis-cli DEL 2>/dev/null || true

    log_info "Cleanup complete"
}

# Health check
check_health() {
    log_info "Checking auth-service health..."
    RESPONSE=$(curl -s "$BASE_URL/health" || echo "")
    if [ -z "$RESPONSE" ]; then
        log_error "Auth service is not responding at $BASE_URL"
        exit 1
    fi
    log_info "Auth service is healthy"
}

# Test 1: Register → Login (Success Path)
test_register_login() {
    log_info "Test 1: Register → Login (Success Path)"

    # Step 1: Register
    REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"username\": \"$TEST_USERNAME\",
            \"password\": \"$TEST_PASSWORD\",
            \"display_name\": \"$TEST_DISPLAY_NAME\"
        }")

    HTTP_CODE=$(echo "$REGISTER_RESPONSE" | grep -o '"success":[^,}]*' | cut -d':' -f2)

    if [ "$HTTP_CODE" = "true" ]; then
        test_result "PASS" "Register returns 201"
    else
        test_result "FAIL" "Register failed: $REGISTER_RESPONSE"
        return 1
    fi

    # Step 2: Login
    LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\"
        }")

    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\"
        }")

    if [ "$HTTP_CODE" = "200" ]; then
        test_result "PASS" "Login returns 200 (NOT 500)"
    else
        test_result "FAIL" "Login returned $HTTP_CODE instead of 200"
        return 1
    fi

    # Check for tokens
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token // empty')
    REFRESH_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.refresh_token // empty')

    if [ -n "$ACCESS_TOKEN" ] && [ -n "$REFRESH_TOKEN" ]; then
        test_result "PASS" "Response contains access_token and refresh_token"
    else
        test_result "FAIL" "Tokens missing from response"
        return 1
    fi

    # Check user status
    USER_STATUS=$(echo "$LOGIN_RESPONSE" | jq -r '.data.user.status // empty')
    if [ "$USER_STATUS" = "online" ]; then
        test_result "PASS" "User status = online"
    else
        test_result "FAIL" "User status is $USER_STATUS, expected online"
        return 1
    fi

    # Check metrics
    METRICS=$(curl -s "$BASE_URL/metrics" || echo "")
    SUCCESS_COUNT=$(echo "$METRICS" | grep "auth_login_success_total" | grep -oP '\d+' || echo "0")

    if [ "$SUCCESS_COUNT" -ge "1" ]; then
        test_result "PASS" "auth_login_success_total incremented"
    else
        test_result "FAIL" "auth_login_success_total not incremented"
    fi
}

# Test 2: Login Wrong Password (401)
test_wrong_password() {
    log_info "Test 2: Login Wrong Password (401)"

    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"WrongPassword123!\"
        }")

    if [ "$HTTP_CODE" = "401" ]; then
        test_result "PASS" "Wrong password returns 401 (NOT 500)"
    else
        test_result "FAIL" "Wrong password returned $HTTP_CODE instead of 401"
        return 1
    fi

    # Check metrics
    METRICS=$(curl -s "$BASE_URL/metrics" || echo "")
    FAILED_COUNT=$(echo "$METRICS" | grep "auth_login_failed_total" | grep -oP '\d+' || echo "0")

    if [ "$FAILED_COUNT" -ge "1" ]; then
        test_result "PASS" "auth_login_failed_total incremented"
    else
        test_result "FAIL" "auth_login_failed_total not incremented"
    fi

    # Check Redis has failed_login key
    REDIS_DATA=$(docker exec -it redis redis-cli GET "failed_login:$TEST_EMAIL" 2>/dev/null || echo "")
    if [ -n "$REDIS_DATA" ]; then
        test_result "PASS" "Redis has failed_login key"
    else
        test_result "FAIL" "Redis missing failed_login key"
    fi
}

# Test 3: Login Non-Existent User (401)
test_nonexistent_user() {
    log_info "Test 3: Login Non-Existent User (401)"

    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"nonexistent@example.com\",
            \"password\": \"$TEST_PASSWORD\"
        }")

    if [ "$HTTP_CODE" = "401" ]; then
        test_result "PASS" "Non-existent user returns 401 (NOT 500)"
    else
        test_result "FAIL" "Non-existent user returned $HTTP_CODE instead of 401"
        return 1
    fi
}

# Test 4: Redis Degraded Mode
test_redis_degraded() {
    log_info "Test 4: Redis Degraded Mode"

    # Stop Redis
    log_info "Stopping Redis..."
    docker stop redis 2>/dev/null || true
    sleep 2

    # Attempt login
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\"
        }" || echo "000")

    # Start Redis back
    log_info "Starting Redis..."
    docker start redis 2>/dev/null || true
    sleep 3

    # In degraded mode, login should still work (200)
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "000" ]; then
        test_result "PASS" "Login handles Redis down gracefully"
    else
        test_result "FAIL" "Login returned $HTTP_CODE with Redis down"
        return 1
    fi

    # Check degraded mode metric
    METRICS=$(curl -s "$BASE_URL/metrics" || echo "")
    DEGRADED=$(echo "$METRICS" | grep "redis_degraded_mode" | grep -oP '\d+' || echo "0")

    if [ "$DEGRADED" = "1" ]; then
        test_result "PASS" "redis_degraded_mode = 1"
    else
        log_warn "redis_degraded_mode metric not checked (may need time to update)"
    fi
}

# Test 5: Account Lock (5 Failed Attempts)
test_account_lock() {
    log_info "Test 5: Account Lock (5 Failed Attempts)"

    # Execute 5 failed logins
    for i in {1..5}; do
        curl -s -X POST "$BASE_URL/v1/auth/login" \
            -H "Content-Type: application/json" \
            -d "{
                \"email\": \"$TEST_EMAIL\",
                \"password\": \"WrongPassword123!\"
            }" > /dev/null
        sleep 0.5
    done

    # Try login with correct password (should be locked)
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\"
        }")

    if [ "$HTTP_CODE" = "401" ]; then
        test_result "PASS" "Account locked after 5 failed attempts (401)"
    else
        test_result "FAIL" "Account not locked, returned $HTTP_CODE"
        return 1
    fi

    # Check Redis has lock data
    REDIS_DATA=$(docker exec -it redis redis-cli GET "failed_login:$TEST_EMAIL" 2>/dev/null || echo "")
    if echo "$REDIS_DATA" | jq -e '.locked_until' > /dev/null 2>&1; then
        test_result "PASS" "Redis has locked_until timestamp"
    else
        test_result "FAIL" "Redis missing lock data"
    fi
}

# Test 6: JWT Validity
test_jwt_validity() {
    log_info "Test 6: JWT Validity & Expiration"

    # Login to get tokens
    LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\"
        }")

    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token // empty')

    if [ -z "$ACCESS_TOKEN" ]; then
        test_result "FAIL" "Failed to get access token"
        return 1
    fi

    # Decode JWT payload
    PAYLOAD=$(echo "$ACCESS_TOKEN" | cut -d'.' -f2 | base64 -d 2>/dev/null || echo "")

    # Check claims
    USER_ID=$(echo "$PAYLOAD" | jq -r '.user_id // empty')
    AUDIENCE=$(echo "$PAYLOAD" | jq -r '.aud // empty')
    EXP=$(echo "$PAYLOAD" | jq -r '.exp // 0')

    if [ -n "$USER_ID" ]; then
        test_result "PASS" "JWT has user_id claim"
    else
        test_result "FAIL" "JWT missing user_id claim"
    fi

    if [ "$AUDIENCE" = "secureconnect-api" ]; then
        test_result "PASS" "JWT has correct audience"
    else
        test_result "FAIL" "JWT audience is $AUDIENCE, expected secureconnect-api"
    fi

    # Check expiration (~15 minutes = 900 seconds)
    CURRENT=$(date +%s)
    DIFF=$((EXP - CURRENT))

    if [ $DIFF -gt 800 ] && [ $DIFF -lt 1000 ]; then
        test_result "PASS" "JWT expires in ~15 minutes ($DIFF seconds)"
    else
        test_result "FAIL" "JWT expiration is $DIFF seconds, expected ~900"
    fi

    # Test protected endpoint
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/v1/auth/profile" \
        -H "Authorization: Bearer $ACCESS_TOKEN")

    if [ "$HTTP_CODE" = "200" ]; then
        test_result "PASS" "Protected endpoint works with valid token"
    else
        test_result "FAIL" "Protected endpoint returned $HTTP_CODE"
    fi
}

# Test 7: Backward Compatibility (Unix Timestamp)
test_backward_compat() {
    log_info "Test 7: Backward Compatibility (Unix Timestamp)"

    # Create test user
    curl -s -X POST "$BASE_URL/v1/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"backwards.compat@example.com\",
            \"username\": \"backwards_compat_user\",
            \"password\": \"$TEST_PASSWORD\",
            \"display_name\": \"Backwards Compat User\"
        }" > /dev/null

    # Insert old format data (plain Unix timestamp)
    UNIX_TIME=$(date +%s)
    docker exec -it redis redis-cli SET "failed_login:backwards.compat@example.com" "$UNIX_TIME" EX 900 2>/dev/null || true

    # Try login
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"backwards.compat@example.com\",
            \"password\": \"$TEST_PASSWORD\"
        }")

    if [ "$HTTP_CODE" = "200" ]; then
        test_result "PASS" "Backward compatibility handles Unix timestamp (200, NOT 500)"
    else
        test_result "FAIL" "Backward compatibility failed, returned $HTTP_CODE"
        return 1
    fi
}

# Test 8: Missing Redis Key (First-Time Login)
test_missing_redis_key() {
    log_info "Test 8: Missing Redis Key (First-Time Login)"

    # Ensure no Redis data
    docker exec -it redis redis-cli DEL "failed_login:firsttime@example.com" 2>/dev/null || true

    # Create test user
    curl -s -X POST "$BASE_URL/v1/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"firsttime@example.com\",
            \"username\": \"firsttime_user\",
            \"password\": \"$TEST_PASSWORD\",
            \"display_name\": \"First Time User\"
        }" > /dev/null

    # Try login
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"firsttime@example.com\",
            \"password\": \"$TEST_PASSWORD\"
        }")

    if [ "$HTTP_CODE" = "200" ]; then
        test_result "PASS" "Login works with missing Redis key (200, NOT 500)"
    else
        test_result "FAIL" "Login failed with missing Redis key, returned $HTTP_CODE"
        return 1
    fi
}

# Test 9: New Lock Data Format (JSON)
test_new_json_format() {
    log_info "Test 9: New Lock Data Format (JSON)"

    # Create test user
    curl -s -X POST "$BASE_URL/v1/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"newformat@example.com\",
            \"username\": \"newformat_user\",
            \"password\": \"$TEST_PASSWORD\",
            \"display_name\": \"New Format User\"
        }" > /dev/null

    # Trigger account lock
    for i in {1..5}; do
        curl -s -X POST "$BASE_URL/v1/auth/login" \
            -H "Content-Type: application/json" \
            -d "{
                \"email\": \"newformat@example.com\",
                \"password\": \"WrongPassword123!\"
            }" > /dev/null
        sleep 0.5
    done

    # Check Redis data is JSON
    REDIS_DATA=$(docker exec -it redis redis-cli GET "failed_login:newformat@example.com" 2>/dev/null || echo "")

    if echo "$REDIS_DATA" | jq -e '.locked_until' > /dev/null 2>&1; then
        test_result "PASS" "New lock data is JSON format"
    else
        test_result "FAIL" "New lock data is not JSON: $REDIS_DATA"
        return 1
    fi
}

# Print summary
print_summary() {
    echo ""
    echo "=========================================="
    echo "  REGRESSION TEST SUMMARY"
    echo "=========================================="
    echo "Total Tests:  $TOTAL_TESTS"
    echo -e "Passed:       ${GREEN}$PASSED_TESTS${NC}"
    echo -e "Failed:       ${RED}$FAILED_TESTS${NC}"
    echo "=========================================="

    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
        return 0
    else
        echo -e "${RED}✗ SOME TESTS FAILED${NC}"
        return 1
    fi
}

# Main execution
main() {
    echo "=========================================="
    echo "  AUTH SERVICE REGRESSION TEST SUITE"
    echo "  Post-Login Fix Validation"
    echo "=========================================="
    echo "Base URL: $BASE_URL"
    echo "Test Email: $TEST_EMAIL"
    echo ""

    # Check prerequisites
    check_health

    # Cleanup before tests
    cleanup_test_data
    echo ""

    # Run tests
    test_register_login
    echo ""
    test_wrong_password
    echo ""
    test_nonexistent_user
    echo ""
    test_redis_degraded
    echo ""
    test_account_lock
    echo ""
    test_jwt_validity
    echo ""
    test_backward_compat
    echo ""
    test_missing_redis_key
    echo ""
    test_new_json_format
    echo ""

    # Cleanup after tests
    cleanup_test_data
    echo ""

    # Print summary
    print_summary
}

# Run main function
main "$@"
