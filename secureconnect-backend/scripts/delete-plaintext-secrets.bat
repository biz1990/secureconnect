@echo off
REM =============================================================================
REM DELETE PLAINTEXT FIREBASE CREDENTIALS - CRITICAL SECURITY STEP
REM =============================================================================
REM This script safely deletes all plaintext Firebase credential files
REM Execute this script AFTER rotating Firebase credentials
REM =============================================================================

echo.
echo ===============================================
echo  DELETING PLAINTEXT FIREBASE CREDENTIALS
echo ===============================================
echo.
echo WARNING: This will permanently delete the following files:
echo   - secrets\firebase.json
echo   - secrets\firebase1.json
echo   - secrets\chatapp-27370-firebase-adminsdk-fbsvc-d4681a8c2e.json
echo.
echo BEFORE CONTINUING:
echo   1. Have you rotated Firebase credentials? (Y/N)
echo   2. Have you saved new credentials outside the repo? (Y/N)
echo   3. Have you created Docker secret 'firebase_credentials'? (Y/N)
echo.

set /p confirm="Type 'DELETE' to confirm deletion: "

if NOT "%confirm%"=="DELETE" (
    echo.
    echo Deletion cancelled. Exiting...
    exit /b 0
)

echo.
echo Proceeding with deletion...
echo.

cd /d "%~dp0"
cd ..

REM Delete files
if exist "secrets\firebase.json" (
    del /F /Q "secrets\firebase.json"
    echo [x] Deleted: secrets\firebase.json
) else (
    echo [ ] Not found: secrets\firebase.json
)

if exist "secrets\firebase1.json" (
    del /F /Q "secrets\firebase1.json"
    echo [x] Deleted: secrets\firebase1.json
) else (
    echo [ ] Not found: secrets\firebase1.json
)

if exist "secrets\chatapp-27370-firebase-adminsdk-fbsvc-d4681a8c2e.json" (
    del /F /Q "secrets\chatapp-27370-firebase-adminsdk-fbsvc-d4681a8c2e.json"
    echo [x] Deleted: secrets\chatapp-27370-firebase-adminsdk-fbsvc-d4681a8c2e.json
) else (
    echo [ ] Not found: secrets\chatapp-27370-firebase-adminsdk-fbsvc-d4681a8c2e.json
)

echo.
echo ===============================================
echo  DELETION COMPLETE
echo ===============================================
echo.
echo Remaining files in secrets directory:
dir /B secrets
echo.
echo Next steps:
echo   1. Verify directory is empty or contains only .gitkeep
echo   2. Commit .gitignore: git add .gitignore ^&^& git commit -m "chore: add .gitignore"
echo   3. Run: bash scripts/create-secrets.sh
echo   4. Run: bash scripts/generate-certs.sh
echo.
pause
