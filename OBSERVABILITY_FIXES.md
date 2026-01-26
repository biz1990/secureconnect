# OBSERVABILITY CONFIGURATION FIXES
**Date**: 2026-01-25
**Goal**: Fix observability WITHOUT changing business logic

---

## 📋 MISCONFIGURATIONS IDENTIFIED

### CRITICAL ISSUES

| # | Severity | File | Line | Issue | Impact |
|---|----------|-------|-------|--------|
| 1 | CRITICAL | `configs/prometheus.yml` | 12 | Alertmanager targets empty (`targets: []`) | Alerts won't be sent |
| 2 | CRITICAL | `configs/prometheus.yml` | 16 | Alert rules file commented out | No alert rules loaded |
| 3 | HIGH | `configs/prometheus.yml` | 30 | Auth service scrape port incorrect | Metrics not collected |
| 4 | HIGH | `configs/prometheus.yml` | 51 | Storage service scrape port incorrect | Metrics not collected |
| 5 | MEDIUM | `configs/alertmanager.yml` | 22-31 | Receivers have no-op configuration | No notifications sent |

---

## 🔧 FIXES REQUIRED

### Fix #1: Enable Alert Rules in Prometheus

**File**: [`secureconnect-backend/configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:1)
**Line**: 16

**BEFORE**:
```yaml
# Rule files (optional)
rule_files:
  # - "alerts.yml"
```

**AFTER**:
```yaml
# Rule files (optional)
rule_files:
  - "/etc/prometheus/alerts.yml"
```

**Rationale**: The alerts rule file is commented out, so no alert rules are loaded. This must be uncommented to enable alerting.

---

### Fix #2: Configure Alertmanager Target in Prometheus

**File**: [`secureconnect-backend/configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:1)
**Line**: 12

**BEFORE**:
```yaml
# Alertmanager configuration (optional)
alerting:
  alertmanagers:
    - static_configs:
        - targets: []
```

**AFTER**:
```yaml
# Alertmanager configuration (optional)
alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']
```

**Rationale**: Alertmanager targets are empty, so Prometheus won't send alerts to Alertmanager. Must specify the Alertmanager service name and port.

---

### Fix #3: Fix Auth Service Scrape Port

**File**: [`secureconnect-backend/configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:1)
**Line**: 30

**BEFORE**:
```yaml
  # Auth Service
  - job_name: 'auth-service'
    static_configs:
      - targets: ['auth-service:8080']
    metrics_path: '/metrics'
    scrape_interval: 10s
```

**AFTER**:
```yaml
  # Auth Service
  - job_name: 'auth-service'
    static_configs:
      - targets: ['auth-service:8081']
    metrics_path: '/metrics'
    scrape_interval: 10s
```

**Rationale**: Docker compose maps auth-service to port 8081 (line269), but Prometheus is scraping port 8080. This will cause metrics collection to fail.

---

### Fix #4: Fix Storage Service Scrape Port

**File**: [`secureconnect-backend/configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:1)
**Line**: 51

**BEFORE**:
```yaml
  # Storage Service
  - job_name: 'storage-service'
    static_configs:
      - targets: ['storage-service:8080']
    metrics_path: '/metrics'
    scrape_interval: 10s
```

**AFTER**:
```yaml
  # Storage Service
  - job_name: 'storage-service'
    static_configs:
      - targets: ['storage-service:8084']
    metrics_path: '/metrics'
    scrape_interval: 10s
```

**Rationale**: Docker compose maps storage-service to port 8084 (line421), but Prometheus is scraping port 8080. This will cause metrics collection to fail.

---

### Fix #5: Configure Alertmanager Receivers (Optional but Recommended)

**File**: [`secureconnect-backend/configs/alertmanager.yml`](secureconnect-backend/configs/alertmanager.yml:1)
**Line**: 22-31

**BEFORE**:
```yaml
# Receivers
receivers:
  - name: 'default'
    # Default receiver (no-op)

  - name: 'critical-alerts'
    # Email or webhook configuration can be added here
    # For local development, using a no-op receiver

  - name: 'warning-alerts'
    # Email or webhook configuration can be added here
    # For local development, using a no-op receiver
```

**AFTER** (Example - Email):
```yaml
# Receivers
receivers:
  - name: 'default'
    # Default receiver (no-op)

  - name: 'critical-alerts'
    email_configs:
      - to: 'oncall@example.com'
        from: 'alertmanager@secureconnect.com'
        smarthost: 'smtp.gmail.com'
        auth_username: 'your-smtp-username'
        auth_password: 'your-smtp-password'

  - name: 'warning-alerts'
    email_configs:
      - to: 'devops@example.com'
        from: 'alertmanager@secureconnect.com'
        smarthost: 'smtp.gmail.com'
        auth_username: 'your-smtp-username'
        auth_password: 'your-smtp-password'
```

**Rationale**: Receivers have no-op configuration, so even if alerts fire, no notifications will be sent. This is optional but recommended for production.

---

## 📁 FILE MOUNTING REQUIRED

### Mount alerts.yml in Docker Compose

