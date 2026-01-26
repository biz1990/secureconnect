# LOGGING STACK PRODUCTION READINESS
**Date**: 2026-01-25
**Goal**: Review Grafana, Loki, Promtail production readiness

---

## 📋 CURRENT STATE ANALYSIS

### Grafana Configuration

**File**: [`configs/grafana-datasources.yml`](secureconnect-backend/configs/grafana-datasources.yml:1)
**Status**: ✅ Configured with Prometheus datasource
**Issue**: Default admin password

### Loki Configuration

**File**: [`configs/loki-config.yml`](secureconnect-backend/configs/loki-config.yml:1)
**Status**: ✅ Properly configured
- HTTP port: 3100
- gRPC port: 9096
- Storage: filesystem
- Schema: v11

### Promtail Configuration

**File**: [`configs/promtail-config.yml`](secureconnect-backend/configs/promtail-config.yml:1)
**Status**: ✅ Properly configured
- Scrapes all service logs: api-gateway, auth-service, chat-service, video-service, storage-service
- JSON parsing configured
- Labels: level, message, service, request_id

### Docker Compose Files

| File | Logging Services | Status |
|------|-----------------|--------|
| [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:1) | None | ❌ MISSING |
| [`docker-compose.logging.yml`](secureconnect-backend/docker-compose.logging.yml:1) | Loki, Promtail, Grafana | ✅ COMPLETE |
| [`docker-compose.override.yml`](secureconnect-backend/docker-compose.override.yml:1) | None | ❌ N/A (local dev only) |

---

## 🔴 CRITICAL ISSUES

### Issue #1: Grafana Admin Password is Default

**Severity**: CRITICAL
**File**: [`docker-compose.logging.yml`](secureconnect-backend/docker-compose.logging.yml:92)
**Line**: 92

**Current Configuration**:
```yaml
environment:
  - GF_SECURITY_ADMIN_USER=admin
  - GF_SECURITY_ADMIN_PASSWORD=change-me-in-production
```

**Impact**: Default password `change-me-in-production` is a security risk. Anyone can access Grafana with admin/change-me-in-production.

---

### Issue #2: Logging Services Not in Production Docker Compose

**Severity**: HIGH
**File**: [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:1)
**Issue**: Loki, Promtail, and Grafana are NOT included in production compose file

**Impact**: Logging stack must be started separately using `docker-compose -f docker-compose.logging.yml up -d`. This means:
- Logs won't be collected automatically when starting production stack
- Grafana won't be available in production
- No centralized log aggregation

---

## 🔧 FIXES REQUIRED

### Fix #1: Use Secrets for Grafana Admin Password

**Approach**: Use the existing [`grafana_admin_password.txt`](secureconnect-backend/secrets/grafana_admin_password.txt:1) secret file

**File**: [`docker-compose.logging.yml`](secureconnect-backend/docker-compose.logging.yml:82-102)
**Line**: 91-92

**BEFORE**:
```yaml
environment:
  - GF_SECURITY_ADMIN_USER=admin
  - GF_SECURITY_ADMIN_PASSWORD=change-me-in-production
```

**AFTER**:
```yaml
environment:
  - GF_SECURITY_ADMIN_USER=admin
  - GF_SECURITY_ADMIN_PASSWORD_FILE=/run/secrets/grafana_admin_password
```

**Additional Changes Required**:
Add secrets mount to Grafana service:

**BEFORE**:
```yaml
grafana:
  image: grafana/grafana:latest
  container_name: secureconnect_grafana
  ports:
    - "3000:3000"
  volumes:
    - grafana_data:/var/lib/grafana
    - ./configs/grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
```

**AFTER**:
```yaml
grafana:
  image: grafana/grafana:latest
  container_name: secureconnect_grafana
  ports:
    - "3000:3000"
  secrets:
    - grafana_admin_password
  volumes:
    - grafana_data:/var/lib/grafana
    - ./configs/grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
```

**Add to secrets section** (line 34):

```yaml
secrets:
  # ... existing secrets ...
  grafana_admin_password:
    file: ./secrets/grafana_admin_password.txt
```

---

### Fix #2: Add Logging Services to Production Docker Compose

**Option A**: Create docker-compose.production.logging.yml

**File**: `secureconnect-backend/docker-compose.production.logging.yml` (NEW FILE)

