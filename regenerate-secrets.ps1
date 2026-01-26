###############################################################################
# SECURECONNECT SECRETS REGENERATION SCRIPT (PowerShell)
#
# This script generates new secure secrets for all services.
# Run this AFTER cleaning git history.
#
# Usage: .\regenerate-secrets.ps1
###############################################################################

$ErrorActionPreference = "Stop"

# Colors
function Write-ColorOutput {
    param(
        [string]$ForegroundColor,
        [string]$Message
    )
    Write-Host $Message -ForegroundColor $ForegroundColor
}

Write-ColorOutput "Yellow" "========================================"
Write-ColorOutput "Yellow" "SECURECONNECT SECRETS REGENERATION"
Write-ColorOutput "Yellow" "========================================"
Write-Host ""

# Navigate to secrets directory
$SECRETS_DIR = Join-Path $PSScriptRoot "secureconnect-backend\secrets"
if (-not (Test-Path $SECRETS_DIR)) {
    Write-ColorOutput "Red" "ERROR: Secrets directory not found: $SECRETS_DIR"
    exit 1
}

Write-ColorOutput "Green" "Secrets directory: $SECRETS_DIR"
Write-Host ""

# Function to generate random hex string
function New-RandomHex {
    param([int]$Length)
    $bytes = New-Object byte[] $Length
    $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create()
    $rng.GetBytes($bytes)
    return -join ($bytes | ForEach-Object { "{0:x2}" -f $_ })
}

# Generate new JWT secret (32 bytes = 64 hex characters)
Write-ColorOutput "Yellow" "[1/13] Generating JWT secret..."
$jwtSecret = New-RandomHex 32
$jwtSecret | Out-File -FilePath (Join-Path $SECRETS_DIR "jwt_secret.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: jwt_secret.txt"
Write-Host ""

# Generate new database password (24 bytes = 48 hex characters)
Write-ColorOutput "Yellow" "[2/13] Generating database password..."
$dbPassword = New-RandomHex 24
$dbPassword | Out-File -FilePath (Join-Path $SECRETS_DIR "db_password.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: db_password.txt"
Write-Host ""

# Generate new Cassandra username (16 bytes = 32 hex characters)
Write-ColorOutput "Yellow" "[3/13] Generating Cassandra username..."
$cassandraUser = New-RandomHex 16
$cassandraUser | Out-File -FilePath (Join-Path $SECRETS_DIR "cassandra_user.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: cassandra_user.txt"
Write-Host ""

# Generate new Cassandra password (24 bytes = 48 hex characters)
Write-ColorOutput "Yellow" "[4/13] Generating Cassandra password..."
$cassandraPassword = New-RandomHex 24
$cassandraPassword | Out-File -FilePath (Join-Path $SECRETS_DIR "cassandra_password.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: cassandra_password.txt"
Write-Host ""

# Generate new Redis password (24 bytes = 48 hex characters)
Write-ColorOutput "Yellow" "[5/13] Generating Redis password..."
$redisPassword = New-RandomHex 24
$redisPassword | Out-File -FilePath (Join-Path $SECRETS_DIR "redis_password.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: redis_password.txt"
Write-Host ""

# Generate new MinIO access key (20 bytes = 40 hex characters)
Write-ColorOutput "Yellow" "[6/13] Generating MinIO access key..."
$minioAccessKey = New-RandomHex 20
$minioAccessKey | Out-File -FilePath (Join-Path $SECRETS_DIR "minio_access_key.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: minio_access_key.txt"
Write-Host ""

# Generate new MinIO secret key (40 bytes = 80 hex characters)
Write-ColorOutput "Yellow" "[7/13] Generating MinIO secret key..."
$minioSecretKey = New-RandomHex 40
$minioSecretKey | Out-File -FilePath (Join-Path $SECRETS_DIR "minio_secret_key.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: minio_secret_key.txt"
Write-Host ""

# Generate new TURN username (16 bytes = 32 hex characters)
Write-ColorOutput "Yellow" "[8/13] Generating TURN username..."
$turnUser = New-RandomHex 16
$turnUser | Out-File -FilePath (Join-Path $SECRETS_DIR "turn_user.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: turn_user.txt"
Write-Host ""

# Generate new TURN password (24 bytes = 48 hex characters)
Write-ColorOutput "Yellow" "[9/13] Generating TURN password..."
$turnPassword = New-RandomHex 24
$turnPassword | Out-File -FilePath (Join-Path $SECRETS_DIR "turn_password.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: turn_password.txt"
Write-Host ""

# Generate new Grafana password (24 bytes = 48 hex characters)
Write-ColorOutput "Yellow" "[10/13] Generating Grafana password..."
$grafanaPassword = New-RandomHex 24
$grafanaPassword | Out-File -FilePath (Join-Path $SECRETS_DIR "grafana_admin_password.txt") -Encoding utf8
Write-ColorOutput "Green" "Generated: grafana_admin_password.txt"
Write-Host ""

Write-ColorOutput "Green" "========================================"
Write-ColorOutput "Green" "REGENERATION COMPLETED"
Write-ColorOutput "Green" "========================================"
Write-Host ""

Write-ColorOutput "Yellow" "MANUAL STEPS REQUIRED:"
Write-Host ""
Write-Host "1. SMTP Credentials:"
Write-Host "   - Go to your email provider (SendGrid, Mailgun, AWS SES, etc.)"
Write-Host "   - Generate new SMTP username and password"
Write-Host "   - Update: secureconnect-backend\secrets\smtp_username.txt"
Write-Host "   - Update: secureconnect-backend\secrets\smtp_password.txt"
Write-Host ""
Write-Host "2. Firebase Credentials:"
Write-Host "   - Go to Firebase Console: https://console.firebase.google.com/"
Write-Host "   - Select your project or create a new one"
Write-Host "   - Go to Project Settings > Service Accounts"
Write-Host "   - Click 'Generate new private key'"
Write-Host "   - Download the JSON file"
Write-Host "   - Save as: secureconnect-backend\secrets\firebase_credentials.json"
Write-Host "   - Extract project ID and save as: secureconnect-backend\secrets\firebase_project_id.txt"
Write-Host ""

Write-ColorOutput "Yellow" "NEXT STEPS:"
Write-Host ""
Write-Host "1. Restart Docker services with new secrets:"
Write-Host "   cd secureconnect-backend"
Write-Host "   docker-compose -f docker-compose.production.yml down"
Write-Host "   docker-compose -f docker-compose.production.yml up -d"
Write-Host ""
Write-Host "2. Verify services start correctly:"
Write-Host "   docker-compose -f docker-compose.production.yml ps"
Write-Host ""
Write-Host "3. Check logs for any secret-related errors:"
Write-Host "   docker-compose -f docker-compose.production.yml logs"
Write-Host ""
