# PRODUCTION REDEPLOYMENT AUTOMATION PLAN

**Document Version:** 1.0  
**Date:** 2026-01-28  
**Role:** Production Automation Controller (Senior SRE)  
**Environment:** Docker Desktop (Production-like)  
**Target:** Production deployment using [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml)

---

## EXECUTION OVERVIEW

This document provides a strict, step-by-step automation plan for production redeployment. Each step includes exact commands, validation criteria, and stop conditions.

**STOP CONDITION:** If any step fails, STOP execution and do not proceed to the next step until the failure is resolved.

---

## STEP 1 – FULL SYSTEM SHUTDOWN

### Purpose
Gracefully stop all running containers and clean up existing resources to ensure a clean deployment state.

### Commands to Run

```bash
# Navigate to backend directory
cd d:/secureconnect/secureconnect-backend

# Stop all containers and remove volumes (WARNING: This will delete data!)
docker-compose down -v

# Verify all containers are stopped
docker ps -a
```

### Expected Output
- `docker-compose down -v` should show containers being stopped and removed
- `docker ps -a` should show no containers running (or only containers from other projects)

### How to Verify Success
```bash
# Check that no SecureConnect containers are running
docker ps --filter "name=secureconnect" --format "{{.Names}}"
# Expected: Empty output (no containers found)

# Check that no SecureConnect containers exist (even stopped)
docker ps -a --filter "name=secureconnect" --format "{{.Names}}"
# Expected: Empty output (no containers found)
```

### What Constitutes a Failure
1. Command fails with error
2. Containers remain running after shutdown
3. Docker daemon is not responding
4. Permission denied errors

### What to Do if Failure Occurs
1. Check Docker Desktop status - ensure it is running
2. Try manual stop: `docker stop $(docker ps -aq)`
3. Try manual remove: `docker rm $(docker ps -aq)`
4. If Docker daemon issues, restart Docker Desktop
5. **DO NOT PROCEED** until all containers are stopped

---

## STEP 2 – DOCKER SECRETS EXISTENCE VERIFICATION

### Purpose
Verify that all required Docker secrets files exist in the [`secrets/`](secureconnect-backend/secrets) directory before deployment.

### Commands to Run

```bash
# Navigate to backend directory
cd d:/secureconnect/secureconnect-backend

# List all secret files
dir secrets
```

### Expected Output
All of the following files must exist:
```
cassandra_password.txt
cassandra_user.txt
db_password.txt
firebase_credentials.json
firebase_project_id.txt
grafana_admin_password.txt
jwt_secret.txt
minio_access_key.txt
minio_secret_key.txt
redis_password.txt
smtp_password.txt
smtp_username.txt
turn_password.txt
turn_user.txt
```

### How to Verify Success

```bash
# Count the number of secret files (Windows PowerShell)
powershell -Command "(Get-ChildItem -Path secrets -File).Count"
# Expected: 14 files

# Verify each file is not empty (Windows PowerShell)
powershell -Command "Get-ChildItem -Path secrets -File | Where-Object { $_.Length -eq 0 }"
# Expected: Empty output (no empty files)
```

### What Constitutes a Failure
1. Any secret file is missing
2. Any secret file is empty (0 bytes)
3. Secrets directory does not exist
4. Secret files have incorrect permissions

### What to Do if Failure Occurs
1. If files are missing, run: `powershell -File scripts/generate-secret-files.ps1`
2. If directory is missing, create it: `mkdir secrets`
3. Verify file permissions: `icacls secrets`
4. **DO NOT PROCEED** until all 14 secret files exist and are non-empty

---

## STEP 3 – PRODUCTION DEPLOYMENT USING DOCKER-COMPOSE.PRODUCTION.YML

### Purpose
Deploy the system using the production configuration file with Docker secrets properly configured.

### Commands to Run

```bash
# Navigate to backend directory
cd d:/secureconnect/secureconnect-backend

# Deploy with production configuration
docker-compose -f docker-compose.production.yml up -d

# Wait for containers to start (approximately 60 seconds)
timeout /t 60 /nobreak

# Check container status
docker ps
```