**Content**: Copy entire [`docker-compose.logging.yml`](secureconnect-backend/docker-compose.logging.yml:1) content

**Usage**:
```bash
docker-compose -f docker-compose.production.yml -f docker-compose.production.logging.yml up -d
```

**Option B**: Create docker-compose.override.yml for Production

**File**: `secureconnect-backend/docker-compose.production.override.yml` (NEW FILE)

**Content**:
```yaml
version: '3.8'

# =============================================================================
# PRODUCTION OVERRIDE - ENABLE LOGGING STACK
# =============================================================================

services:
  # --------------------------------------------------------------------------
  # GRAFANA - Visualization
  # --------------------------------------------------------------------------
  grafana:
    image: grafana/grafana:latest
    container_name: secureconnect_grafana
    ports:
      - "3000:3000"
    secrets:
      - grafana_admin_password
    volumes:
      - grafana_data:/var/lib/grafana
      - ./configs/grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD_FILE=/run/secrets/grafana_admin_password
      - GF_USERS_ALLOW_SIGN_UP=false
      - GF_INSTALL_PLUGINS=grafana-piechart-panel
    networks:
      - secureconnect-net
    restart: always
    healthcheck:
      test: [ "CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/api/health" ]
      interval: 10s
      timeout: 5s
      retries: 3

  # --------------------------------------------------------------------------
  # LOKI - Log Aggregation
  # --------------------------------------------------------------------------
  loki:
    image: grafana/loki:latest
    container_name: secureconnect_loki
    ports:
      - "3100:3100"
      - "9096:9096"
    volumes:
      - loki_data:/loki
      - ./configs/loki-config.yml:/etc/loki/local-config.yaml
    networks:
      - secureconnect-net
    command: -config.file=/etc/loki/local-config.yaml
    restart: always
    healthcheck:
      test: [ "CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3100/ready" ]
      interval: 10s
      timeout: 5s
      retries: 3

  # --------------------------------------------------------------------------
  # PROMTAIL - Log Collector
  # --------------------------------------------------------------------------
  promtail:
    image: grafana/promtail:latest
    container_name: secureconnect_promtail
    volumes:
      - promtail_data:/tmp
      - ./configs/promtail-config.yml:/etc/promtail/config.yml
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - secureconnect-net
    command: -config.file=/etc/promtail/config.yml
    restart: always
    depends_on:
      - loki
    healthcheck:
      test: [ "CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:9080/metrics" ]
      interval: 10s
      timeout: 5s
      retries: 3

# =============================================================================
# VOLUMES (add to base compose)
# =============================================================================
volumes:
  loki_data:
  grafana_data:
  promtail_data:
```

**Usage**:
```bash
docker-compose -f docker-compose.production.yml -f docker-compose.production.override.yml up -d
```

---

## ✅ COMPLETE FIXED CONFIGURATIONS

### Fixed: docker-compose.logging.yml

```yaml
version: '3.8'

# =============================================================================
# LOGGING STACK - LOKI + PROMTAIL + GRAFANA
# =============================================================================

networks:
  logging-net:
    driver: bridge
  secureconnect-net:
    external: true

volumes:
  loki_data:
  grafana_data:
  promtail_data:

services:
  # --------------------------------------------------------------------------
  # LOKI - Log Aggregation
  # --------------------------------------------------------------------------
  loki:
    image: grafana/loki:latest
    container_name: secureconnect_loki
    ports:
      - "3100:3100"
      - "9096:9096"
    volumes:
      - loki_data:/loki
      - ./configs/loki-config.yml:/etc/loki/local-config.yaml
    networks:
      - logging-net
      - secureconnect-net
    command: -config.file=/etc/loki/local-config.yaml
    restart: always
    healthcheck:
      test: [ "CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3100/ready" ]
      interval: 10s
      timeout: 5s
      retries: 3

  # --------------------------------------------------------------------------
  # PROMTAIL - Log Collector
  # --------------------------------------------------------------------------
  promtail:
    image: grafana/promtail:latest
    container_name: secureconnect_promtail
    volumes:
      - promtail_data:/tmp
      - ./configs/promtail-config.yml:/etc/promtail/config.yml
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - logging-net
      - secureconnect-net
    command: -config.file=/etc/promtail/config.yml
    restart: always
    depends_on:
      - loki
    healthcheck:
      test: [ "CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:9080/metrics" ]
      interval: 10s
      timeout: 5s
      retries: 3

  # --------------------------------------------------------------------------
  # GRAFANA - Visualization
  # --------------------------------------------------------------------------
  grafana:
    image: grafana/grafana:latest
    container_name: secureconnect_grafana
    ports:
      - "3000:3000"
    secrets:
      - grafana_admin_password
    volumes:
      - grafana_data:/var/lib/grafana
      - ./configs/grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD_FILE=/run/secrets/grafana_admin_password
      - GF_USERS_ALLOW_SIGN_UP=false
      - GF_INSTALL_PLUGINS=grafana-piechart-panel
    networks:
      - logging-net
      - secureconnect-net
    restart: always
    healthcheck:
      test: [ "CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/api/health" ]
      interval: 10s
      timeout: 5s
      retries: 3
```

