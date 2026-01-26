@echo off
REM =============================================================================
REM SecureConnect - Generate Secret Files for Docker Compose (Windows)
REM =============================================================================
REM Creates all required secret files for production deployment
REM Usage: scripts\generate-secret-files.bat
REM =============================================================================

setlocal enabledelayedexpansion

echo === SecureConnect Production Secret Files Generation ===
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
:generate_random_secret
for /f "delims=" %%i in ('powershell -Command "[Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Minimum 48 -Maximum 122 }) -join '' | ForEach-Object { [char]$_ })"') do set RANDOM_SECRET=%%i
set RANDOM_SECRET=%RANDOM_SECRET:~0,32%
goto :EOF

REM Function to create secret file
:create_secret_file
set SECRET_NAME=%~1
set SECRET_VALUE=%~2
set SECRET_FILE=%SECRETS_DIR%\%SECRET_NAME%.txt

if exist "%SECRET_FILE%" (
    echo [OK] Secret file '%SECRET_NAME%' already exists ^(skipping^)
    goto :EOF
)

echo !SECRET_VALUE! > "%SECRET_FILE%"
echo [OK] Created secret file: %SECRET_NAME%
goto :EOF

REM =============================================================================
REM Generate all secrets
REM =============================================================================

echo.
echo --- Critical Secrets ---

REM JWT Secret
echo.
echo JWT_SECRET: Used for signing authentication tokens
set /p AUTO_JWT="Auto-generate JWT secret? (Y/n): "
if /i "%AUTO_JWT%"=="n" (
    set /p JWT_SECRET="Enter JWT secret (min 32 characters): "
    call :create_secret_file jwt_secret !JWT_SECRET!
) else (
    call :generate_random_secret
    call :create_secret_file jwt_secret !RANDOM_SECRET!
)

REM Database Password (CockroachDB)
echo.
echo DB_PASSWORD: CockroachDB root password
set /p AUTO_DB="Auto-generate DB password? (Y/n): "
if /i "%AUTO_DB%"=="n" (
    set /p DB_PASSWORD="Enter database password: "
    call :create_secret_file db_password !DB_PASSWORD!
) else (
    call :generate_random_secret
    call :create_secret_file db_password !RANDOM_SECRET!
)

REM Cassandra Credentials
echo.
echo CASSANDRA_USER: Cassandra username
set /p USE_DEFAULT_CASSANDRA_USER="Use default username 'cassandra'? (Y/n): "
if /i "%USE_DEFAULT_CASSANDRA_USER%"=="n" (
    set /p CASSANDRA_USER="Enter Cassandra username: "
    call :create_secret_file cassandra_user !CASSANDRA_USER!
) else (
    call :create_secret_file cassandra_user cassandra
)

echo.
echo CASSANDRA_PASSWORD: Cassandra password
set /p AUTO_CASSANDRA="Auto-generate Cassandra password? (Y/n): "
if /i "%AUTO_CASSANDRA%"=="n" (
    set /p CASSANDRA_PASSWORD="Enter Cassandra password: "
    call :create_secret_file cassandra_password !CASSANDRA_PASSWORD!
) else (
    call :generate_random_secret
    call :create_secret_file cassandra_password !RANDOM_SECRET!
)

REM Redis Password
echo.
echo REDIS_PASSWORD: Redis authentication password
set /p AUTO_REDIS="Auto-generate Redis password? (Y/n): "
if /i "%AUTO_REDIS%"=="n" (
    set /p REDIS_PASSWORD="Enter Redis password: "
    call :create_secret_file redis_password !REDIS_PASSWORD!
) else (
    call :generate_random_secret
    call :create_secret_file redis_password !RANDOM_SECRET!
)

echo.
echo --- MinIO Credentials ---

