# Docker Compose Production Fix

## Problem

The `docker-compose.production.yml` file was using `external: true` for Docker secrets, which requires Docker Swarm mode. When running `docker compose` (non-Swarm mode), this caused the error:

```
✘ Container secureconnect_cassandra        unsupported external secret cassandra_user
unsupported external secret cassandra_user
```

## Solution

Changed the Docker secrets configuration from `external: true` to file-based secrets that work with `docker compose`:

### Before (Docker Swarm only):
```yaml
secrets:
  cassandra_user:
    external: true
```

### After (Docker Compose compatible):
```yaml
secrets:
  cassandra_user:
    file: ./secrets/cassandra_user.txt
```

## How to Use

### Step 1: Generate Secret Files

**For Windows:**
```cmd
cd secureconnect-backend
scripts\generate-secret-files.bat
```

**For Linux/Mac:**
```bash
cd secureconnect-backend
chmod +x scripts/generate-secret-files.sh
./scripts/generate-secret-files.sh
```

This script will:
- Create a `secrets/` directory
- Generate all required secret files:
  - `jwt_secret.txt`
  - `db_password.txt`
  - `cassandra_user.txt`
  - `cassandra_password.txt`
  - `redis_password.txt`
  - `minio_access_key.txt`
  - `minio_secret_key.txt`
  - `smtp_username.txt`
  - `smtp_password.txt`
  - `firebase_project_id.txt`
  - `firebase_credentials.json`
  - `turn_user.txt`
  - `turn_password.txt`

### Step 2: Generate TLS Certificates (Optional but Recommended)

**For Windows:**
```cmd
scripts\generate-certs.bat
```

**For Linux/Mac:**
```bash
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh
```

### Step 3: Build and Start Services

```bash
docker compose -f docker-compose.production.yml up -d --build
```

## Security Notes

1. **Never commit secrets to version control** - The `secrets/` directory is already in `.gitignore`
2. **Store backup copies securely** - Save generated secrets in a password manager
3. **File permissions** - On Linux/Mac, secret files are set to `600` (owner read/write only)
4. **Rotate secrets regularly** - Update secret files and restart services as needed

## Troubleshooting

### Error: "no such file or directory" for secret files

Make sure you ran the secret generation script before starting the services:
```bash
# Windows
scripts\generate-secret-files.bat

# Linux/Mac
./scripts/generate-secret-files.sh
```

### Error: "permission denied" accessing secret files

On Linux/Mac, ensure proper permissions:
```bash
chmod 600 secrets/*.txt
```

### Want to use Docker Swarm instead?

If you prefer using Docker Swarm with true external secrets:
1. Initialize Swarm: `docker swarm init`
2. Run: `./scripts/create-secrets.sh` (creates Docker Swarm secrets)
3. Revert `docker-compose.production.yml` to use `external: true`
4. Deploy with: `docker stack deploy -c docker-compose.production.yml secureconnect`

## Migration from Old Configuration

If you previously had external secrets created via Docker Swarm:

1. Export existing secrets:
```bash
docker secret ls -q | xargs -I {} sh -c 'docker secret inspect {} | jq -r ".[0].Spec.Name" > secrets/{}.txt'
```

2. Or simply regenerate new secrets using the provided script.

## Files Modified

- `docker-compose.production.yml` - Changed secrets from `external: true` to file-based
- `scripts/generate-secret-files.sh` - New script for Linux/Mac
- `scripts/generate-secret-files.bat` - New script for Windows
- `.gitignore` - Already includes `secrets/` directory (no change needed)
