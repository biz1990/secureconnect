#!/bin/bash
# verify-firebase-production.sh
# Production SRE Security Audit: Firebase Production Mode Verification
# This script verifies Firebase is running in REAL production mode
# with proper Docker secrets configuration.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track overall status
ALL_PASSED=true

# Helper functions
log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ALL_PASSED=false
}

log_section() {
    echo ""
    echo "========================================"
    echo "$1"
    echo "========================================"
}

# =============================================================================
# CHECK 1: No Secret Files in Repository
# =============================================================================
log_section "CHECK 1: Repository Security - No Secret Files"

SECRET_FILES=(
    "firebase-service-account.json"
    "firebase_credentials.json"
    "service-account.json"
    "*.p12"
    "*.key"
)

SECRETS_FOUND=false
for pattern in "${SECRET_FILES[@]}"; do
    if find . -name "$pattern" -type f 2>/dev/null | grep -q .; then
        log_fail "Found secret file matching pattern: $pattern"
        find . -name "$pattern" -type f 2>/dev/null
        SECRETS_FOUND=true
    fi
done

if [ "$SECRETS_FOUND" = false ]; then
    log_pass "No Firebase secret files found in repository"
else
    log_fail "FAIL: Secret files found in repository. Remove them immediately!"
    log_fail "File path to fix: Remove files matching patterns above"
fi

# =============================================================================
# CHECK 2: Docker Secrets Exist
# =============================================================================
log_section "CHECK 2: Docker Secrets Configuration"

if ! docker secret ls | grep -q "firebase_project_id"; then
    log_fail "Docker secret 'firebase_project_id' not found"
    log_fail "Fix: echo 'your-project-id' | docker secret create firebase_project_id -"
    ALL_PASSED=false
else
    log_pass "Docker secret 'firebase_project_id' exists"
fi

if ! docker secret ls | grep -q "firebase_credentials"; then
    log_fail "Docker secret 'firebase_credentials' not found"
    log_fail "Fix: cat firebase-service-account.json | docker secret create firebase_credentials -"
    ALL_PASSED=false
else
    log_pass "Docker secret 'firebase_credentials' exists"
fi

# =============================================================================
# CHECK 3: Docker Compose Configuration
# =============================================================================
log_section "CHECK 3: Docker Compose Environment Variables"

COMPOSE_FILE="docker-compose.production.yml"

if [ ! -f "$COMPOSE_FILE" ]; then
    log_fail "Docker compose file not found: $COMPOSE_FILE"
    ALL_PASSED=false
else
    # Check for PUSH_PROVIDER=firebase
    if grep -q "PUSH_PROVIDER=firebase" "$COMPOSE_FILE"; then
        log_pass "PUSH_PROVIDER=firebase configured in docker-compose.production.yml"
    else
        log_fail "PUSH_PROVIDER=firebase NOT found in docker-compose.production.yml"
        log_fail "Fix: Add '- PUSH_PROVIDER=firebase' to video-service environment"
        ALL_PASSED=false
    fi

    # Check for FIREBASE_CREDENTIALS_PATH
    if grep -q "FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials" "$COMPOSE_FILE"; then
        log_pass "FIREBASE_CREDENTIALS_PATH points to /run/secrets/firebase_credentials"
    else
        log_fail "FIREBASE_CREDENTIALS_PATH not pointing to /run/secrets/firebase_credentials"
        log_fail "Fix: Add '- FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials' to video-service"
        ALL_PASSED=false
    fi

    # Check for FIREBASE_PROJECT_ID_FILE
    if grep -q "FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id" "$COMPOSE_FILE"; then
        log_pass "FIREBASE_PROJECT_ID_FILE points to /run/secrets/firebase_project_id"
    else
        log_fail "FIREBASE_PROJECT_ID_FILE not pointing to /run/secrets/firebase_project_id"
        log_fail "Fix: Add '- FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id' to video-service"
        ALL_PASSED=false
    fi
fi

# =============================================================================
# CHECK 4: Go Code - Fail-Fast Logic
# =============================================================================
log_section "CHECK 4: Go Code - Production Fail-Fast Logic"

FIREBASE_GO="pkg/push/firebase.go"

if [ ! -f "$FIREBASE_GO" ]; then
    log_fail "Firebase provider file not found: $FIREBASE_GO"
    ALL_PASSED=false
else
    # Check for production mode detection
    if grep -q 'productionMode := os.Getenv("ENV") == "production"' "$FIREBASE_GO"; then
        log_pass "Production mode detection present in pkg/push/firebase.go"
    else
        log_fail "Production mode detection NOT found in pkg/push/firebase.go"
        log_fail "Fix: Add 'productionMode := os.Getenv(\"ENV\") == \"production\"'"
        ALL_PASSED=false
    fi

    # Check for fail-fast on missing credentials
    if grep -q 'log.Fatal("Firebase credentials required in production mode")' "$FIREBASE_GO"; then
        log_pass "Fail-fast for missing credentials present"
    else
        log_fail "Fail-fast for missing credentials NOT found"
        log_fail "Fix: Add log.Fatal() when credentials missing in production"
        ALL_PASSED=false
    fi

    # Check for fail-fast on initialization failure
    if grep -q 'log.Fatal("Firebase initialization failed in production mode")' "$FIREBASE_GO"; then
        log_pass "Fail-fast for Firebase init failure present"
    else
        log_fail "Fail-fast for Firebase init failure NOT found"
        log_fail "Fix: Add log.Fatal() when Firebase init fails in production"
        ALL_PASSED=false
    fi