### Expected Output
- `docker-compose up -d` should show containers being created and started
- `docker ps` should show all containers in "Up" status
- Expected containers:
  - secureconnect_crdb
  - secureconnect_cassandra
  - secureconnect_redis
  - secureconnect_minio
  - api-gateway
  - auth-service
  - chat-service
  - video-service
  - storage-service
  - secureconnect_turn
  - secureconnect_prometheus
  - secureconnect_alertmanager
  - secureconnect_grafana
  - secureconnect_loki
  - secureconnect_promtail
  - secureconnect_backup
  - secureconnect_nginx

### How to Verify Success

```bash
# Verify all containers are running
docker ps --filter "name=secureconnect" --format "table {{.Names}}\t{{.Status}}"
# Expected: All containers showing "Up" status

# Verify production compose file was used
docker inspect api-gateway --format "{{index .Config.Labels \"com.docker.compose.project.config_files\"}}"
# Expected: Contains "docker-compose.production.yml"

# Wait for health checks (60 seconds)
timeout /t 60 /nobreak

# Check health status
docker ps --format "table {{.Names}}\t{{.Status}}"
```

### What Constitutes a Failure
1. Any container fails to start
2. Containers start but immediately exit (Exit status)
3. Containers show "Restarting" status continuously
4. Production compose file was not used
5. Docker Compose errors during deployment

### What to Do if Failure Occurs
1. Check logs: `docker-compose -f docker-compose.production.yml logs`
2. Check specific container logs: `docker logs <container-name>`
3. Verify secrets are accessible: `docker-compose -f docker-compose.production.yml config`
4. Check port conflicts: `netstat -ano | findstr "LISTENING"`
5. **DO NOT PROCEED** until all containers are running and healthy

---

## STEP 4 – SECRETS INJECTION VALIDATION (NO PLAINTEXT SECRETS ALLOWED)

### Purpose
Verify that Docker secrets are properly mounted and no plaintext secrets exist in container environment variables.

### Commands to Run

```bash
# Check that secrets directory is mounted in containers
docker exec api-gateway ls -la /run/secrets/

# Check environment variables for plaintext secrets (SHOULD BE EMPTY)
docker exec api-gateway env | findstr /I "SECRET PASSWORD KEY" | findstr /V "_FILE"

# Verify _FILE variables are set (SHOULD EXIST)
docker exec api-gateway env | findstr "_FILE"
```

### Expected Output

**Secrets directory listing:**
```
total XX
drwxr-xr-x    2 root     root          XXX Jan XX XX:XX .
drwxr-xr-x    1 root     root          XXX Jan XX XX:XX ..
-r--r--r--    1 root     root          XX Jan XX XX:XX cassandra_password
-r--r--r--    1 root     root          XX Jan XX XX:XX cassandra_user
-r--r--r--    1 root     root          XX Jan XX XX:XX db_password
-r--r--r--    1 root     root          XX Jan XX XX:XX firebase_credentials.json
-r--r--r--    1 root     root          XX Jan XX XX:XX firebase_project_id
-r--r--r--    1 root     root          XX Jan XX XX:XX grafana_admin_password
-r--r--r--    1 root     root          XX Jan XX XX:XX jwt_secret
-r--r--r--    1 root     root          XX Jan XX XX:XX minio_access_key
-r--r--r--    1 root     root          XX Jan XX XX:XX minio_secret_key
-r--r--r--    1 root     root          XX Jan XX XX:XX redis_password
-r--r--r--    1 root     root          XX Jan XX XX:XX smtp_password
-r--r--r--    1 root     root          XX Jan XX XX:XX smtp_username
-r--r--r--    1 root     root          XX Jan XX XX:XX turn_password
-r--r--r--    1 root     root          XX Jan XX XX:XX turn_user
```

**Plaintext secrets check (SHOULD BE EMPTY):**
```
<No output>
```

**_FILE variables check (SHOULD EXIST):**
```
REDIS_PASSWORD_FILE=/run/secrets/redis_password
MINIO_ACCESS_KEY_FILE=/run/secrets/minio_access_key
MINIO_SECRET_KEY_FILE=/run/secrets/minio_secret_key
CASSANDRA_USER_FILE=/run/secrets/cassandra_user
CASSANDRA_PASSWORD_FILE=/run/secrets/cassandra_password
JWT_SECRET_FILE=/run/secrets/jwt_secret
SMTP_USERNAME_FILE=/run/secrets/smtp_username
SMTP_PASSWORD_FILE=/run/secrets/smtp_password
```

