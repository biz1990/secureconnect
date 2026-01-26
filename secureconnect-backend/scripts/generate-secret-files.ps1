# =============================================================================
# SecureConnect - Generate Secret Files for Docker Compose (PowerShell)
# =============================================================================
# Creates all required secret files for production deployment
# Usage: .\scripts\generate-secret-files.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

Write-Host "=== SecureConnect Production Secret Files Generation ===" -ForegroundColor Cyan
Write-Host ""

# Create secrets directory
$secretsDir = ".\secrets"
if (-not (Test-Path $secretsDir)) {
    New-Item -ItemType Directory -Path $secretsDir | Out-Null
    Write-Host "[OK] Created secrets directory" -ForegroundColor Green
}
else {
    Write-Host "[OK] Secrets directory already exists" -ForegroundColor Yellow
}

# Function to generate random secret
function Get-RandomSecret {
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create()
    $rng.GetBytes($bytes)
    $base64 = [Convert]::ToBase64String($bytes)
    $clean = $base64 -replace '[+=/]', ''
    return $clean.Substring(0, [Math]::Min(32, $clean.Length))
}

# Function to create secret file
function New-SecretFile {
    param(
        [string]$Name,
        [string]$Value
    )
    
    $secretFile = Join-Path $secretsDir "$Name.txt"
    
    if (Test-Path $secretFile) {
        Write-Host "[OK] Secret file '$Name' already exists (skipping)" -ForegroundColor Yellow
        return
    }
    
    $Value | Out-File -FilePath $secretFile -Encoding UTF8 -NoNewline
    Write-Host "[OK] Created secret file: $Name" -ForegroundColor Green
}

# Function to create secret file from user input
function New-SecretFileFromInput {
    param(
        [string]$Name,
        [string]$Prompt,
        [bool]$IsPassword = $false
    )
    
    $secretFile = Join-Path $secretsDir "$Name.txt"
    
    if (Test-Path $secretFile) {
        Write-Host "[OK] Secret file '$Name' already exists (skipping)" -ForegroundColor Yellow
        return
    }
    
    Write-Host "Creating secret file: $Name" -ForegroundColor Cyan
    Write-Host $Prompt
    
    if ($IsPassword) {
        $value1 = Read-Host "Enter value" -AsSecureString
        $value2 = Read-Host "Confirm value" -AsSecureString
        
        $bstr1 = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($value1)
        $bstr2 = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($value2)
        $plain1 = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr1)
        $plain2 = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr2)
        [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr1)
        [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr2)
        
        if ($plain1 -ne $plain2) {
            Write-Host "[ERROR] Values don't match. Skipping $Name" -ForegroundColor Red
            return
        }
        
        New-SecretFile -Name $Name -Value $plain1
    }
    else {
        $value = Read-Host "Enter value"
        New-SecretFile -Name $Name -Value $value
    }
    
    Write-Host ""
}

# =============================================================================
# Generate all secrets
# =============================================================================

Write-Host ""
Write-Host "--- Critical Secrets ---" -ForegroundColor Cyan

# JWT Secret
Write-Host ""
Write-Host "JWT_SECRET: Used for signing authentication tokens" -ForegroundColor Yellow
$autoJwt = Read-Host "Auto-generate JWT secret? (Y/n)"
if ($autoJwt -ne "n" -and $autoJwt -ne "N") {
    $jwtSecret = Get-RandomSecret
    New-SecretFile -Name "jwt_secret" -Value $jwtSecret
}
else {
    New-SecretFileFromInput -Name "jwt_secret" -Prompt "Enter JWT secret (min 32 characters):" -IsPassword $true
}

# Database Password (CockroachDB)
Write-Host ""
Write-Host "DB_PASSWORD: CockroachDB root password" -ForegroundColor Yellow
$autoDb = Read-Host "Auto-generate DB password? (Y/n)"
if ($autoDb -ne "n" -and $autoDb -ne "N") {
    $dbPassword = Get-RandomSecret
    New-SecretFile -Name "db_password" -Value $dbPassword
}
else {
    New-SecretFileFromInput -Name "db_password" -Prompt "Enter database password:" -IsPassword $true
}

# Cassandra Credentials
Write-Host ""
Write-Host "CASSANDRA_USER: Cassandra username" -ForegroundColor Yellow
$useDefaultCassandraUser = Read-Host "Use default username 'cassandra'? (Y/n)"
if ($useDefaultCassandraUser -ne "n" -and $useDefaultCassandraUser -ne "N") {
    New-SecretFile -Name "cassandra_user" -Value "cassandra"
}
else {
    New-SecretFileFromInput -Name "cassandra_user" -Prompt "Enter Cassandra username:"
}

Write-Host ""
Write-Host "CASSANDRA_PASSWORD: Cassandra password" -ForegroundColor Yellow
$autoCassandra = Read-Host "Auto-generate Cassandra password? (Y/n)"
if ($autoCassandra -ne "n" -and $autoCassandra -ne "N") {
    $cassandraPassword = Get-RandomSecret
    New-SecretFile -Name "cassandra_password" -Value $cassandraPassword
}
else {
    New-SecretFileFromInput -Name "cassandra_password" -Prompt "Enter Cassandra password:" -IsPassword $true
}

# Redis Password
Write-Host ""
Write-Host "REDIS_PASSWORD: Redis authentication password" -ForegroundColor Yellow
$autoRedis = Read-Host "Auto-generate Redis password? (Y/n)"
if ($autoRedis -ne "n" -and $autoRedis -ne "N") {
    $redisPassword = Get-RandomSecret
    New-SecretFile -Name "redis_password" -Value $redisPassword
}
else {
    New-SecretFileFromInput -Name "redis_password" -Prompt "Enter Redis password:" -IsPassword $true
}

