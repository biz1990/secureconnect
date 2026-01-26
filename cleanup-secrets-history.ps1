###############################################################################
# SECURECONNECT SECRETS REMEDIATION SCRIPT (PowerShell)
# Option A: Using BFG Repo-Cleaner
#
# WARNING: This script will rewrite git history and force push changes.
#          All team members must re-clone the repository after execution.
#
# Usage: .\cleanup-secrets-history.ps1
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
Write-ColorOutput "Yellow" "SECURECONNECT SECRETS REMEDIATION"
Write-ColorOutput "Yellow" "========================================"
Write-Host ""
Write-ColorOutput "Red" "WARNING: This will rewrite git history!"
Write-ColorOutput "Red" "All team members must re-clone after execution."
Write-Host ""

$confirm = Read-Host "Do you want to continue? (yes/no)"
if ($confirm -ne "yes") {
    Write-Host "Aborted."
    exit 1
}

# Get current directory
$REPO_DIR = Get-Location
Write-ColorOutput "Green" "Repository directory: $REPO_DIR"
Write-Host ""

# Step 1: Create backup
Write-ColorOutput "Yellow" "[1/7] Creating backup..."
$TIMESTAMP = Get-Date -Format "yyyyMMdd-HHmmss"
$BACKUP_DIR = "..\secureconnect-backup-$TIMESTAMP"
Copy-Item -Path "$REPO_DIR" -Destination "$BACKUP_DIR" -Recurse -Force
Write-ColorOutput "Green" "Backup created: $BACKUP_DIR"
Write-Host ""

# Step 2: Check if BFG is installed
Write-ColorOutput "Yellow" "[2/7] Checking for BFG Repo-Cleaner..."
try {
    $bfgVersion = bfg --version 2>&1
    Write-ColorOutput "Green" "BFG Repo-Cleaner found: $bfgVersion"
}
catch {
    Write-ColorOutput "Red" "ERROR: BFG Repo-Cleaner is not installed."
    Write-Host ""
    Write-Host "Please install BFG Repo-Cleaner:"
    Write-Host "  Windows:  Download from https://rtyley.github.io/bfg-repo-cleaner/"
    Write-Host ""
    exit 1
}
Write-Host ""

# Step 3: Create mirror clone
Write-ColorOutput "Yellow" "[3/7] Creating mirror clone..."
$PARENT_DIR = Split-Path -Parent $REPO_DIR
$MIRROR_DIR = "$PARENT_DIR\secureconnect-mirror-$TIMESTAMP"
if (Test-Path $MIRROR_DIR) {
    Write-ColorOutput "Red" "ERROR: Mirror directory already exists: $MIRROR_DIR"
    exit 1
}
Set-Location $PARENT_DIR
git clone --mirror secureconnect $MIRROR_DIR
Set-Location $REPO_DIR
Write-ColorOutput "Green" "Mirror created: $MIRROR_DIR"
Write-Host ""

# Step 4: Remove secrets directory from history
Write-ColorOutput "Yellow" "[4/7] Removing secrets directory from git history..."
Set-Location $MIRROR_DIR
bfg --delete-folders secureconnect-backend/secrets --no-blob-protection
Write-ColorOutput "Green" "Secrets directory removed from history"
Write-Host ""

# Step 5: Clean up refs
Write-ColorOutput "Yellow" "[5/7] Cleaning up refs..."
git reflog expire --expire=now --all
git gc --prune=now --aggressive
Write-ColorOutput "Green" "Refs cleaned up"
Write-Host ""

# Step 6: Force push to remote
Write-ColorOutput "Yellow" "[6/7] Force pushing to remote repository..."
Write-ColorOutput "Red" "This will rewrite history on the remote!"
$push_confirm = Read-Host "Are you sure you want to force push? (yes/no)"
if ($push_confirm -ne "yes") {
    Write-Host "Force push aborted. You can manually push later:"
    Write-Host "  cd $MIRROR_DIR"
    Write-Host "  git push origin --force --all"
    Set-Location $REPO_DIR
    exit 0
}

git push origin --force --all
Write-ColorOutput "Green" "Force push completed"
Write-Host ""

# Step 7: Clean up mirror
Write-ColorOutput "Yellow" "[7/7] Cleaning up mirror..."
Set-Location $PARENT_DIR
Remove-Item -Path $MIRROR_DIR -Recurse -Force
Write-ColorOutput "Green" "Mirror cleaned up"
Write-Host ""

# Return to original directory
Set-Location $REPO_DIR

Write-ColorOutput "Green" "========================================"
Write-ColorOutput "Green" "CLEANUP COMPLETED SUCCESSFULLY"
Write-ColorOutput "Green" "========================================"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Notify all team members to re-clone the repository"
Write-Host "  2. Regenerate all secrets (see SECRETS_REMEDIATION_PLAN.md)"
Write-Host "  3. Verify secrets are removed from git history"
Write-Host "  4. Test Docker secrets work with new secrets"
Write-Host ""
Write-Host "Backup location: $BACKUP_DIR"
Write-Host ""

# Verification commands
Write-ColorOutput "Yellow" "VERIFICATION COMMANDS:"
Write-Host ""
Write-Host "To verify secrets are removed from git history, run:"
Write-Host "  cd $REPO_DIR"
Write-Host "  git log --all --full-history --format=%H -- 'secureconnect-backend/secrets/' | Select-Object -First 1"
Write-Host ""
Write-Host "This should return nothing if secrets are properly removed."
Write-Host ""
