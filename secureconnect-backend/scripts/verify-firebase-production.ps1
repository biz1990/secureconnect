# verify-firebase-production.ps1
# Production SRE Security Audit: Firebase Production Mode Verification
# This script verifies Firebase is running in REAL production mode
# with proper Docker secrets configuration.

# Error handling
$ErrorActionPreference = "Stop"

# Track overall status
$AllPassed = $true

# Helper functions
function Log-Info {
    Write-Host "[INFO] $args" -ForegroundColor Yellow
}

function Log-Pass {
    Write-Host "[PASS] $args" -ForegroundColor Green
}

function Log-Fail {
    Write-Host "[FAIL] $args" -ForegroundColor Red
    $script:AllPassed = $false
}

function Log-Section {
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host $args -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
}

# =============================================================================
# CHECK 1: No Secret Files in Repository
# =============================================================================
Log-Section "CHECK 1: Repository Security - No Secret Files"

$SecretFiles = @(
    "firebase-service-account.json",
    "firebase_credentials.json",
    "service-account.json",
    "*.p12",
    "*.key"
)

$SecretsFound = $false
foreach ($pattern in $SecretFiles) {
    $files = Get-ChildItem -Path . -Filter $pattern -Recurse -ErrorAction SilentlyContinue
    if ($files) {
        Log-Fail "Found secret file matching pattern: $pattern"
        $files | ForEach-Object { Write-Host "  - $($_.FullName)" -ForegroundColor Red }
        $SecretsFound = $true
    }
}

if (-not $SecretsFound) {
    Log-Pass "No Firebase secret files found in repository"
}
else {
    Log-Fail "FAIL: Secret files found in repository. Remove them immediately!"
    Log-Fail "File path to fix: Remove files matching patterns above"
}

# =============================================================================
# CHECK 2: Docker Secrets Exist
# =============================================================================
Log-Section "CHECK 2: Docker Secrets Configuration"

$Secrets = docker secret ls
if ($Secrets -notmatch "firebase_project_id") {
    Log-Fail "Docker secret 'firebase_project_id' not found"
    Log-Fail "Fix: echo 'your-project-id' | docker secret create firebase_project_id -"
    $script:AllPassed = $false
}
else {
    Log-Pass "Docker secret 'firebase_project_id' exists"
}

if ($Secrets -notmatch "firebase_credentials") {
    Log-Fail "Docker secret 'firebase_credentials' not found"
    Log-Fail "Fix: cat firebase-service-account.json | docker secret create firebase_credentials -"
    $script:AllPassed = $false
}
else {
    Log-Pass "Docker secret 'firebase_credentials' exists"
}

# =============================================================================
# CHECK 3: Docker Compose Configuration
# =============================================================================
Log-Section "CHECK 3: Docker Compose Environment Variables"

$ComposeFile = "docker-compose.production.yml"

if (-not (Test-Path $ComposeFile)) {
    Log-Fail "Docker compose file not found: $ComposeFile"
    $script:AllPassed = $false
}
else {
    $Content = Get-Content $ComposeFile -Raw
    
    # Check for PUSH_PROVIDER=firebase
    if ($Content -match "PUSH_PROVIDER=firebase") {
        Log-Pass "PUSH_PROVIDER=firebase configured in docker-compose.production.yml"
    }
    else {
        Log-Fail "PUSH_PROVIDER=firebase NOT found in docker-compose.production.yml"
        Log-Fail "Fix: Add '- PUSH_PROVIDER=firebase' to video-service environment"
        $script:AllPassed = $false
    }

    # Check for FIREBASE_CREDENTIALS_PATH
    if ($Content -match "FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials") {
        Log-Pass "FIREBASE_CREDENTIALS_PATH points to /run/secrets/firebase_credentials"
    }
    else {
        Log-Fail "FIREBASE_CREDENTIALS_PATH not pointing to /run/secrets/firebase_credentials"
        Log-Fail "Fix: Add '- FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials' to video-service"
        $script:AllPassed = $false
    }

    # Check for FIREBASE_PROJECT_ID_FILE
    if ($Content -match "FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id") {
        Log-Pass "FIREBASE_PROJECT_ID_FILE points to /run/secrets/firebase_project_id"
    }
    else {
        Log-Fail "FIREBASE_PROJECT_ID_FILE not pointing to /run/secrets/firebase_project_id"
        Log-Fail "Fix: Add '- FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id' to video-service"
        $script:AllPassed = $false
    }
}