### How to Verify Success

```bash
# Verify no plaintext secrets in api-gateway
docker exec api-gateway powershell -Command "Get-ChildItem Env: | Where-Object { $_.Name -match 'SECRET|PASSWORD|KEY' -and $_.Name -notmatch '_FILE' }"
# Expected: Empty output

# Verify no plaintext secrets in auth-service
docker exec auth-service powershell -Command "Get-ChildItem Env: | Where-Object { $_.Name -match 'SECRET|PASSWORD|KEY' -and $_.Name -notmatch '_FILE' }"
# Expected: Empty output

# Verify no plaintext secrets in chat-service
docker exec chat-service powershell -Command "Get-ChildItem Env: | Where-Object { $_.Name -match 'SECRET|PASSWORD|KEY' -and $_.Name -notmatch '_FILE' }"
# Expected: Empty output

# Verify secrets are readable from files
docker exec api-gateway cat /run/secrets/jwt_secret
# Expected: Secret value (not empty)
```

### What Constitutes a Failure
1. Secrets directory `/run/secrets/` does not exist in containers
2. Any plaintext secret found in environment variables (e.g., `JWT_SECRET=value`)
3. `_FILE` environment variables are not set
4. Secrets files are not readable from containers
5. Secrets are exposed in `docker inspect` output

### What to Do if Failure Occurs
1. **CRITICAL FAILURE** - This is a security issue
2. Verify [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:212-240) has correct `secrets:` sections
3. Verify secrets files exist in [`secrets/`](secureconnect-backend/secrets) directory
4. Restart deployment: `docker-compose -f docker-compose.production.yml down && docker-compose -f docker-compose.production.yml up -d`
5. **DO NOT PROCEED** until no plaintext secrets are found

---

## STEP 5 – PRODUCTION MODE VERIFICATION AND FINAL SANITY CHECKS

### Purpose
Verify that the system is running in production mode and all services are healthy and functional.

### Commands to Run

```bash
# Verify production environment variable
docker exec api-gateway env | findstr "ENV"
docker exec auth-service env | findstr "ENV"
docker exec chat-service env | findstr "ENV"

# Check all container health status
docker ps --format "table {{.Names}}\t{{.Status}}"

# Test service health endpoints
curl -s -o /dev/null -w "api-gateway: %{http_code}\n" http://localhost:8080/health
curl -s -o /dev/null -w "chat-service: %{http_code}\n" http://localhost:8082/health
curl -s -o /dev/null -w "storage-service: %{http_code}\n" http://localhost:8084/health
curl -s -o /dev/null -w "auth-service: %{http_code}\n" http://localhost:8081/health

# Test metrics endpoints
curl -s -o /dev/null -w "api-gateway metrics: %{http_code}\n" http://localhost:8080/metrics
curl -s -o /dev/null -w "chat-service metrics: %{http_code}\n" http://localhost:8082/metrics

# Test monitoring stack
curl -s -o /dev/null -w "Prometheus: %{http_code}\n" http://localhost:9091/-/healthy
curl -s -o /dev/null -w "Grafana: %{http_code}\n" http://localhost:3000/api/health
curl -s -o /dev/null -w "Alertmanager: %{http_code}\n" http://localhost:9093/-/healthy

# Check resource usage
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"
```

### Expected Output

**Environment variables:**
```
ENV=production
```

**Container health status:**
- All containers should show "Up" status
- Services with health checks should show "(healthy)" status:
  - secureconnect_crdb
  - secureconnect_cassandra
  - secureconnect_minio
  - secureconnect_turn
  - storage-service
  - secureconnect_prometheus
  - secureconnect_alertmanager
  - secureconnect_grafana
  - secureconnect_promtail

**Health endpoints:**
```
api-gateway: 200
chat-service: 200
storage-service: 200
auth-service: 200
api-gateway metrics: 200
chat-service metrics: 200
Prometheus: 200
Grafana: 200
Alertmanager: 200
```

