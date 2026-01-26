# SECRETS REMEDIATION PLAN
**Critical Security Incident**: Secrets Committed to Git Repository
**Date**: 2026-01-24
**Severity**: CRITICAL - P0

---

## SUMMARY

The following secret files have been committed to the repository and must be removed from git history immediately:

| Secret File | Risk Level | Impact |
|-------------|-------------|--------|
| `secureconnect-backend/secrets/jwt_secret.txt` | CRITICAL | JWT signing key exposed - complete authentication bypass |
| `secureconnect-backend/secrets/db_password.txt` | CRITICAL | Database password exposed - full data access |
| `secureconnect-backend/secrets/cassandra_user.txt` | HIGH | Cassandra username exposed |
| `secureconnect-backend/secrets/cassandra_password.txt` | CRITICAL | Cassandra password exposed - full data access |
| `secureconnect-backend/secrets/redis_password.txt` | HIGH | Redis password exposed - cache access |
| `secureconnect-backend/secrets/minio_access_key.txt` | HIGH | MinIO access key exposed - storage access |
| `secureconnect-backend/secrets/minio_secret_key.txt` | CRITICAL | MinIO secret key exposed - full storage access |
| `secureconnect-backend/secrets/smtp_username.txt` | MEDIUM | SMTP username exposed |
| `secureconnect-backend/secrets/smtp_password.txt` | HIGH | SMTP password exposed - email spoofing |
| `secureconnect-backend/secrets/firebase_credentials.json` | CRITICAL | Firebase service account exposed - push notification access |
| `secureconnect-backend/secrets/firebase_project_id.txt` | MEDIUM | Firebase project ID exposed |
| `secureconnect-backend/secrets/turn_user.txt` | MEDIUM | TURN username exposed |
| `secureconnect-backend/secrets/turn_password.txt` | HIGH | TURN password exposed - relay access |
| `secureconnect-backend/secrets/grafana_admin_password.txt` | MEDIUM | Grafana password exposed - dashboard access |

---

## REMEDIATION CHECKLIST

### Phase 1: Immediate Actions (Do First)

- [ ] **STOP ALL PRODUCTION DEPLOYMENTS** - If any production systems are running, stop them immediately
- [ ] **NOTIFY SECURITY TEAM** - Alert all stakeholders about the security breach
- [ ] **ROTATE ALL EXPOSED CREDENTIALS** - Change all passwords, keys, and tokens
- [ ] **REVOKE EXPOSED TOKENS** - Invalidate any JWTs, API keys, or tokens that may have been issued
- [ ] **AUDIT ACCESS LOGS** - Review access logs for unauthorized access using exposed credentials

### Phase 2: Git History Cleanup

- [ ] Backup current working directory
- [ ] Create fresh clone for cleanup
- [ ] Remove secrets from git history using BFG or git filter-repo
- [ ] Force push cleaned history
- [ ] Verify secrets are removed from all branches

### Phase 3: Regenerate Secrets

- [ ] Generate new JWT secret (32+ characters)
- [ ] Generate new database password (24+ characters)
- [ ] Generate new Cassandra credentials
- [ ] Generate new Redis password
- [ ] Generate new MinIO credentials
- [ ] Generate new SMTP credentials
- [ ] Generate new Firebase service account
- [ ] Generate new TURN credentials
- [ ] Generate new Grafana password

### Phase 4: Verification

- [ ] Verify secrets are not in git history
- [ ] Verify .gitignore is working correctly
- [ ] Verify Docker secrets pattern is preserved
- [ ] Verify services work with new secrets

---

## EXACT GIT COMMANDS

### Option A: Using BFG Repo-Cleaner (Recommended - Faster)

```bash
# Step 1: Install BFG Repo-Cleaner
# Download from: https://rtyley.github.io/bfg-repo-cleaner/
# Or on macOS: brew install bfg
# Or on Linux: wget https://repo1.maven.org/maven2/com/madgag/bfg/1.14.0/bfg-1.14.0.jar

# Step 2: Create a backup of your repository
cd d:/secureconnect
cp -r . ../secureconnect-backup-$(date +%Y%m%d-%H%M%S)

# Step 3: Create a fresh clone (DO NOT work on your main repo)
cd ..
git clone --mirror secureconnect secureconnect-mirror
cd secureconnect-mirror

# Step 4: Remove secrets directory from history
# This removes all files in the secrets/ directory from git history
bfg --delete-folders secureconnect-backend/secrets --no-blob-protection

# Step 5: Clean up refs
git reflog expire --expire=now --all
git gc --prune=now --aggressive

# Step 6: Force push to clean the remote repository
# WARNING: This will rewrite history - ensure all team members are aware
git push origin --force --all

# Step 7: Clean up the mirror clone
cd ..
rm -rf secureconnect-mirror
```

