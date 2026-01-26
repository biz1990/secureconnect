# BFG REPO-CLEANER INSTALLATION AND USAGE GUIDE
**For Windows Systems**

---

## WHAT IS BFG REPO-CLEANER?

BFG Repo-Cleaner is a Java-based tool for removing large files or sensitive data from Git history. It's faster and easier to use than git-filter-repo.

---

## INSTALLATION INSTRUCTIONS

### Step 1: Download BFG

**Option A: Download from Website**
1. Go to: https://rtyley.github.io/bfg-repo-cleaner/
2. Download the latest JAR file (e.g., `bfg-1.14.0.jar`)
3. Save to a directory in your PATH (recommended)

**Option B: Download via PowerShell**
```powershell
# Download to current directory
Invoke-WebRequest -Uri "https://repo1.maven.org/maven2/com/madgag/bfg/1.14.0/bfg-1.14.0.jar" -OutFile "bfg-1.14.0.jar"
```

**Option C: Download via curl (if available)**
```bash
curl -L -o bfg-1.14.0.jar https://repo1.maven.org/maven2/com/madgag/bfg/1.14.0/bfg-1.14.0.jar
```

### Step 2: Add to PATH (Recommended)

**Option A: Add to System PATH**
1. Create a directory for BFG (e.g., `C:\Tools\BFG`)
2. Move the downloaded JAR file to that directory
3. Add the directory to your system PATH:
   - Press `Win + R`, type `sysdm.cpl`, press Enter
   - Go to "Advanced" > "Environment Variables"
   - Under "System variables", edit "Path"
   - Add `C:\Tools\BFG` to the list
4. Restart your terminal/command prompt

**Option B: Create a batch file wrapper (Easier)**
Create a file named `bfg.bat` in a directory in your PATH:

```batch
@echo off
java -jar "C:\Path\To\bfg-1.14.0.jar" %*
```

Replace `C:\Path\To\bfg-1.14.0.jar` with the actual path to your JAR file.

---

## USAGE INSTRUCTIONS

### Option A: Using Java Directly (After PATH Setup)

```bash
# Check version
bfg --version

# Run cleanup
bfg --delete-folders secureconnect-backend/secrets --no-blob-protection
```

### Option B: Using Java with JAR Path

```bash
# Navigate to repository
cd d:\secureconnect

# Run BFG directly with Java
java -jar "C:\Path\To\bfg-1.14.0.jar" --delete-folders secureconnect-backend/secrets --no-blob-protection
```

### Option C: Using Batch File Wrapper

```batch
# Navigate to repository
cd d:\secureconnect

# Run cleanup
bfg --delete-folders secureconnect-backend/secrets --no-blob-protection
```

---

## COMPLETE CLEANUP WORKFLOW

### Step 1: Download and Install BFG

```powershell
# Download BFG
Invoke-WebRequest -Uri "https://repo1.maven.org/maven2/com/madgag/bfg/1.14.0/bfg-1.14.0.jar" -OutFile "C:\Tools\bfg-1.14.0.jar"

# Create batch wrapper
@"
@echo off
java -jar "C:\Tools\bfg-1.14.0.jar" %*
"@ | Out-File "C:\Tools\bfg.bat" -Encoding ascii
```

### Step 2: Navigate to Repository

```powershell
cd d:\secureconnect
```

### Step 3: Create Mirror Clone

```bash
# Go to parent directory
cd ..

# Create mirror clone
git clone --mirror secureconnect secureconnect-mirror

# Enter mirror
cd secureconnect-mirror
```

### Step 4: Run BFG to Remove Secrets

```bash
# Remove secrets directory from history
bfg --delete-folders secureconnect-backend/secrets --no-blob-protection
```

Or using Java directly:

```bash
java -jar "C:\Tools\bfg-1.14.0.jar" --delete-folders secureconnect-backend/secrets --no-blob-protection
```

### Step 5: Clean Up Refs

```bash
git reflog expire --expire=now --all
git gc --prune=now --aggressive
```

### Step 6: Force Push to Remote

```bash
git push origin --force --all
```

### Step 7: Clean Up Mirror

```bash
# Go back to parent
cd ..

# Remove mirror directory
rm -rf secureconnect-mirror
```

---

## QUICK START (One-Liner Commands)

If you have BFG in PATH:

```powershell
cd d:\secureconnect; cd ..; git clone --mirror secureconnect secureconnect-mirror; cd secureconnect-mirror; bfg --delete-folders secureconnect-backend/secrets --no-blob-protection; git reflog expire --expire=now --all; git gc --prune=now --aggressive; git push origin --force --all; cd ..; rm -rf secureconnect-mirror
```

If using Java directly:

```powershell
cd d:\secureconnect; cd ..; git clone --mirror secureconnect secureconnect-mirror; cd secureconnect-mirror; java -jar "C:\Tools\bfg-1.14.0.jar" --delete-folders secureconnect-backend/secrets --no-blob-protection; git reflog expire --expire=now --all; git gc --prune=now --aggressive; git push origin --force --all; cd ..; rm -rf secureconnect-mirror
```

---

## COMMON BFG COMMANDS

| Command | Description |
|---------|-------------|
| `bfg --version` | Show BFG version |
| `bfg --delete-folders <path>` | Remove folder from history |
| `bfg --delete-files <pattern>` | Remove files matching pattern |
| `bfg --strip-blobs-bigger-than <size>` | Remove large files |
| `bfg --no-blob-protection` | Skip blob protection checks |
| `bfg --protect-blobs-from <file>` | Protect specific blobs |

---

## VERIFICATION

After cleanup, verify secrets are removed:

```bash
# Check if secrets directory exists in any commit
git log --all --full-history --format=%H -- 'secureconnect-backend/secrets/'

# Should return nothing if cleanup was successful
```

---

## TROUBLESHOOTING

### "bfg is not recognized"

**Cause**: BFG is not in your PATH

**Solution**: Use Java directly:
```bash
java -jar "C:\Path\To\bfg-1.14.0.jar" <command>
```

### "Java is not recognized"

**Cause**: Java is not installed or not in PATH

**Solution**: Install Java JDK from https://www.oracle.com/java/technologies/downloads/

### "Error: Could not find or load main class"

**Cause**: JAR file is corrupted or download failed

**Solution**: Re-download the JAR file

### "Permission denied"

**Cause**: Git repository is read-only

**Solution**: Check file permissions on the repository

---

## ALTERNATIVE: GIT-FILTER-REPO

If BFG doesn't work, use git-filter-repo:

```bash
# Install
pip install git-filter-repo

# Remove secrets from history
git filter-repo --path secureconnect-backend/secrets/ --invert-paths

# Clean up
git for-each-ref --format='delete %(refname)' refs/original | git update-ref --stdin
git reflog expire --expire=now --all
git gc --prune=now --aggressive

# Force push
git push origin --force --all
```

---

**Document Version**: 1.0
**Last Updated**: 2026-01-25