fi

# =============================================================================
# CHECK 5: Video Service - Mock Provider Rejection
# =============================================================================
log_section "CHECK 5: Video Service - Mock Provider Rejection"

VIDEO_MAIN="cmd/video-service/main.go"

if [ ! -f "$VIDEO_MAIN" ]; then
    log_fail "Video service main file not found: $VIDEO_MAIN"
    ALL_PASSED=false
else
    # Check for mock provider rejection in production
    if grep -q 'log.Fatal("Mock push provider not allowed in production")' "$VIDEO_MAIN"; then
        log_pass "Mock provider rejection in production present"
    else
        log_fail "Mock provider rejection NOT found"
        log_fail "Fix: Add fail-fast when PUSH_PROVIDER=mock in production"
        ALL_PASSED=false
    fi

    # Check for StartupCheck call
    if grep -q 'push.StartupCheck(fbProvider)' "$VIDEO_MAIN"; then
        log_pass "StartupCheck function called after provider creation"
    else
        log_fail "StartupCheck call NOT found"
        log_fail "Fix: Call push.StartupCheck() after Firebase provider creation"
        ALL_PASSED=false
    fi
fi

# =============================================================================
# CHECK 6: Container Runtime Verification
# =============================================================================
log_section "CHECK 6: Container Runtime Verification"

# Check if video-service container is running
if docker ps | grep -q "video-service"; then
    log_pass "video-service container is running"
    
    # Check logs for Firebase initialization
    if docker logs video-service 2>&1 | grep -q "Firebase Admin SDK initialized successfully"; then
        log_pass "Firebase Admin SDK initialized successfully (found in logs)"
    else
        log_fail "Firebase Admin SDK initialization NOT found in logs"
        log_fail "Fix: Check container logs: docker logs video-service"
        ALL_PASSED=false
    fi
    
    # Check logs for startup check
    if docker logs video-service 2>&1 | grep -q "Firebase startup check passed"; then
        log_pass "Firebase startup check passed (found in logs)"
    else
        log_fail "Firebase startup check NOT found in logs"
        log_fail "Fix: Ensure StartupCheck() is called in video-service"
        ALL_PASSED=false
    fi
    
    # Check for mock provider (should NOT exist in production)
    if docker logs video-service 2>&1 | grep -q "MockProvider"; then
        log_fail "FAIL: MockProvider detected in production logs!"
        log_fail "Fix: Ensure PUSH_PROVIDER=firebase and secrets are properly configured"
        ALL_PASSED=false
    else
        log_pass "No MockProvider detected in production logs"
    fi
    
    # Verify Firebase credentials path in logs
    if docker logs video-service 2>&1 | grep -q "credentials_path=/run/secrets/firebase_credentials"; then
        log_pass "Firebase credentials loaded from /run/secrets/firebase_credentials"
    else
        log_fail "Firebase credentials NOT loaded from /run/secrets/firebase_credentials"
        log_fail "Fix: Verify FIREBASE_CREDENTIALS_PATH environment variable"
        ALL_PASSED=false
    fi
    
else
    log_fail "video-service container is NOT running"
    log_fail "Fix: Start service: docker-compose -f docker-compose.production.yml up -d video-service"
    ALL_PASSED=false
fi

# =============================================================================
# CHECK 7: Push Notification Code Paths Active
# =============================================================================
log_section "CHECK 7: Push Notification Code Paths"

# Check if FirebaseProvider is used in video service
if grep -q "push.NewFirebaseProvider" "$VIDEO_MAIN"; then
    log_pass "FirebaseProvider instantiated in video-service"
else
    log_fail "FirebaseProvider NOT instantiated in video-service"
    log_fail "Fix: Ensure push.NewFirebaseProvider() is called when PUSH_PROVIDER=firebase"
    ALL_PASSED=false
fi

# Check if push service is initialized with Firebase provider
if grep -q "push.NewService(pushProvider" "$VIDEO_MAIN"; then
    log_pass "Push service initialized with provider"
else
    log_fail "Push service NOT initialized with provider"
    log_fail "Fix: Ensure push.NewService() is called with pushProvider"
    ALL_PASSED=false
fi

# =============================================================================
# FINAL RESULT
# =============================================================================
log_section "VERIFICATION RESULT"

if [ "$ALL_PASSED" = true ]; then
    echo -e "${GREEN}✓✓✓ ALL CHECKS PASSED ✓✓✓${NC}"
    echo ""
    echo "Firebase is running in REAL production mode with:"
    echo "  - Docker secrets properly configured"
    echo "  - Credentials loaded from /run/secrets/"
    echo "  - No mock provider fallback"
    echo "  - Fail-fast behavior enabled"
    echo "  - No secrets in repository"
    exit 0
else
    echo -e "${RED}✗✗✗ VERIFICATION FAILED ✗✗✗${NC}"
    echo ""
    echo "FAIL REASONS:"
    echo "  - One or more checks failed (see details above)"
    echo "  - Review the [FAIL] sections for fix instructions"
    echo ""
    echo "CRITICAL ISSUES:"
    if [ "$SECRETS_FOUND" = true ]; then
        echo "  - SECRET FILES FOUND IN REPOSITORY (REMOVE IMMEDIATELY)"
    fi
    if docker ps | grep -q "video-service" && docker logs video-service 2>&1 | grep -q "MockProvider"; then
        echo "  - MOCK PROVIDER RUNNING IN PRODUCTION"
    fi
    exit 1
fi
