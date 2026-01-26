# Git Large File Issue - Resolution Plan

**Date:** 2026-01-26T13:26:20+07:00  
**Issue:** Git push rejected due to large files in history (324.60 MB)  
**Error:** `File secureconnect-mirror/objects/pack/pack-d03ae73f730ea6c2f88b138ac98bb5bd6ca4f92b.pack is 324.60 MB`

---

## Problem Analysis

### Large Files in Git History

| File Pattern | Size | Count | Total Size |
|--------------|------|-------|------------|
| `*.exe` binaries | 60-66 MB each | 10+ | ~600+ MB |
| `secureconnect-mirror/` | 324.60 MB | 1 pack file | 324.60 MB |

**Root Cause:** Compiled binaries and git mirror directory were accidentally committed to repository.

### Specific Files Found

1. **Compiled Binaries:**
   - `secureconnect-backend/bin/video-service.exe` (66 MB)
   - `secureconnect-backend/bin/auth-service.exe` (61 MB)
   - `secureconnect-backend/chat-service.exe` (61 MB)
   - Multiple versions of each service

2. **Git Mirror Directory:**
   - `secureconnect-mirror/objects/pack/pack-d03ae73f730ea6c2f88b138ac98bb5bd6ca4f92b.pack` (324.60 MB)
   - Entire `.git` mirror directory structure

---

## Solution Options

### Option 1: Use BFG Repo-Cleaner (Recommended)

**Pros:**
- Fast and efficient
- Specifically designed for this purpose
- Safer than git filter-branch

**Cons:**
- Requires Java installation
- Requires downloading BFG tool

**Steps:**
```powershell
# 1. Download BFG Repo-Cleaner
# Already documented in BFG_INSTALLATION_GUIDE.md

# 2. Clone a fresh copy (backup)
cd d:\
git clone --mirror https://github.com/biz1990/secureconnect.git secureconnect-backup.git

# 3. Remove large files
cd d:\secureconnect
java -jar bfg-1.14.0.jar --delete-files "*.exe"
java -jar bfg-1.14.0.jar --delete-folders "secureconnect-mirror"

# 4. Clean up
git reflog expire --expire=now --all
git gc --prune=now --aggressive

# 5. Force push
git push origin --force --all
```

### Option 2: Use git filter-repo (Alternative)

**Pros:**
- Official Git recommendation
- More powerful than filter-branch

**Cons:**
- Requires Python installation
- More complex syntax

**Steps:**
```powershell
# 1. Install git filter-repo
pip install git-filter-repo

# 2. Remove files
cd d:\secureconnect
git filter-repo --path secureconnect-backend/bin --invert-paths
git filter-repo --path secureconnect-mirror --invert-paths
git filter-repo --path-glob '*.exe' --invert-paths

# 3. Force push
git push origin --force --all
```

### Option 3: Rewrite History Manually (Not Recommended)

Using `git filter-branch` - deprecated and slow.

---

## Recommended Approach

### Step 1: Update .gitignore

First, ensure these files won't be committed again:

```bash
# Add to .gitignore
echo "*.exe" >> .gitignore
echo "*.dll" >> .gitignore
echo "bin/" >> .gitignore
echo "secureconnect-mirror/" >> .gitignore
```

### Step 2: Remove from Current Working Directory

```powershell
# Remove if they exist in working directory
Remove-Item -Path "d:\secureconnect\secureconnect-backend\bin" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path "d:\secureconnect\secureconnect-mirror" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path "d:\secureconnect\secureconnect-backend\*.exe" -Force -ErrorAction SilentlyContinue
```

### Step 3: Clean Git History with BFG

```powershell
# Navigate to repo
cd d:\secureconnect

# Remove .exe files from history
java -jar bfg-1.14.0.jar --delete-files "*.exe"

# Remove secureconnect-mirror directory from history
java -jar bfg-1.14.0.jar --delete-folders "secureconnect-mirror"

# Clean up repository
git reflog expire --expire=now --all
git gc --prune=now --aggressive

# Verify size reduction
git count-objects -vH
```

### Step 4: Force Push

```powershell
# Push all branches
git push origin --force --all

# Push all tags
git push origin --force --tags
```

---

## Quick Fix (If BFG Not Available)

If you can't use BFG immediately, you can:

1. **Create a new branch without the large files:**
```powershell
# Create a new orphan branch
git checkout --orphan clean-branch

# Add only the files you want
git add configs/ cmd/ internal/ pkg/ scripts/ *.md *.yml *.yaml Dockerfile .gitignore

# Commit
git commit -m "Clean repository without binaries"

# Replace main/master branch
git branch -D main
git branch -m main

# Force push
git push origin main --force
```

**⚠️ WARNING:** This loses all git history. Only use as last resort.

---

## Prevention

### Update .gitignore

Add these patterns to `.gitignore`:

```gitignore
# Compiled binaries
*.exe
*.dll
*.so
*.dylib

# Build directories
bin/
build/
dist/
target/

# Git mirrors
*.git/
*-mirror/

# Large files
*.pack
```

### Add Pre-commit Hook

Create `.git/hooks/pre-commit`:

```bash
#!/bin/sh
# Prevent committing large files

MAX_SIZE=10485760  # 10MB in bytes

large_files=$(git diff --cached --name-only | while read file; do
    if [ -f "$file" ]; then
        size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)
        if [ $size -gt $MAX_SIZE ]; then
            echo "$file ($size bytes)"
        fi
    fi
done)

if [ -n "$large_files" ]; then
    echo "ERROR: Attempting to commit large files:"
    echo "$large_files"
    echo "Please use Git LFS or remove these files"
    exit 1
fi
```

---

## Immediate Action Plan

1. **Update .gitignore** (safe, no history changes)
2. **Remove files from working directory** (safe, local only)
3. **Choose removal method:**
   - If BFG available: Use Option 1
   - If not: Use Quick Fix (new orphan branch)
4. **Force push** (requires coordination with team)
5. **Notify team** to re-clone repository

---

## Verification

After cleanup:

```powershell
# Check repository size
git count-objects -vH

# Verify no large files remain
git rev-list --objects --all | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' | Where-Object {$_ -match '^blob'} | ForEach-Object {$parts = $_ -split '\s+'; [PSCustomObject]@{Size=[int64]$parts[2]; Path=$parts[3..$parts.Length] -join ' '}} | Where-Object {$_.Size -gt 10MB}

# Should return nothing or only small files
```

---

## Risk Assessment

| Action | Risk | Impact |
|--------|------|--------|
| Update .gitignore | ✅ LOW | Prevents future issues |
| Remove from working dir | ✅ LOW | Local only |
| BFG cleanup | ⚠️ MEDIUM | Rewrites history, requires force push |
| Orphan branch | 🔴 HIGH | Loses all history |
| Force push | ⚠️ MEDIUM | Team must re-clone |

---

## Recommended Next Steps

1. **Immediate:** Update .gitignore to prevent re-committing
2. **Short-term:** Use BFG to clean history
3. **Long-term:** Set up pre-commit hooks to prevent large files

---

**Status:** Analysis complete, ready for cleanup  
**Recommended:** Use BFG Repo-Cleaner (Option 1)  
**Estimated Time:** 10-15 minutes
