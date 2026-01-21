# Loki Log Ingestion Validation Report

**Date:** 2026-01-19
**Environment:** Production (docker-compose.production.yml)
**Validator:** Production SRE

---

## Executive Summary

**Status:** ❌ **FAIL**

Loki log ingestion is **NOT WORKING** in the current production deployment. The root cause has been identified and a fix has been documented and partially applied.

**Overall Assessment:**
- Promtail Container: ✅ Running
- Loki Container: ✅ Running
- Log Files: ❌ Not created by services
- Log Ingestion: ❌ No logs received by Loki
- Log Queryability: ❌ Cannot query logs in Grafana

---

## 1. Promtail Container Status

### Check 1.1: Container Running
```bash
docker ps --filter "name=promtail" --format "table {{.Names}}\t{{.Status}}"
```

**Result:** ✅ **PASS**
```
NAMES                    STATUS
secureconnect_promtail   Up 2 minutes
```

### Check 1.2: Promtail Logs Analysis

**Recent Logs (docker logs secureconnect_promtail):**

```
level=info ts=2026-01-19T08:46:02.742266534Z caller=filetargetmanager.go:361 msg="Adding target" key="/var/log/secureconnect/storage-service.log:{job=\"storage-service\", service=\"storage-service\"}"
level=info ts=2026-01-19T08:46:02.742329765Z caller=filetargetmanager.go:361 msg="Adding target" key="/var/log/secureconnect/api-gateway.log:{job=\"api-gateway\", service=\"api-gateway\"}"
level=info ts=2026-01-19T08:46:02.742346477Z caller=filetargetmanager.go:361 msg="Adding target" key="/var/log/secureconnect/auth-service.log:{job=\"auth-service\", service=\"auth-service\"}"
level=info ts=2026-01-19T08:46:02.742359792Z caller=filetargetmanager.go:361 msg="Adding target" key="/var/log/secureconnect/video-service.log:{job=\"video-service\", service=\"video-service\"}"
level=info ts=2026-01-19T08:46:02.742371585Z caller=filetargetmanager.go:361 msg="Adding target" key="/var/log/secureconnect/chat-service.log:{job=\"chat-service\", service=\"chat-service\"}"
```

**Analysis:** ✅ **PASS**
- Promtail successfully loaded configuration
- All 5 service targets are configured
- Promtail is attempting to read log files from `/var/log/secureconnect/`

---

## 2. Log Files Availability Check

### Check 2.1: Log Files in Promtail Container
```bash
docker exec secureconnect_promtail ls -la /var/log/secureconnect/
```

**Result:** ❌ **FAIL**
```
total 4
drwxrwxrwx 1 root root 4096 Jan 13 00:35 .
drwxr-xr-x 1 root root 4096 Jan 16 01:09 ..
-rwxrwxrwx 1 root root  256 Jan 13 00:35 logger.go
```

**Analysis:** ❌ **FAIL**
- No log files exist (`api-gateway.log`, `auth-service.log`, etc.)
- Only a `logger.go` file exists (likely a copy operation artifact)
- Promtail cannot find files to read

### Check 2.2: Service Environment Variables

**API Gateway Environment:**
```bash
docker exec api-gateway printenv
```

**Result:** ❌ **FAIL**
```
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
HOSTNAME=212b11abd4fe
REDIS_HOST=redis
MINIO_ENDPOINT=minio:9000
JWT_SECRET=super-secret-key-please-use-longer-key
DB_HOST=cockroachdb
ENV=production
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
CASSANDRA_HOST=cassandra
HOME=/home/appuser
```

**Analysis:** ❌ **FAIL**
- `LOG_OUTPUT` environment variable is **NOT SET**
- `LOG_FILE_PATH` environment variable is **NOT SET**
- Services are using default logger configuration (output to `stdout`)

---

## 3. Loki Logs Analysis

### Check 3.1: Loki Container Logs

**Recent Logs (docker logs secureconnect_loki):**

