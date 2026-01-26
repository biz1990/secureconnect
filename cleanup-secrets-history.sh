#!/bin/bash

###############################################################################
# SECURECONNECT SECRETS REMEDIATION SCRIPT
# Option A: Using BFG Repo-Cleaner
#
# WARNING: This script will rewrite git history and force push changes.
#          All team members must re-clone the repository after execution.
#
# Usage: ./cleanup-secrets-history.sh
###############################################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}SECURECONNECT SECRETS REMEDIATION${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo -e "${RED}WARNING: This will rewrite git history!${NC}"
echo -e "${RED}All team members must re-clone after execution.${NC}"
echo ""
read -p "Do you want to continue? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

# Get the current directory
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo -e "${GREEN}Repository directory: $REPO_DIR${NC}"
echo ""

# Step 1: Create backup
echo -e "${YELLOW}[1/7] Creating backup...${NC}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR="../secureconnect-backup-$TIMESTAMP"
cp -r "$REPO_DIR" "$BACKUP_DIR"
echo -e "${GREEN}Backup created: $BACKUP_DIR${NC}"
echo ""

# Step 2: Check if BFG is installed
echo -e "${YELLOW}[2/7] Checking for BFG Repo-Cleaner...${NC}"
if ! command -v bfg &> /dev/null; then
    echo -e "${RED}ERROR: BFG Repo-Cleaner is not installed.${NC}"
    echo ""
    echo "Please install BFG Repo-Cleaner:"
    echo "  macOS:   brew install bfg"
    echo "  Linux:   wget https://repo1.maven.org/maven2/com/madgag/bfg/1.14.0/bfg-1.14.0.jar"
    echo "  Windows:  Download from https://rtyley.github.io/bfg-repo-cleaner/"
    echo ""
    exit 1
fi
echo -e "${GREEN}BFG Repo-Cleaner found: $(bfg --version)${NC}"
echo ""

# Step 3: Create mirror clone
echo -e "${YELLOW}[3/7] Creating mirror clone...${NC}"
cd ..
MIRROR_DIR="secureconnect-mirror-$TIMESTAMP"
if [ -d "$MIRROR_DIR" ]; then
    echo -e "${RED}ERROR: Mirror directory already exists: $MIRROR_DIR${NC}"
    exit 1
fi
git clone --mirror secureconnect "$MIRROR_DIR"
echo -e "${GREEN}Mirror created: $MIRROR_DIR${NC}"
echo ""

# Step 4: Remove secrets directory from history
echo -e "${YELLOW}[4/7] Removing secrets directory from git history...${NC}"
cd "$MIRROR_DIR"
bfg --delete-folders secureconnect-backend/secrets --no-blob-protection
echo -e "${GREEN}Secrets directory removed from history${NC}"
echo ""

# Step 5: Clean up refs
echo -e "${YELLOW}[5/7] Cleaning up refs...${NC}"
git reflog expire --expire=now --all
git gc --prune=now --aggressive
echo -e "${GREEN}Refs cleaned up${NC}"
echo ""

# Step 6: Force push to remote
echo -e "${YELLOW}[6/7] Force pushing to remote repository...${NC}"
echo -e "${RED}This will rewrite history on the remote!${NC}"
read -p "Are you sure you want to force push? (yes/no): " push_confirm
if [ "$push_confirm" != "yes" ]; then
    echo "Force push aborted. You can manually push later:"
    echo "  cd $MIRROR_DIR"
    echo "  git push origin --force --all"
    exit 0
fi

git push origin --force --all
echo -e "${GREEN}Force push completed${NC}"
echo ""

# Step 7: Clean up mirror
echo -e "${YELLOW}[7/7] Cleaning up mirror...${NC}"
cd ..
rm -rf "$MIRROR_DIR"
echo -e "${GREEN}Mirror cleaned up${NC}"
echo ""

# Return to original directory
cd "$REPO_DIR"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}CLEANUP COMPLETED SUCCESSFULLY${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Next steps:"
echo "  1. Notify all team members to re-clone the repository"
echo "  2. Regenerate all secrets (see SECRETS_REMEDIATION_PLAN.md)"
echo "  3. Verify secrets are removed from git history"
echo "  4. Test Docker secrets work with new secrets"
echo ""
echo "Backup location: $BACKUP_DIR"
echo ""

# Verification commands
echo -e "${YELLOW}VERIFICATION COMMANDS:${NC}"
echo ""
echo "To verify secrets are removed from git history, run:"
echo "  cd $REPO_DIR"
echo "  git log --all --full-history --format=%H -- 'secureconnect-backend/secrets/' | head -n 1"
echo ""
echo "This should return nothing if secrets are properly removed."
echo ""
