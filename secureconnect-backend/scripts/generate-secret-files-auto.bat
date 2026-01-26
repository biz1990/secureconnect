@echo off
REM =============================================================================
REM SecureConnect - Auto-Generate Secret Files (Non-Interactive)
REM =============================================================================
REM Creates all required secret files with auto-generated values
REM Usage: scripts\generate-secret-files-auto.bat
REM =============================================================================

setlocal enabledelayedexpansion

echo === SecureConnect Production Secret Files Generation (Auto) ===
echo.

REM Create secrets directory
set SECRETS_DIR=.\secrets
if not exist "%SECRETS_DIR%" (
    mkdir "%SECRETS_DIR%"
    echo [OK] Created secrets directory
) else (
    echo [OK] Secrets directory already exists
)

REM Function to generate random secret (PowerShell based)
for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set RANDOM_SECRET=%%i

REM Generate all secrets
echo.
echo Generating secrets...

REM JWT Secret
if not exist "%SECRETS_DIR%\jwt_secret.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set JWT_SECRET=%%i
    echo !JWT_SECRET! > "%SECRETS_DIR%\jwt_secret.txt"
    echo [OK] Created: jwt_secret.txt
) else (
    echo [SKIP] jwt_secret.txt already exists
)

REM Database Password
if not exist "%SECRETS_DIR%\db_password.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set DB_PASSWORD=%%i
    echo !DB_PASSWORD! > "%SECRETS_DIR%\db_password.txt"
    echo [OK] Created: db_password.txt
) else (
    echo [SKIP] db_password.txt already exists
)

REM Cassandra User
if not exist "%SECRETS_DIR%\cassandra_user.txt" (
    echo cassandra > "%SECRETS_DIR%\cassandra_user.txt"
    echo [OK] Created: cassandra_user.txt
) else (
    echo [SKIP] cassandra_user.txt already exists
)

REM Cassandra Password
if not exist "%SECRETS_DIR%\cassandra_password.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set CASSANDRA_PASSWORD=%%i
    echo !CASSANDRA_PASSWORD! > "%SECRETS_DIR%\cassandra_password.txt"
    echo [OK] Created: cassandra_password.txt
) else (
    echo [SKIP] cassandra_password.txt already exists
)

REM Redis Password
if not exist "%SECRETS_DIR%\redis_password.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set REDIS_PASSWORD=%%i
    echo !REDIS_PASSWORD! > "%SECRETS_DIR%\redis_password.txt"
    echo [OK] Created: redis_password.txt
) else (
    echo [SKIP] redis_password.txt already exists
)

REM MinIO Access Key
if not exist "%SECRETS_DIR%\minio_access_key.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(20, $clean.Length))"') do set MINIO_ACCESS_KEY=%%i
    echo !MINIO_ACCESS_KEY! > "%SECRETS_DIR%\minio_access_key.txt"
    echo [OK] Created: minio_access_key.txt
) else (
    echo [SKIP] minio_access_key.txt already exists
)

REM MinIO Secret Key
if not exist "%SECRETS_DIR%\minio_secret_key.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set MINIO_SECRET_KEY=%%i
    echo !MINIO_SECRET_KEY! > "%SECRETS_DIR%\minio_secret_key.txt"
    echo [OK] Created: minio_secret_key.txt
) else (
    echo [SKIP] minio_secret_key.txt already exists
)

REM SMTP Username (placeholder)
if not exist "%SECRETS_DIR%\smtp_username.txt" (
    echo noreply@example.com > "%SECRETS_DIR%\smtp_username.txt"
    echo [OK] Created: smtp_username.txt (placeholder)
) else (
    echo [SKIP] smtp_username.txt already exists
)

REM SMTP Password (placeholder)
if not exist "%SECRETS_DIR%\smtp_password.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set SMTP_PASSWORD=%%i
    echo !SMTP_PASSWORD! > "%SECRETS_DIR%\smtp_password.txt"
    echo [OK] Created: smtp_password.txt (placeholder)
) else (
    echo [SKIP] smtp_password.txt already exists
)

REM Firebase Project ID (placeholder)
if not exist "%SECRETS_DIR%\firebase_project_id.txt" (
    echo secureconnect-dev > "%SECRETS_DIR%\firebase_project_id.txt"
    echo [OK] Created: firebase_project_id.txt (placeholder)
) else (
    echo [SKIP] firebase_project_id.txt already exists
)

REM Firebase Credentials (placeholder)
if not exist "%SECRETS_DIR%\firebase_credentials.json" (
    echo {"type":"service_account","project_id":"secureconnect-dev","private_key_id":"placeholder","private_key":"placeholder","client_email":"placeholder","client_id":"placeholder","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token"} > "%SECRETS_DIR%\firebase_credentials.json"
    echo [OK] Created: firebase_credentials.json (placeholder)
) else (
    echo [SKIP] firebase_credentials.json already exists
)

REM TURN User
if not exist "%SECRETS_DIR%\turn_user.txt" (
    echo turnuser > "%SECRETS_DIR%\turn_user.txt"
    echo [OK] Created: turn_user.txt
) else (
    echo [SKIP] turn_user.txt already exists
)

REM TURN Password
if not exist "%SECRETS_DIR%\turn_password.txt" (
    for /f "delims=" %%i in ('powershell -Command "$bytes = New-Object byte[] 32; $rng = [System.Security.Cryptography.RNGCryptoServiceProvider]::Create(); $rng.GetBytes($bytes); $base64 = [Convert]::ToBase64String($bytes); $clean = $base64 -replace '[+=/]', ''; $clean.Substring(0, [Math]::Min(32, $clean.Length))"') do set TURN_PASSWORD=%%i
    echo !TURN_PASSWORD! > "%SECRETS_DIR%\turn_password.txt"
    echo [OK] Created: turn_password.txt
) else (
    echo [SKIP] turn_password.txt already exists
)

REM =============================================================================
REM Summary
REM =============================================================================

echo.
echo === Secret Files Generation Complete ===
echo.
echo Created secret files in: %SECRETS_DIR%\
dir /b "%SECRETS_DIR%"
echo.
echo [WARNING] IMPORTANT SECURITY REMINDERS:
echo 1. Add '%SECRETS_DIR%\' to .gitignore (already done)
echo 2. Never commit these files to version control
echo 3. Store backup copies in a secure password manager
echo 4. Replace placeholder values with actual production values
echo.
echo Next steps:
echo 1. Update placeholder values in secrets files:
echo    - smtp_username.txt
echo    - smtp_password.txt
echo    - firebase_project_id.txt
echo    - firebase_credentials.json
echo 2. Run: docker compose -f docker-compose.production.yml up -d --build
echo.

endlocal