```
level=info ts=2026-01-19T08:40:08.499573793Z caller=table_manager.go:171 index-store=boltdb-shipper-2024-01-01 msg="handing over indexes to shipper"
level=info ts=2026-01-19T08:41:08.494568908Z caller=table_manager.go:136 index-store=boltdb-shipper-2024-01-01 msg="uploading tables"
level=info ts=2026-01-19T08:41:08.49462433Z caller=table_manager.go:171 index-store=boltdb-shipper-2024-01-01 msg="handing over indexes to shipper"
level=info ts=2026-01-19T08:42:08.490299964Z caller=table_manager.go:136 index-store=boltdb-shipper-2024-01-01 msg="uploading tables"
...
```

**Analysis:** ❌ **FAIL**
- Loki is running and healthy
- Only internal Loki logs (table management, checkpointing) are visible
- **NO application logs** from api-gateway, auth-service, etc. are being received

---

## 4. Root Cause Analysis

### Primary Issue: Services Write to stdout, Not Files

**Current Behavior:**
1. Services use [`pkg/logger/logger.InitDefault()`](secureconnect-backend/pkg/logger/logger.go:81)
2. Default configuration: `LOG_OUTPUT=stdout` (line 85)
3. Services write logs to Docker stdout, not to files
4. Promtail expects log files at `/var/log/secureconnect/*.log`

**Configuration Mismatch:**

| Component | Expected | Actual |
|------------|----------|--------|
| Services | Write to `/logs/*.log` | Write to stdout |
| Promtail | Read from `/var/log/secureconnect/*.log` | No files exist |
| Volume Mount | `app_logs:/logs` (services) | `app_logs:/var/log/secureconnect` (promtail) |

### Secondary Issue: Environment Variables Not Applied

**Problem:**
- [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml) was modified to add:
  - `LOG_OUTPUT=file`
  - `LOG_FILE_PATH=/logs/<service>.log`
- Containers were created with **OLD** configuration
- New environment variables require container recreation

**Recreation Failed:**
```bash
docker-compose -f docker-compose.production.yml up -d api-gateway auth-service chat-service storage-service
```
**Error:**
```
unsupported external secret minio_access_key
```

**Analysis:**
- External secrets are not properly configured in the environment
- Container recreation requires proper secret setup
- This is a **pre-existing deployment issue**, not related to logging

---

## 5. Configuration Changes Made

### 5.1: Promtail Configuration (FIXED)

**File:** [`configs/promtail-config.yml`](secureconnect-backend/configs/promtail-config.yml:1)

**Changes:**
- Added `__path__` labels to all 5 service jobs
- Configured to read from `/var/log/secureconnect/<service>.log`

**Status:** ✅ **APPLIED**

### 5.2: Docker Compose Monitoring (FIXED)

**File:** [`docker-compose.monitoring.yml`](secureconnect-backend/docker-compose.monitoring.yml:100)

**Changes:**
- Changed volume mount from `./logs` to `app_logs:/var/log/secureconnect:ro`
- This ensures Promtail reads from the same volume as services

**Status:** ✅ **APPLIED**

### 5.3: Docker Compose Production (MODIFIED - NOT APPLIED)

**File:** [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml)

**Changes:**
Added to all services:
```yaml
environment:
  - LOG_OUTPUT=file
  - LOG_FILE_PATH=/logs/<service-name>.log
```

**Status:** ⚠️ **NOT APPLIED**
- Configuration file modified
- Containers need to be recreated to apply changes
- Container recreation blocked by external secrets issue

---

## 6. Validation Results

| Check | Status | Details |
|--------|--------|----------|
| Promtail container running | ✅ PASS | Container is healthy and running |
| Promtail configuration loaded | ✅ PASS | All 5 service targets configured |
| Log files exist | ❌ FAIL | No log files in `/var/log/secureconnect/` |
| Services have LOG_OUTPUT=file | ❌ FAIL | Environment variables not set |
| Loki receiving logs | ❌ FAIL | No application logs visible |
| Grafana queryable | ❌ FAIL | Cannot query logs |

---

## 7. Required Actions

### Immediate Actions Required

1. **Configure External Secrets**
   ```bash
   # Create secrets in Docker
   echo "your-secret-value" | docker secret create minio_access_key -
   echo "your-secret-value" | docker secret create minio_secret_key -
   # ... for all required secrets
   ```