REM MinIO Access Key
echo.
echo MINIO_ACCESS_KEY: MinIO access key (username)
set /p AUTO_MINIO_ACCESS="Auto-generate MinIO access key? (Y/n): "
if /i "%AUTO_MINIO_ACCESS%"=="n" (
    set /p MINIO_ACCESS_KEY="Enter MinIO access key (20 chars): "
    call :create_secret_file minio_access_key !MINIO_ACCESS_KEY!
) else (
    call :generate_random_secret
    set MINIO_ACCESS_KEY=!RANDOM_SECRET:~0,20!
    call :create_secret_file minio_access_key !MINIO_ACCESS_KEY!
)

REM MinIO Secret Key
echo.
echo MINIO_SECRET_KEY: MinIO secret key
set /p AUTO_MINIO_SECRET="Auto-generate MinIO secret key? (Y/n): "
if /i "%AUTO_MINIO_SECRET%"=="n" (
    set /p MINIO_SECRET_KEY="Enter MinIO secret key: "
    call :create_secret_file minio_secret_key !MINIO_SECRET_KEY!
) else (
    call :generate_random_secret
    call :create_secret_file minio_secret_key !RANDOM_SECRET!
)

echo.
echo --- SMTP Credentials ---

REM SMTP Username
echo.
echo SMTP_USERNAME: SMTP username (email address)
set /p SMTP_USERNAME="Enter SMTP username: "
call :create_secret_file smtp_username !SMTP_USERNAME!

REM SMTP Password
echo.
echo SMTP_PASSWORD: SMTP password
set /p SMTP_PASSWORD="Enter SMTP password: "
call :create_secret_file smtp_password !SMTP_PASSWORD!

echo.
echo --- Firebase Credentials ---

REM Firebase Project ID
echo.
echo FIREBASE_PROJECT_ID: Firebase project ID
set /p FIREBASE_PROJECT_ID="Enter Firebase project ID: "
call :create_secret_file firebase_project_id !FIREBASE_PROJECT_ID!

REM Firebase Credentials File
echo.
echo FIREBASE_CREDENTIALS: Firebase service account JSON file
echo Please provide the path to your Firebase service account JSON file
echo Example: .\firebase-service-account.json
set /p FIREBASE_FILE="Enter file path: "

if exist "%FIREBASE_FILE%" (
    if not exist "%SECRETS_DIR%\firebase_credentials.json" (
        copy "%FIREBASE_FILE%" "%SECRETS_DIR%\firebase_credentials.json" >nul
        echo [OK] Created secret file: firebase_credentials.json
    ) else (
        echo [OK] Secret file 'firebase_credentials.json' already exists ^(skipping^)
    )
) else (
    echo [ERROR] Invalid file path. Skipping firebase_credentials
    echo [WARNING] You can create this file later by copying your Firebase JSON to:
    echo     .\secrets\firebase_credentials.json
)

echo.
echo --- TURN Server Credentials ---

REM TURN User
echo.
echo TURN_USER: TURN server username
set /p USE_DEFAULT_TURN_USER="Use default username 'turnuser'? (Y/n): "
if /i "%USE_DEFAULT_TURN_USER%"=="n" (
    set /p TURN_USER="Enter TURN server username: "
    call :create_secret_file turn_user !TURN_USER!
) else (
    call :create_secret_file turn_user turnuser
)

REM TURN Password
echo.
echo TURN_PASSWORD: TURN server password
set /p AUTO_TURN="Auto-generate TURN password? (Y/n): "
if /i "%AUTO_TURN%"=="n" (
    set /p TURN_PASSWORD="Enter TURN server password: "
    call :create_secret_file turn_password !TURN_PASSWORD!
) else (
    call :generate_random_secret
    call :create_secret_file turn_password !RANDOM_SECRET!
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
echo 1. Add '%SECRETS_DIR%\' to .gitignore
echo 2. Never commit these files to version control
echo 3. Store backup copies in a secure password manager
echo 4. Set appropriate file permissions
echo.
echo Next steps:
echo 1. Run: .\scripts\generate-certs.bat (generate CockroachDB TLS certs)
echo 2. Run: docker compose -f docker-compose.production.yml up -d --build
echo.

endlocal
