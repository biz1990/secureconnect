# Secrets Configuration Guide

This guide explains how to properly configure and manage secrets for SecureConnect.

⚠️ **SECURITY WARNING:** Never commit `.env` files or secrets to version control!

---

## Table of Contents

1. [Local Development](#local-development)
2. [Production Deployment](#production-deployment)
3. [Firebase Setup](#firebase-setup)
4. [Secret Rotation](#secret-rotation)
5. [Troubleshooting](#troubleshooting)

---

## Local Development

### Step 1: Copy Example File

```bash
cd secureconnect-backend
cp .env.example .env.local
```

### Step 2: Generate Strong Secrets

Use the following commands to generate cryptographically secure secrets:

```bash
# JWT Secret (minimum 32 characters)
openssl rand -base64 32

# MinIO Access Key (20 characters)
openssl rand -base64 24 | cut -c1-20

# MinIO Secret Key (32 characters)
openssl rand -base64 32

# TURN Password (16 characters)
openssl rand -base64 16

# Database Password (24 characters)
openssl rand -base64 24

# Redis Password (24 characters)
openssl rand -base64 24
```

### Step 3: Update .env.local

Edit the `.env.local` file with the generated values:

```bash
# Example .env.local
ENV=local

# JWT Configuration
JWT_SECRET=<paste-your-generated-jwt-secret-here>

# MinIO Configuration
MINIO_ACCESS_KEY=<paste-your-minio-access-key-here>
MINIO_SECRET_KEY=<paste-your-minio-secret-key-here>

# TURN Configuration
TURN_PASSWORD=<paste-your-turn-password-here>

# Database Configuration
DB_PASSWORD=<paste-your-db-password-here>

# Redis Configuration
REDIS_PASSWORD=<paste-your-redis-password-here>
```

### Step 4: Verify .gitignore

Ensure `.env.local` is in `.gitignore`:

```bash
# Check if .env.local is ignored
git check-ignore -v .env.local

# If not ignored, add it
echo ".env.local" >> .gitignore
```

---

## Production Deployment

### Option 1: Using Docker Secrets (Recommended)

Docker Secrets is the most secure method for production deployments.

#### Step 1: Generate Secrets

```bash
# Generate all required secrets
JWT_SECRET=$(openssl rand -base64 32)
DB_PASSWORD=$(openssl rand -base64 24)
REDIS_PASSWORD=$(openssl rand -base64 24)
MINIO_ACCESS_KEY=$(openssl rand -base64 24 | cut -c1-20)
MINIO_SECRET_KEY=$(openssl rand -base64 32)
TURN_PASSWORD=$(openssl rand -base64 16)
```

#### Step 2: Create Docker Secrets

```bash
# JWT Secret
echo "$JWT_SECRET" | docker secret create jwt_secret -

# Database Password
echo "$DB_PASSWORD" | docker secret create db_password -

# Redis Password
echo "$REDIS_PASSWORD" | docker secret create redis_password -

# MinIO Credentials
echo "$MINIO_ACCESS_KEY" | docker secret create minio_access_key -
echo "$MINIO_SECRET_KEY" | docker secret create minio_secret_key -

# TURN Password
echo "$TURN_PASSWORD" | docker secret create turn_password -

# SMTP Credentials
echo "your-smtp-username" | docker secret create smtp_username -
echo "your-smtp-password" | docker secret create smtp_password -
```

#### Step 3: Deploy with Docker Secrets

```bash
# Use production compose file
docker-compose -f docker-compose.production.yml up -d

# Secrets are automatically mounted at /run/secrets/<secret-name>
```

### Option 2: Using Environment Variables

Create a `.env.production` file (never commit this file):

```bash
# Generate secrets
JWT_SECRET=$(openssl rand -base64 32)
MINIO_ACCESS_KEY=$(openssl rand -base64 24 | cut -c1-20)
MINIO_SECRET_KEY=$(openssl rand -base64 32)
TURN_PASSWORD=$(openssl rand -base64 16)
DB_PASSWORD=$(openssl rand -base64 24)
REDIS_PASSWORD=$(openssl rand -base64 24)

# Create .env.production
cat > .env.production <<EOF
# General Configuration
ENV=production
APP_URL=https://secureconnect.com

# JWT Configuration
JWT_SECRET=$JWT_SECRET

# Database Configuration
DB_PASSWORD=$DB_PASSWORD

# Redis Configuration
REDIS_PASSWORD=$REDIS_PASSWORD

# MinIO Configuration
MINIO_ACCESS_KEY=$MINIO_ACCESS_KEY
MINIO_SECRET_KEY=$MINIO_SECRET_KEY

# TURN Configuration
TURN_PASSWORD=$TURN_PASSWORD
EOF

# Add to .gitignore
echo ".env.production" >> .gitignore
```

### Option 3: Using HashiCorp Vault

For enterprise deployments, use HashiCorp Vault:

```bash
# Enable KV secrets engine
vault secrets enable -path=secureconnect kv

# Store secrets
vault kv put secureconnect/jwt secret="$JWT_SECRET"
vault kv put secureconnect/minio access_key="$MINIO_ACCESS_KEY" secret_key="$MINIO_SECRET_KEY"
vault kv put secureconnect/database password="$DB_PASSWORD"
vault kv put secureconnect/redis password="$REDIS_PASSWORD"

# Retrieve secrets in application
vault kv get -field=secret secureconnect/jwt
```

### Option 4: Using AWS Secrets Manager

For AWS deployments:

```bash
# Store JWT secret
aws secretsmanager create-secret \
  --name secureconnect/jwt-secret \
  --secret-string "$(openssl rand -base64 32)" \
  --description "JWT signing secret for SecureConnect"

# Store MinIO credentials
aws secretsmanager create-secret \
  --name secureconnect/minio-credentials \
  --secret-string '{"access_key":"'"$(openssl rand -base64 24 | cut -c1-20)"'","secret_key":"'"$(openssl rand -base64 32)"'"}' \
  --description "MinIO credentials for SecureConnect"

# Retrieve in application
aws secretsmanager get-secret-value \
  --secret-id secureconnect/jwt-secret \
  --query SecretString \
  --output text
```

---

## Firebase Setup

### Step 1: Create Firebase Project

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Click "Add project"
3. Enter project name (e.g., `secureconnect-prod`)
4. Follow the setup wizard

### Step 2: Generate Service Account Key

1. Go to Project Settings (gear icon)
2. Select "Service accounts" tab
3. Click "Generate new private key"
4. Select JSON format
5. Click "Generate"
6. Save the file as `secrets/firebase-adminsdk.json`

### Step 3: Configure Firebase in Application

#### For Docker Secrets:

```bash
# Store Firebase project ID
echo "your-project-id" | docker secret create firebase_project_id -

# Store Firebase credentials
cat secrets/firebase-adminsdk.json | docker secret create firebase_credentials -
```

#### For Environment Variables (Local Development Only):

```bash
# Add to .env.local for local development
cat >> .env.local <<EOF

# Firebase Configuration (Local Development)
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDENTIALS_PATH=./firebase-adminsdk.json
EOF
```

**Note:** For production, always use Docker secrets as shown above. Do not use environment variables for Firebase credentials in production.

### Step 4: Verify Firebase Configuration

```bash
# Test Firebase connection
docker-compose -f docker-compose.production.yml logs video-service | grep Firebase

# Expected output:
# Firebase Admin SDK initialized successfully: project_id=your-project-id
```

---

## Secret Rotation

Rotate secrets regularly to maintain security:

### Rotation Schedule

| Secret | Rotation Frequency | Command |
|--------|-------------------|----------|
| JWT Secret | Every 90 days | `openssl rand -base64 32` |
| Database Password | Every 180 days | `openssl rand -base64 24` |
| Redis Password | Every 180 days | `openssl rand -base64 24` |
| MinIO Credentials | Every 180 days | `openssl rand -base64 24` |
| TURN Password | Every 90 days | `openssl rand -base64 16` |
| Firebase Key | Immediately if compromised | Firebase Console |

### Rotation Procedure

1. **Generate new secret:**
   ```bash
   NEW_SECRET=$(openssl rand -base64 32)
   ```

2. **Update configuration:**
   ```bash
   # For Docker Secrets
   echo "$NEW_SECRET" | docker secret create jwt_secret -
   
   # For Environment Variables
   sed -i 's/JWT_SECRET=.*/JWT_SECRET='"$NEW_SECRET"'/' .env.production
   ```

3. **Restart services:**
   ```bash
   docker-compose -f docker-compose.production.yml up -d
   ```

4. **Verify services:**
   ```bash
   docker-compose -f docker-compose.production.yml logs -f
   ```

5. **Delete old secret:**
   ```bash
   docker secret rm jwt_secret
   ```

---

## Troubleshooting

### Issue: Services fail to start with "JWT_SECRET not set"

**Solution:**
```bash
# Check if secret exists
docker secret ls | grep jwt_secret

# If missing, create it
openssl rand -base64 32 | docker secret create jwt_secret -

# Restart services
docker-compose -f docker-compose.production.yml up -d
```

### Issue: Firebase authentication fails

**Solution:**
```bash
# Verify Firebase credentials file exists
ls -la secrets/firebase-adminsdk.json

# Check file permissions
chmod 600 secrets/firebase-adminsdk.json

# Verify project ID
cat secrets/firebase-adminsdk.json | grep project_id
```

### Issue: MinIO access denied

**Solution:**
```bash
# Regenerate MinIO credentials
NEW_ACCESS_KEY=$(openssl rand -base64 24 | cut -c1-20)
NEW_SECRET_KEY=$(openssl rand -base64 32)

# Update secrets
echo "$NEW_ACCESS_KEY" | docker secret create minio_access_key -
echo "$NEW_SECRET_KEY" | docker secret create minio_secret_key -

# Restart MinIO
docker-compose -f docker-compose.production.yml restart minio
```

### Issue: TURN server authentication fails

**Solution:**
```bash
# Generate new TURN password
NEW_TURN_PASSWORD=$(openssl rand -base64 16)

# Update secret
echo "$NEW_TURN_PASSWORD" | docker secret create turn_password -

# Restart TURN server
docker-compose -f docker-compose.production.yml restart turn
```

---

## Security Best Practices

1. **Never commit secrets to version control**
   - Always use `.gitignore` to exclude `.env` files
   - Use pre-commit hooks to detect secrets

2. **Use strong, randomly generated secrets**
   - Never use default passwords
   - Use `openssl rand` for cryptographically secure secrets

3. **Rotate secrets regularly**
   - Follow the rotation schedule above
   - Rotate immediately if secrets are suspected to be compromised

4. **Use secrets management in production**
   - Docker Secrets for containerized deployments
   - HashiCorp Vault for enterprise
   - AWS Secrets Manager for AWS deployments

5. **Monitor for secret leaks**
   - Use secret scanning tools (TruffleHog, Gitleaks)
   - Enable GitHub secret scanning
   - Regular security audits

---

## Additional Resources

- [Docker Secrets Documentation](https://docs.docker.com/engine/swarm/secrets/)
- [HashiCorp Vault Documentation](https://www.vaultproject.io/docs)
- [AWS Secrets Manager Documentation](https://docs.aws.amazon.com/secretsmanager/)
- [Firebase Admin SDK Documentation](https://firebase.google.com/docs/admin/setup)
- [OpenSSL Documentation](https://www.openssl.org/docs/)

---

**Last Updated:** 2026-01-21