### Option B: Using git filter-repo (Alternative)

```bash
# Step 1: Install git-filter-repo
pip install git-filter-repo

# Step 2: Create a backup of your repository
cd d:/secureconnect
cp -r . ../secureconnect-backup-$(date +%Y%m%d-%H%M%S)

# Step 3: Remove secrets directory from history
git filter-repo --path secureconnect-backend/secrets/ --invert-paths

# Step 4: Clean up refs
git for-each-ref --format='delete %(refname)' refs/original | git update-ref --stdin
git reflog expire --expire=now --all
git gc --prune=now --aggressive

# Step 5: Force push to clean the remote repository
# WARNING: This will rewrite history - ensure all team members are aware
git push origin --force --all

# Step 6: Clean up
git remote prune origin
```

### Option C: Using git filter-branch (Built-in, Slower)

```bash
# Step 1: Create a backup of your repository
cd d:/secureconnect
cp -r . ../secureconnect-backup-$(date +%Y%m%d-%H%M%S)

# Step 2: Remove secrets directory from history
git filter-branch --force --index-filter \
  'git rm --cached -rf --ignore-unmatch secureconnect-backend/secrets/' \
  --prune-empty --tag-name-filter cat

# Step 3: Clean up refs
git for-each-ref --format='delete %(refname)' refs/original | git update-ref --stdin
git reflog expire --expire=now --all
git gc --prune=now --aggressive

# Step 4: Force push to clean the remote repository
# WARNING: This will rewrite history - ensure all team members are aware
git push origin --force --all
```

---

## REGENERATE ALL COMPROMISED SECRETS

### Shell Commands to Generate New Secrets

```bash
#!/bin/bash

# Navigate to secrets directory
cd secureconnect-backend/secrets

# Generate new JWT secret (32 bytes = 64 hex characters)
openssl rand -hex 32 > jwt_secret.txt
echo "Generated: jwt_secret.txt"

# Generate new database password (24 bytes = 48 hex characters)
openssl rand -hex 24 > db_password.txt
echo "Generated: db_password.txt"

# Generate new Cassandra username (16 bytes = 32 hex characters)
openssl rand -hex 16 > cassandra_user.txt
echo "Generated: cassandra_user.txt"

# Generate new Cassandra password (24 bytes = 48 hex characters)
openssl rand -hex 24 > cassandra_password.txt
echo "Generated: cassandra_password.txt"

# Generate new Redis password (24 bytes = 48 hex characters)
openssl rand -hex 24 > redis_password.txt
echo "Generated: redis_password.txt"

# Generate new MinIO access key (20 bytes = 40 hex characters)
openssl rand -hex 20 > minio_access_key.txt
echo "Generated: minio_access_key.txt"

# Generate new MinIO secret key (40 bytes = 80 hex characters)
openssl rand -hex 40 > minio_secret_key.txt
echo "Generated: minio_secret_key.txt"

# Generate new TURN username (16 bytes = 32 hex characters)
openssl rand -hex 16 > turn_user.txt
echo "Generated: turn_user.txt"

# Generate new TURN password (24 bytes = 48 hex characters)
openssl rand -hex 24 > turn_password.txt
echo "Generated: turn_password.txt"

# Generate new Grafana password (24 bytes = 48 hex characters)
openssl rand -hex 24 > grafana_admin_password.txt
echo "Generated: grafana_admin_password.txt"

# SMTP credentials and Firebase credentials must be generated manually
# SMTP: Use your email provider's interface (SendGrid, Mailgun, AWS SES, etc.)
# Firebase: Go to Firebase Console > Project Settings > Service Accounts

echo ""
echo "=========================================="
echo "IMPORTANT: Generate these manually:"
echo "1. SMTP credentials - use your email provider"
echo "2. Firebase service account JSON - use Firebase Console"
echo "=========================================="
```

### PowerShell Version (for Windows)