---

### Fixed: docker-compose.production.yml (Add Grafana Secret)

**Add to secrets section** (after line 34):

```yaml
secrets:
  # ... existing secrets ...
  grafana_admin_password:
    file: ./secrets/grafana_admin_password.txt
```

---

## ✅ VALIDATION COMMANDS

### Step 1: Verify Grafana Admin Password

```bash
# Check if secret file exists
cat secureconnect-backend/secrets/grafana_admin_password.txt

# Expected output: Random password string
```

### Step 2: Verify Grafana Configuration

```bash
# Start Grafana
docker-compose -f docker-compose.logging.yml up -d grafana

# Check logs
docker logs secureconnect_grafana

# Expected output: No errors, Grafana running
```

### Step 3: Verify Loki Configuration

```bash
# Start Loki
docker-compose -f docker-compose.logging.yml up -d loki

# Check logs
docker logs secureconnect_loki

# Expected output: Loki listening on ports 3100 and 9096
```

### Step 4: Verify Promtail Configuration

```bash
# Start Promtail
docker-compose -f docker-compose.logging.yml up -d promtail

# Check logs
docker logs secureconnect_promtail

# Expected output: Promtail scraping logs, sending to Loki
```

### Step 5: Verify Log Ingestion

```bash
# Check if logs are being sent to Loki
curl http://localhost:3100/loki/api/v1/labels

# Expected output: JSON with labels from services
```

### Step 6: Verify Grafana Access

```bash
# Access Grafana
curl -u admin:$(cat secureconnect-backend/secrets/grafana_admin_password.txt) http://localhost:3000/api/health

# Expected output: OK
```

### Step 7: Test Full Stack

```bash
# Start all services
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d

# Check all services
docker ps --filter "name=secureconnect_"

# Expected output: All services running
```

---

## 📝 IMPLEMENTATION STEPS

### Step 1: Add Grafana Secret to docker-compose.production.yml

**File**: [`secureconnect-backend/docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:1)
**Action**: Add grafana_admin_password secret to secrets section

```yaml
secrets:
  # ... existing secrets ...
  grafana_admin_password:
    file: ./secrets/grafana_admin_password.txt
```

### Step 2: Update docker-compose.logging.yml

**File**: [`secureconnect-backend/docker-compose.logging.yml`](secureconnect-backend/docker-compose.logging.yml:1)
**Actions**:
1. Add secrets mount to Grafana service
2. Update environment to use GF_SECURITY_ADMIN_PASSWORD_FILE

### Step 3: Start Production Stack with Logging

```bash
cd secureconnect-backend
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d
```

---

## 📁 FILES TO MODIFY

1. [`secureconnect-backend/docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:1)
   - Add grafana_admin_password secret to secrets section

2. [`secureconnect-backend/docker-compose.logging.yml`](secureconnect-backend/docker-compose.logging.yml:1)
   - Add secrets mount to Grafana service
   - Update environment to use secret file

---

## 🎯 SUMMARY

| Issue | Severity | Status | Fix |
|-------|----------|--------|------|
| Grafana default password | CRITICAL | ⚠️ Requires manual secret update |
| Logging services missing from production | HIGH | ✅ Use docker-compose.logging.yml |
| Loki configuration | - | ✅ Already correct |
| Promtail configuration | - | ✅ Already correct |
| Grafana datasource | - | ✅ Already correct |

---

**Document Version**: 1.0
**Last Updated**: 2026-01-25