# =============================================================================
# CHECK 4: Go Code - Fail-Fast Logic
# =============================================================================
Log-Section "CHECK 4: Go Code - Production Fail-Fast Logic"

$FirebaseGo = "pkg/push/firebase.go"

if (-not (Test-Path $FirebaseGo)) {
    Log-Fail "Firebase provider file not found: $FirebaseGo"
    $script:AllPassed = $false
}
else {
    $Content = Get-Content $FirebaseGo -Raw
    
    # Check for production mode detection
    if ($Content -match 'productionMode := os.Getenv\("ENV"\) == "production"') {
        Log-Pass "Production mode detection present in pkg/push/firebase.go"
    }
    else {
        Log-Fail "Production mode detection NOT found in pkg/push/firebase.go"
        Log-Fail "Fix: Add 'productionMode := os.Getenv(\"ENV\") == \"production\"'"
        $script:AllPassed = $false
    }

    # Check for fail-fast on missing credentials (flexible pattern matching)
    if ($Content -match 'Firebase credentials required in production mode') {
        Log-Pass "Fail-fast for missing credentials present"
    }
    else {
        Log-Fail "Fail-fast for missing credentials NOT found"
        Log-Fail "Fix: Add log.Fatal() when credentials missing in production"
        $script:AllPassed = $false
    }

    # Check for fail-fast on initialization failure (flexible pattern matching)
    if ($Content -match 'Firebase initialization failed in production mode') {
        Log-Pass "Fail-fast for Firebase init failure present"
    }
    else {
        Log-Fail "Fail-fast for Firebase init failure NOT found"
        Log-Fail "Fix: Add log.Fatal() when Firebase init fails in production"
        $script:AllPassed = $false
    }
}

# =============================================================================
# CHECK 5: Video Service - Mock Provider Rejection
# =============================================================================
Log-Section "CHECK 5: Video Service - Mock Provider Rejection"

$VideoMain = "cmd/video-service/main.go"

if (-not (Test-Path $VideoMain)) {
    Log-Fail "Video service main file not found: $VideoMain"
    $script:AllPassed = $false
}
else {
    $Content = Get-Content $VideoMain -Raw
    
    # Check for mock provider rejection in production (flexible pattern matching)
    if ($Content -match 'Mock push provider not allowed in production') {
        Log-Pass "Mock provider rejection in production present"
    }
    else {
        Log-Fail "Mock provider rejection NOT found"
        Log-Fail "Fix: Add fail-fast when PUSH_PROVIDER=mock in production"
        $script:AllPassed = $false
    }

    # Check for StartupCheck call
    if ($Content -match 'push\.StartupCheck\(fbProvider\)') {
        Log-Pass "StartupCheck function called after provider creation"
    }
    else {
        Log-Fail "StartupCheck call NOT found"
        Log-Fail "Fix: Call push.StartupCheck() after Firebase provider creation"
        $script:AllPassed = $false
    }
}

# =============================================================================
# CHECK 6: Container Runtime Verification
# =============================================================================
Log-Section "CHECK 6: Container Runtime Verification"