Write-Host ""
Write-Host "--- MinIO Credentials ---" -ForegroundColor Cyan

# MinIO Access Key
Write-Host ""
Write-Host "MINIO_ACCESS_KEY: MinIO access key (username)" -ForegroundColor Yellow
$autoMinioAccess = Read-Host "Auto-generate MinIO access key? (Y/n)"
if ($autoMinioAccess -ne "n" -and $autoMinioAccess -ne "N") {
    $minioAccessKey = (Get-RandomSecret).Substring(0, 20)
    New-SecretFile -Name "minio_access_key" -Value $minioAccessKey
}
else {
    New-SecretFileFromInput -Name "minio_access_key" -Prompt "Enter MinIO access key (20 chars):"
}

# MinIO Secret Key
Write-Host ""
Write-Host "MINIO_SECRET_KEY: MinIO secret key" -ForegroundColor Yellow
$autoMinioSecret = Read-Host "Auto-generate MinIO secret key? (Y/n)"
if ($autoMinioSecret -ne "n" -and $autoMinioSecret -ne "N") {
    $minioSecretKey = Get-RandomSecret
    New-SecretFile -Name "minio_secret_key" -Value $minioSecretKey
}
else {
    New-SecretFileFromInput -Name "minio_secret_key" -Prompt "Enter MinIO secret key:" -IsPassword $true
}

Write-Host ""
Write-Host "--- SMTP Credentials ---" -ForegroundColor Cyan

# SMTP Username
Write-Host ""
Write-Host "SMTP_USERNAME: SMTP username (email address)" -ForegroundColor Yellow
New-SecretFileFromInput -Name "smtp_username" -Prompt "Enter SMTP username:"

# SMTP Password
Write-Host ""
Write-Host "SMTP_PASSWORD: SMTP password" -ForegroundColor Yellow
New-SecretFileFromInput -Name "smtp_password" -Prompt "Enter SMTP password:" -IsPassword $true

Write-Host ""
Write-Host "--- Firebase Credentials ---" -ForegroundColor Cyan

# Firebase Project ID
Write-Host ""
Write-Host "FIREBASE_PROJECT_ID: Firebase project ID" -ForegroundColor Yellow
New-SecretFileFromInput -Name "firebase_project_id" -Prompt "Enter Firebase project ID:"

# Firebase Credentials File
Write-Host ""
Write-Host "FIREBASE_CREDENTIALS: Firebase service account JSON file" -ForegroundColor Yellow
Write-Host "Please provide the path to your Firebase service account JSON file"
Write-Host "Example: .\firebase-service-account.json"
$firebaseFile = Read-Host "Enter file path"

if (-not [string]::IsNullOrWhiteSpace($firebaseFile) -and (Test-Path $firebaseFile)) {
    $destFile = Join-Path $secretsDir "firebase_credentials.json"
    if (-not (Test-Path $destFile)) {
        Copy-Item -Path $firebaseFile -Destination $destFile
        Write-Host "[OK] Created secret file: firebase_credentials.json" -ForegroundColor Green
    }
    else {
        Write-Host "[OK] Secret file 'firebase_credentials.json' already exists (skipping)" -ForegroundColor Yellow
    }
}
else {
    Write-Host "[ERROR] Invalid file path. Skipping firebase_credentials" -ForegroundColor Red
    Write-Host "[WARNING] You can create this file later by copying your Firebase JSON to:" -ForegroundColor Yellow
    Write-Host "    .\secrets\firebase_credentials.json"
}

Write-Host ""
Write-Host "--- TURN Server Credentials ---" -ForegroundColor Cyan

# TURN User
Write-Host ""
Write-Host "TURN_USER: TURN server username" -ForegroundColor Yellow
$useDefaultTurnUser = Read-Host "Use default username 'turnuser'? (Y/n)"
if ($useDefaultTurnUser -ne "n" -and $useDefaultTurnUser -ne "N") {
    New-SecretFile -Name "turn_user" -Value "turnuser"
}
else {
    New-SecretFileFromInput -Name "turn_user" -Prompt "Enter TURN server username:"
}

# TURN Password
Write-Host ""
Write-Host "TURN_PASSWORD: TURN server password" -ForegroundColor Yellow
$autoTurn = Read-Host "Auto-generate TURN password? (Y/n)"
if ($autoTurn -ne "n" -and $autoTurn -ne "N") {
    $turnPassword = Get-RandomSecret
    New-SecretFile -Name "turn_password" -Value $turnPassword
}
else {
    New-SecretFileFromInput -Name "turn_password" -Prompt "Enter TURN server password:" -IsPassword $true
}

# =============================================================================
# Summary
# =============================================================================

Write-Host ""
Write-Host "=== Secret Files Generation Complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "Created secret files in: $secretsDir\"
Get-ChildItem $secretsDir | ForEach-Object { Write-Host "  $($_.Name)" }
Write-Host ""
Write-Host "[WARNING] IMPORTANT SECURITY REMINDERS:" -ForegroundColor Yellow
Write-Host "1. Add '$secretsDir\' to .gitignore"
Write-Host "2. Never commit these files to version control"
Write-Host "3. Store backup copies in a secure password manager"
Write-Host "4. Set appropriate file permissions"
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "1. Run: .\scripts\generate-certs.ps1 (generate CockroachDB TLS certs)"
Write-Host "2. Run: docker compose -f docker-compose.production.yml up -d --build"
Write-Host ""