```powershell
# Navigate to secrets directory
cd secureconnect-backend/secrets

# Generate new JWT secret
$jwt = -join ((1..32) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$jwt | Out-File -Encoding utf8 jwt_secret.txt
Write-Host "Generated: jwt_secret.txt"

# Generate new database password
$dbPass = -join ((1..24) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$dbPass | Out-File -Encoding utf8 db_password.txt
Write-Host "Generated: db_password.txt"

# Generate new Cassandra username
$cassUser = -join ((1..16) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$cassUser | Out-File -Encoding utf8 cassandra_user.txt
Write-Host "Generated: cassandra_user.txt"

# Generate new Cassandra password
$cassPass = -join ((1..24) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$cassPass | Out-File -Encoding utf8 cassandra_password.txt
Write-Host "Generated: cassandra_password.txt"

# Generate new Redis password
$redisPass = -join ((1..24) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$redisPass | Out-File -Encoding utf8 redis_password.txt
Write-Host "Generated: redis_password.txt"

# Generate new MinIO access key
$minioKey = -join ((1..20) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$minioKey | Out-File -Encoding utf8 minio_access_key.txt
Write-Host "Generated: minio_access_key.txt"

# Generate new MinIO secret key
$minioSecret = -join ((1..40) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$minioSecret | Out-File -Encoding utf8 minio_secret_key.txt
Write-Host "Generated: minio_secret_key.txt"

# Generate new TURN username
$turnUser = -join ((1..16) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$turnUser | Out-File -Encoding utf8 turn_user.txt
Write-Host "Generated: turn_user.txt"

# Generate new TURN password
$turnPass = -join ((1..24) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$turnPass | Out-File -Encoding utf8 turn_password.txt
Write-Host "Generated: turn_password.txt"

# Generate new Grafana password
$grafanaPass = -join ((1..24) | ForEach-Object { "{0:x}" -f (Get-Random -Max 256) })
$grafanaPass | Out-File -Encoding utf8 grafana_admin_password.txt
Write-Host "Generated: grafana_admin_password.txt"

Write-Host ""
Write-Host "=========================================="
Write-Host "IMPORTANT: Generate these manually:"
Write-Host "1. SMTP credentials - use your email provider"
Write-Host "2. Firebase service account JSON - use Firebase Console"
Write-Host "=========================================="
```

---

## VERIFY SECRETS REMOVED FROM GIT HISTORY

### Verification Commands

```bash
# Step 1: Check if secrets directory exists in any commit
git log --all --full-history --format=%H -- "secureconnect-backend/secrets/" | head -n 1

# If this returns any commit hash, secrets are still in history

# Step 2: Search for specific secret content in git history
# WARNING: This will search ALL commits - may take time
git log --all --oneline --source -- "*jwt_secret*" | head -n 5
git log --all --oneline --source -- "*db_password*" | head -n 5
git log --all --oneline --source -- "*cassandra_password*" | head -n 5
git log --all --oneline --source -- "*redis_password*" | head -n 5
git log --all --oneline --source -- "*minio_secret_key*" | head -n 5
git log --all --oneline --source -- "*firebase_credentials*" | head -n 5

# Step 3: Verify .gitignore is working
cat .gitignore | grep secrets

# Should show: secrets/

# Step 4: Check if secrets directory is tracked
git ls-files | grep secrets/

# Should return nothing if secrets are properly ignored

# Step 5: Verify Docker secrets pattern is preserved
grep -r "/run/secrets/" secureconnect-backend/docker-compose*.yml

# Should show Docker secrets references are intact
```

---

## PRESERVE DOCKER SECRETS PATTERN

The Docker secrets pattern (`/run/secrets/*`) is already correctly configured in [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:8-34). This pattern is preserved and should NOT be modified.

### Docker Secrets Configuration (DO NOT MODIFY)

```yaml
secrets:
  jwt_secret:
    file: ./secrets/jwt_secret.txt
  db_password:
    file: ./secrets/db_password.txt
  cassandra_user:
    file: ./secrets/cassandra_user.txt
  cassandra_password:
    file: ./secrets/cassandra_password.txt
  redis_password:
    file: ./secrets/redis_password.txt
  minio_access_key:
    file: ./secrets/minio_access_key.txt
  minio_secret_key:
    file: ./secrets/minio_secret_key.txt
  smtp_username:
    file: ./secrets/smtp_username.txt
  smtp_password:
    file: ./secrets/smtp_password.txt
  firebase_project_id:
    file: ./secrets/firebase_project_id.txt
  firebase_credentials:
    file: ./secrets/firebase_credentials.json
  turn_user:
    file: ./secrets/turn_user.txt
  turn_password:
    file: ./secrets/turn_password.txt
```

### Environment Variable Pattern (DO NOT MODIFY)