**Resource usage:**
- All containers within memory limits
- CPU usage reasonable (< 50% for most services)
- No containers using excessive resources

### How to Verify Success

```bash
# Verify production mode is set
docker exec api-gateway env | findstr "ENV=production"
# Expected: ENV=production

# Verify all services are healthy
docker ps --filter "status=running" --filter "name=secureconnect" --format "{{.Names}}"
# Expected: All 17 containers listed

# Verify no containers are restarting
docker ps --filter "status=restarting" --format "{{.Names}}"
# Expected: Empty output

# Verify no containers have exited
docker ps --filter "status=exited" --format "{{.Names}}"
# Expected: Empty output

# Check logs for errors (last 50 lines)
docker logs api-gateway --tail 50 | findstr /I "error panic fatal"
# Expected: No critical errors

docker logs chat-service --tail 50 | findstr /I "error panic fatal"
# Expected: No critical errors

docker logs storage-service --tail 50 | findstr /I "error panic fatal"
# Expected: No critical errors
```

### What Constitutes a Failure
1. Any service shows `ENV=local` or `ENV=development`
2. Any service health endpoint returns non-200 status
3. Any container is in "Restarting" or "Exited" status
4. Critical errors (panic, fatal) in service logs
5. Metrics endpoints not accessible
6. Monitoring stack not accessible
7. Resource usage exceeds limits

### What to Do if Failure Occurs
1. Check service logs: `docker logs <service-name>`
2. Verify environment variables: `docker exec <service-name> env`
3. Check dependencies: `docker ps` (ensure all databases are running)
4. Restart failed service: `docker restart <service-name>`
5. If multiple services failing, consider full redeployment
6. **DO NOT PROCEED** until all checks pass

---

## FINAL VERDICT

### GO / NO-GO Decision Criteria

**GO** (All of the following must be true):
- ✅ All containers started successfully
- ✅ All containers in "Up" status
- ✅ All services showing `ENV=production`
- ✅ No plaintext secrets in environment variables
- ✅ All secrets mounted at `/run/secrets/`
- ✅ All health endpoints returning 200 OK
- ✅ All metrics endpoints returning 200 OK
- ✅ Monitoring stack accessible (Prometheus, Grafana, Alertmanager)
- ✅ No containers restarting or exited
- ✅ No critical errors in logs
- ✅ Resource usage within limits

**NO-GO** (Any of the following):
- ❌ Any container failed to start
- ❌ Any container in "Restarting" or "Exited" status
- ❌ Any service showing `ENV=local` or `ENV=development`
- ❌ Any plaintext secret found in environment variables
- ❌ Secrets not mounted at `/run/secrets/`
- ❌ Any health endpoint returning non-200 status
- ❌ Any metrics endpoint returning non-200 status
- ❌ Monitoring stack not accessible
- ❌ Critical errors (panic, fatal) in logs
- ❌ Resource usage exceeding limits

---

## PRODUCTION READINESS STATEMENT

**This deployment automation plan provides:**

1. **Deterministic execution** - Each step has exact commands and expected outputs
2. **Clear validation** - Success/failure criteria are unambiguous
3. **Stop conditions** - Execution stops on any failure
4. **Security validation** - Explicit checks for plaintext secrets
5. **Production verification** - Confirms production mode is active
6. **Comprehensive testing** - Health, metrics, monitoring, and resource checks

**Any uncertainty results in NO-GO.**

---

## QUICK REFERENCE COMMANDS

### Full Deployment (One Command)
```bash
cd d:/secureconnect/secureconnect-backend && docker-compose -f docker-compose.production.yml down && docker-compose -f docker-compose.production.yml up -d
```

### Quick Health Check
```bash
docker ps --format "table {{.Names}}\t{{.Status}}"
```

### Secrets Verification
```bash
docker exec api-gateway env | findstr /I "SECRET PASSWORD KEY" | findstr /V "_FILE"
```

### Production Mode Verification
```bash
docker exec api-gateway env | findstr "ENV"
```

### Logs Check
```bash
docker-compose -f docker-compose.production.yml logs --tail 50
```

---

**Document Status:** Ready for Execution  
**Next Action:** Execute Step 1