# Check if video-service container is running
$Container = docker ps --filter "name=video-service" --format "{{.Names}}"
if ($Container) {
    Log-Pass "video-service container is running"
    
    # Check logs for Firebase initialization
    $Logs = docker logs video-service 2>&1
    if ($Logs -match "Firebase Admin SDK initialized successfully") {
        Log-Pass "Firebase Admin SDK initialized successfully (found in logs)"
    }
    else {
        Log-Fail "Firebase Admin SDK initialization NOT found in logs"
        Log-Fail "Fix: Check container logs: docker logs video-service"
        $script:AllPassed = $false
    }
    
    # Check logs for startup check
    if ($Logs -match "Firebase startup check passed") {
        Log-Pass "Firebase startup check passed (found in logs)"
    }
    else {
        Log-Fail "Firebase startup check NOT found in logs"
        Log-Fail "Fix: Ensure StartupCheck() is called in video-service"
        $script:AllPassed = $false
    }
    
    # Check for mock provider (should NOT exist in production)
    if ($Logs -match "MockProvider") {
        Log-Fail "FAIL: MockProvider detected in production logs!"
        Log-Fail "Fix: Ensure PUSH_PROVIDER=firebase and secrets are properly configured"
        $script:AllPassed = $false
    }
    else {
        Log-Pass "No MockProvider detected in production logs"
    }
    
    # Verify Firebase credentials path in logs
    if ($Logs -match "credentials_path=/run/secrets/firebase_credentials") {
        Log-Pass "Firebase credentials loaded from /run/secrets/firebase_credentials"
    }
    else {
        Log-Fail "Firebase credentials NOT loaded from /run/secrets/firebase_credentials"
        Log-Fail "Fix: Verify FIREBASE_CREDENTIALS_PATH environment variable"
        $script:AllPassed = $false
    }
    
}
else {
    Log-Fail "video-service container is NOT running"
    Log-Fail "Fix: Start service: docker-compose -f docker-compose.production.yml up -d video-service"
    $script:AllPassed = $false
}

# =============================================================================
# CHECK 7: Push Notification Code Paths Active
# =============================================================================
Log-Section "CHECK 7: Push Notification Code Paths"

$Content = Get-Content $VideoMain -Raw

# Check if FirebaseProvider is used in video service
if ($Content -match "push\.NewFirebaseProvider") {
    Log-Pass "FirebaseProvider instantiated in video-service"
}
else {
    Log-Fail "FirebaseProvider NOT instantiated in video-service"
    Log-Fail "Fix: Ensure push.NewFirebaseProvider() is called when PUSH_PROVIDER=firebase"
    $script:AllPassed = $false
}

# Check if push service is initialized with Firebase provider
if ($Content -match "push\.NewService\(pushProvider") {
    Log-Pass "Push service initialized with provider"
}
else {
    Log-Fail "Push service NOT initialized with provider"
    Log-Fail "Fix: Ensure push.NewService() is called with pushProvider"
    $script:AllPassed = $false
}

# =============================================================================
# FINAL RESULT
# =============================================================================
Log-Section "VERIFICATION RESULT"

if ($AllPassed) {
    Write-Host "✓✓✓ ALL CHECKS PASSED ✓✓✓" -ForegroundColor Green
    Write-Host ""
    Write-Host "Firebase is running in REAL production mode with:" -ForegroundColor Green
    Write-Host "  - Docker secrets properly configured"
    Write-Host "  - Credentials loaded from /run/secrets/"
    Write-Host "  - No mock provider fallback"
    Write-Host "  - Fail-fast behavior enabled"
    Write-Host "  - No secrets in repository"
    exit 0
}
else {
    Write-Host "✗✗✗ VERIFICATION FAILED ✗✗✗" -ForegroundColor Red
    Write-Host ""
    Write-Host "FAIL REASONS:" -ForegroundColor Red
    Write-Host "  - One or more checks failed (see details above)"
    Write-Host "  - Review [FAIL] sections for fix instructions"
    Write-Host ""
    Write-Host "CRITICAL ISSUES:" -ForegroundColor Red
    if ($SecretsFound) {
        Write-Host "  - SECRET FILES FOUND IN REPOSITORY (REMOVE IMMEDIATELY)" -ForegroundColor Red
    }
    if ($Container -and ($Logs -match "MockProvider")) {
        Write-Host "  - MOCK PROVIDER RUNNING IN PRODUCTION" -ForegroundColor Red
    }
    exit 1
}