**File**: [`secureconnect-backend/docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:1)
**Line**: 498-499 (Prometheus volumes section)

**BEFORE**:
```yaml
prometheus:
  image: prom/prometheus:v2.48.0
  container_name: secureconnect_prometheus
  ports:
    - "9091:9090"
  volumes:
    - prometheus_data:/prometheus
    - ./configs/prometheus.yml:/etc/prometheus/prometheus.yml:ro
```

**AFTER**:
```yaml
prometheus:
  image: prom/prometheus:v2.48.0
  container_name: secureconnect_prometheus
  ports:
    - "9091:9090"
  volumes:
    - prometheus_data:/prometheus
    - ./configs/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    - ./configs/alerts.yml:/etc/prometheus/alerts.yml:ro
```

**Rationale**: The alerts.yml file must be mounted into the Prometheus container for it to be loaded.

---

## 🔄 COMPLETE FIXED CONFIGURATIONS

### Fixed: prometheus.yml

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    monitor: 'secureconnect-monitor'
    environment: 'production'

# Alertmanager configuration (optional)
alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']

# Rule files (optional)
rule_files:
  - "/etc/prometheus/alerts.yml"

# Scrape configurations
scrape_configs:
  # API Gateway
  - job_name: 'api-gateway'
    static_configs:
      - targets: ['api-gateway:8080']
    metrics_path: '/metrics'
    scrape_interval: 10s

  # Auth Service
  - job_name: 'auth-service'
    static_configs:
      - targets: ['auth-service:8081']
    metrics_path: '/metrics'
    scrape_interval: 10s

  # Chat Service
  - job_name: 'chat-service'
    static_configs:
      - targets: ['chat-service:8082']
    metrics_path: '/metrics'
    scrape_interval: 10s

  # Video Service
  - job_name: 'video-service'
    static_configs:
      - targets: ['video-service:8083']
    metrics_path: '/metrics'
    scrape_interval: 10s

  # Storage Service
  - job_name: 'storage-service'
    static_configs:
      - targets: ['storage-service:8084']
    metrics_path: '/metrics'
    scrape_interval: 10s

  # Prometheus itself
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
```

---

## ✅ VERIFICATION STEPS

### Step 1: Verify Prometheus Configuration

```bash
# Check if Prometheus config is valid
docker exec secureconnect_prometheus promtool check config /etc/prometheus/prometheus.yml

# Expected output: SUCCESS
```

### Step 2: Verify Alert Rules Are Loaded

```bash
# Check if alert rules are loaded
curl http://localhost:9091/api/v1/rules

# Expected output: JSON with alert rules from alerts.yml
```

### Step 3: Verify Alertmanager Target

```bash
# Check if Alertmanager is configured as target
curl http://localhost:9091/api/v1/targets

# Expected output: alertmanager:9093 should be in targets list
```

### Step 4: Verify Service Metrics Are Being Scraped

```bash
# Check if all services are up
curl http://localhost:9091/api/v1/targets

# Expected output: All services should show "health": "up"
```

### Step 5: Verify Alerts Can Fire

```bash
# Stop a service to trigger ServiceDown alert
docker stop auth-service

# Wait 30 seconds for alert to fire
sleep 30

# Check if alert fired
curl http://localhost:9091/api/v1/alerts

# Expected output: ServiceDown alert should be present

# Restart service
docker start auth-service
```

### Step 6: Verify Alertmanager Receives Alerts

```bash
# Check Alertmanager status
curl http://localhost:9093/api/v1/status

# Expected output: Should show alerts received from Prometheus
```

### Step 7: Verify Notification (if configured)

```bash
# Trigger a test alert
# (This requires manual intervention or service failure)

# Check email inbox for notification
```

---

## 📊 PORT MAPPING SUMMARY

| Service | Docker Port | Prometheus Scrape Port | Status |
|----------|--------------|----------------------|--------|
| api-gateway | 8080:8080 | 8080 | ✅ Correct |
| auth-service | 8081:8080 | 8081 (was 8080) | ✅ Fixed |
| chat-service | 8082:8082 | 8082 | ✅ Correct |
| video-service | 8083:8083 | 8083 | ✅ Correct |
| storage-service | 8084:8084 | 8084 (was 8080) | ✅ Fixed |
| prometheus | 9091:9090 | 9090 | ✅ Correct |
| alertmanager | 9093:9093 | 9093 | ✅ Correct |

---

## 🎯 IMPLEMENTATION ORDER

1. **Edit** [`configs/prometheus.yml`](secureconnect-backend/configs/prometheus.yml:1)
   - Uncomment alerts.yml (line16)
   - Add alertmanager target (line12)
   - Fix auth-service port (line30)
   - Fix storage-service port (line51)

2. **Edit** [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:1)
   - Add alerts.yml volume mount (line499)

3. **Optional**: Edit [`configs/alertmanager.yml`](secureconnect-backend/configs/alertmanager.yml:1)
   - Configure email/webhook receivers

4. **Restart services**:
   ```bash
   cd secureconnect-backend
   docker-compose -f docker-compose.production.yml restart prometheus alertmanager
   ```

5. **Verify** using verification steps above

---

## 📝 NOTES

- All fixes are configuration-only, no business logic changes
- Alertmanager receiver configuration is optional but recommended for production
- For local development, no-op receivers are acceptable
- SMTP credentials for Alertmanager should be stored in secrets, not hardcoded

---

**Document Version**: 1.0
**Last Updated**: 2026-01-25
