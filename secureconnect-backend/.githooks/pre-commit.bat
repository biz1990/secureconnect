@echo off
REM Pre-commit hook for Go code quality checks (Windows)
REM This script runs before each commit to ensure code quality

echo Running pre-commit checks...

REM Change to the project root
cd /d "%~dp0"

echo 1. Running go fmt...
gofmt -l . >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo Code is not formatted. Run 'go fmt ./...'
    exit /b 1
)
echo Code is properly formatted

echo 2. Running go vet...
go vet ./...
if %ERRORLEVEL% neq 0 (
    echo go vet found issues
    exit /b 1
)
echo go vet passed

REM Check if golangci-lint is installed
where golangci-lint >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo 3. Running golangci-lint...
    golangci-lint run --timeout=5m ./...
    if %ERRORLEVEL% neq 0 (
        echo golangci-lint found issues
        exit /b 1
    )
    echo golangci-lint passed
) else (
    echo golangci-lint not found, skipping...
    echo Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
)

echo 4. Running tests...
go test -short ./...
if %ERRORLEVEL% neq 0 (
    echo Tests failed
    exit /b 1
)
echo Tests passed

echo All pre-commit checks passed!
exit /b 0