2. **Recreate Services with New Configuration**
   ```bash
   cd secureconnect-backend
   docker-compose -f docker-compose.production.yml up -d --force-recreate
   ```

3. **Verify Log Files Created**
   ```bash
   docker exec api-gateway ls -la /logs/
   # Should see: api-gateway.log
   ```

4. **Verify Promtail Reading Logs**
   ```bash
   docker logs secureconnect_promtail --tail 20
   # Should see: "tail started for file" messages
   ```

5. **Query Loki for Logs**
   ```bash
   curl -G 'http://localhost:3100/loki/api/v1/query_range?query={job="api-gateway"}&limit=10'
   ```

---

## 8. Grafana Validation

### Check 8.1: Grafana Explore Query

**Expected Query:**
```
{job="api-gateway"}
```

**Expected Result:**
- Log entries visible
- Fields: timestamp, level, msg, service, request_id
- JSON format

**Actual Result:** ❌ **FAIL**
- No logs returned
- Empty result set

### Check 8.2: Log Fields Validation

**Expected Fields (from [`promtail-config.yml`](secureconnect-backend/configs/promtail-config.yml:20)):**
- `timestamp` - ISO8601 format
- `level` - log level (info, warn, error)
- `msg` - log message
- `service` - service name
- `request_id` - correlation ID

**Actual Result:** ❌ **CANNOT VALIDATE**
- No logs available to validate fields

---

## 9. Alternative Approaches Considered

### Option A: File-Based Logging (SELECTED)
**Pros:**
- Simple, traditional approach
- No Docker socket required
- Works with current volume setup

**Cons:**
- Requires service restart
- Additional disk I/O
- Log rotation needs separate configuration

### Option B: Docker API Logging (REJECTED)
**Pros:**
- No service changes required
- Uses Docker native logging
- Automatic log rotation

**Cons:**
- Docker socket not available in current setup
- Requires `/var/run/docker.sock` mount
- More complex configuration

### Option C: Stdout + Docker Log Driver (FUTURE)
**Pros:**
- Best practice for containerized apps
- No file management needed
- Automatic log rotation

**Cons:**
- Requires Promtail Docker API configuration
- Docker socket access required
- More complex setup

---

## 10. Final Recommendation

### Status: ❌ **FAIL - ACTION REQUIRED**

**Summary:**
Loki log ingestion is **NOT WORKING** due to a configuration mismatch:
1. Services write logs to stdout (default behavior)
2. Promtail expects log files from `/var/log/secureconnect/*.log`
3. No log files exist

**Root Cause:**
- Services lack `LOG_OUTPUT=file` and `LOG_FILE_PATH` environment variables
- Configuration was modified but containers were not recreated

**Required Fix:**
1. Configure Docker external secrets
2. Recreate services with new environment variables
3. Verify log files are created
4. Verify Promtail reads and forwards logs to Loki

**Estimated Effort:** 30-60 minutes

**Priority:** **HIGH** - Log ingestion is critical for production observability

---

## Appendix: Configuration Files

### A.1: Modified Files

1. [`configs/promtail-config.yml`](secureconnect-backend/configs/promtail-config.yml:1) - Added `__path__` labels
2. [`docker-compose.monitoring.yml`](secureconnect-backend/docker-compose.monitoring.yml:100) - Fixed volume mount
3. [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml) - Added LOG_OUTPUT and LOG_FILE_PATH

### A.2: Logger Configuration

**Default Behavior ([`pkg/logger/logger.go:81`](secureconnect-backend/pkg/logger/logger.go:81)):**
```go
func InitDefault() {
    cfg := &Config{
        Level:    getEnv("LOG_LEVEL", "info"),
        Format:   getEnv("LOG_FORMAT", "json"),
        Output:   getEnv("LOG_OUTPUT", "stdout"),  // <-- Default: stdout
        FilePath: getEnv("LOG_FILE_PATH", "/logs/app.log"),
    }
}
```

**Required Configuration:**
```yaml
environment:
  - LOG_OUTPUT=file
  - LOG_FILE_PATH=/logs/<service-name>.log
```

---

**Report Generated:** 2026-01-19T08:50:00Z
**Validator:** Production SRE