```yaml
environment:
  - JWT_SECRET_FILE=/run/secrets/jwt_secret
  - DB_PASSWORD_FILE=/run/secrets/db_password
  - CASSANDRA_USER_FILE=/run/secrets/cassandra_user
  - CASSANDRA_PASSWORD_FILE=/run/secrets/cassandra_password
  - REDIS_PASSWORD_FILE=/run/secrets/redis_password
  - MINIO_ACCESS_KEY_FILE=/run/secrets/minio_access_key
  - MINIO_SECRET_KEY_FILE=/run/secrets/minio_secret_key
  - SMTP_USERNAME_FILE=/run/secrets/smtp_username
  - SMTP_PASSWORD_FILE=/run/secrets/smtp_password
  - FIREBASE_PROJECT_ID_FILE=/run/secrets/firebase_project_id
  - FIREBASE_CREDENTIALS_PATH=/run/secrets/firebase_credentials
  - TURN_USER_FILE=/run/secrets/turn_user
  - TURN_PASSWORD_FILE=/run/secrets/turn_password
```

---

## POST-REMEDIATION ACTIONS

### After Git History Cleanup

```bash
# 1. Notify all team members to re-clone the repository
# Send email: "Repository history has been rewritten. Please re-clone from origin."

# 2. Update all local branches
git fetch --all
git reset --hard origin/main

# 3. Verify .gitignore is correct
cat .gitignore | grep -E "^secrets/|\.env\.|firebase.*\.json"

# Expected output:
# secrets/
# .env.*
# firebase*.json

# 4. Test Docker secrets work with new secrets
cd secureconnect-backend
docker-compose -f docker-compose.production.yml config

# Should show secrets are properly mounted

# 5. Start services with new secrets
docker-compose -f docker-compose.production.yml up -d

# 6. Verify services start correctly
docker-compose -f docker-compose.production.yml ps

# 7. Check logs for any secret-related errors
docker-compose -f docker-compose.production.yml logs
```

---

## SECURITY INCIDENT RESPONSE CHECKLIST

### Immediate Actions (Within 1 Hour)

- [ ] Stop all production deployments
- [ ] Rotate all exposed credentials
- [ ] Revoke all issued JWTs and tokens
- [ ] Notify security team and stakeholders
- [ ] Begin git history cleanup

### Short-term Actions (Within 24 Hours)

- [ ] Complete git history cleanup
- [ ] Regenerate all secrets
- [ ] Update all production systems with new secrets
- [ ] Review access logs for unauthorized access
- [ ] Audit all systems that used exposed credentials

### Long-term Actions (Within 1 Week)

- [ ] Implement secret scanning in CI/CD pipeline
- [ ] Add pre-commit hooks to prevent secret commits
- [ ] Review and update .gitignore patterns
- [ ] Document secret management procedures
- [ ] Conduct security training for team members

---

## PREVENTION MEASURES

### Pre-commit Hook to Prevent Secret Commits

Create `.git/hooks/pre-commit`:

```bash
#!/bin/bash

# Patterns that indicate secrets
SECRET_PATTERNS=(
    "password"
    "secret"
    "api_key"
    "private_key"
    "jwt_secret"
    "firebase_credentials"
    "minio_secret"
)

# Check staged files
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM)

for FILE in $STAGED_FILES; do
    # Skip if file is in secrets directory (allowed for local development)
    if [[ $FILE == secrets/* ]]; then
        continue
    fi
    
    # Check for secret patterns in file content
    for PATTERN in "${SECRET_PATTERNS[@]}"; do
        if git diff --cached "$FILE" | grep -i "$PATTERN" > /dev/null; then
            echo "ERROR: Potential secret detected in $FILE"
            echo "Pattern matched: $PATTERN"
            echo "Commit blocked. Remove sensitive data or add to .gitignore."
            exit 1
        fi
    done
done

exit 0
```

Make it executable:

```bash
chmod +x .git/hooks/pre-commit
```

### CI/CD Secret Scanning

Add to your CI pipeline (GitHub Actions, GitLab CI, etc.):

```yaml
# Example for GitHub Actions
- name: Scan for secrets
  run: |
    # Install truffleHog or gitleaks
    pip install truffleHog
    
    # Scan repository
    trufflehog git https://github.com/your-org/secureconnect --json --results_path=secrets.json
    
    # Fail if secrets found
    if [ -s secrets.json ]; then
      echo "Secrets detected in repository!"
      cat secrets.json
      exit 1
    fi
```

---

## IMPORTANT NOTES

1. **DO NOT** modify application logic - only git history and secrets files
2. **DO NOT** remove Docker secrets pattern (`/run/secrets/*`) - this is correct
3. **DO** backup your repository before running git history cleanup commands
4. **DO** notify all team members before force pushing
5. **DO** verify secrets are completely removed from git history
6. **DO** regenerate ALL secrets, even if you think some weren't exposed
7. **DO** revoke all tokens and sessions issued with old secrets

---

**Document Version**: 1.0
**Last Updated**: 2026-01-24
**Status**: READY FOR EXECUTION
