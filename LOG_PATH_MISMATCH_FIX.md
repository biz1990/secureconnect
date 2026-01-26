# LOG PATH MISMATCH FIX - LOKI INGESTION FAILURE

## Problem

Services write logs to Docker's stdout/stderr (captured by Docker), but Promtail is configured to read from `./logs:/var/log/secureconnect:ro` directory on the host. This mismatch causes Loki to receive no application logs.

## Root Cause Analysis

### Current Configuration

**Docker Compose Production ([`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:202))**
```yaml
volumes:
  app_logs:  # Named volume
```

**Services mount:**
```yaml
volumes:
  - app_logs:/logs  # Services write to /logs inside container
```

**Docker Compose Monitoring ([`docker-compose.monitoring.yml`](secureconnect-backend/docker-compose.monitoring.yml:100))**
```yaml
promtail:
  volumes:
    - ./logs:/var/log/secureconnect:ro  # Reads from host ./logs directory
```

### Why Docker Running is Not Enough

Even if all containers run successfully:
1. Services write to stdout (captured by Docker's logging driver)
2. Services also mount `app_logs:/logs` but this is NOT where they write
3. Promtail reads from `./logs` on the HOST
4. The `./logs` directory on host is empty or contains only Docker's stdout capture
5. Loki receives no logs from services

### Log Flow Diagram

```
┌─────────────────┐
│  Service       │
│  Container     │
└──────┬──────────┘
       │
       │ writes to
       ▼
┌─────────────────┐
│  Docker        │
│  stdout/stderr  │
└──────┬──────────┘
       │
       │ captured to
       ▼
┌─────────────────┐
│  Host ./logs   │  ◄───┐
│  directory     │  reads  │
└─────────────────┘          │
       │                  │
       │                  │
       ▼                  ▼
┌─────────────────┐    ┌─────────────────┐
│   Promtail     │    │     Loki     │
│  Container     │───▶│             │
└─────────────────┘    └─────────────────┘
```

**Current State:** Promtail reads from host directory, but services write to Docker's stdout (not to that directory).

---

## FIX OPTIONS

### Option A: Change Promtail Volume Mount (RECOMMENDED)

**Approach:** Mount the same Docker volume that services use (`app_logs`) instead of host directory.

**Pros:**
- ✅ Simple, single-line change
- ✅ Uses Docker's native volume sharing
- ✅ Works with Docker's stdout capture
- ✅ No host filesystem dependencies

**Cons:**
- ⚠️ Requires restarting promtail container
- ⚠️ Logs are in Docker volume, not directly accessible on host

**File:** `secureconnect-backend/docker-compose.monitoring.yml`

```diff
<<<<<<< SEARCH
:start_line:95
-------
  # 4. PROMTAIL - Log Collector (Optional)
  promtail:
    image: grafana/promtail:2.9.2
    container_name: secureconnect_promtail
    volumes:
      - ./configs/promtail-config.yml:/etc/promtail/config.yml:ro
      - ./logs:/var/log/secureconnect:ro
=======
  # 4. PROMTAIL - Log Collector (Optional)
  promtail:
    image: grafana/promtail:2.9.2
    container_name: secureconnect_promtail
    volumes:
      - ./configs/promtail-config.yml:/etc/promtail/config.yml:ro
      - app_logs:/var/log/secureconnect:ro
>>>>>>> REPLACE
```

**Validation:**
```bash
# Restart promtail to pick up new volume mount
docker-compose -f docker-compose.monitoring.yml restart promtail

# Wait for logs to appear
sleep 10

# Verify logs are in Loki
# Query in Grafana: Explore → Loki → {job="api-gateway"}
# Expected: Should see recent log entries
```

---

### Option B: Switch Services to Stdout Logging (ALTERNATIVE)

**Approach:** Configure all services to log to stdout (already default) and configure Promtail to read from Docker socket.

**Pros:**
- ✅ No volume mount changes needed
- ✅ Logs flow naturally via Docker
- ✅ Host directory remains clean
- ✅ More aligned with containerization best practices

**Cons:**
- ⚠️ Requires significant promtail-config.yml changes
- ⚠️ More complex configuration
- ⚠️ Requires Docker socket access

**File:** `secureconnect-backend/configs/promtail-config.yml`

```diff
<<<<<<< SEARCH
:start_line:10
-------
scrape_configs:
  # API Gateway logs
  - job_name: api-gateway
    static_configs:
      - targets:
          - localhost
        labels:
            job: api-gateway
            service: api-gateway
    pipeline_stages:
      - json:
          expressions:
              level: level
              msg: message
              service: service
      - labels:
              level:
              service:
              hostname:
      - output:
          source: stdout
=======
scrape_configs:
  # API Gateway logs - read from Docker
  - job_name: api-gateway
    docker_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        labels:
          job: api-gateway
          service: api-gateway
    pipeline_stages:
      - json:
          expressions:
              level: level
              msg: message
              service: service
      - labels:
              level:
              service:
              hostname:
      - output:
          source: stdout

  # Auth Service logs - read from Docker
  - job_name: auth-service
    docker_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        labels:
          job: auth-service
          service: auth-service
    pipeline_stages:
      - json:
          expressions:
              level: level
              msg: message
              service: service
      - labels:
              level:
              service:
              hostname:
      - output:
          source: stdout

  # Chat Service logs - read from Docker
  - job_name: chat-service
    docker_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        labels:
          job: chat-service
          service: chat-service
    pipeline_stages:
      - json:
          expressions:
              level: level
              msg: message
              service: service
      - labels:
              level:
              service:
              hostname:
      - output:
          source: stdout

  # Video Service logs - read from Docker
  - job_name: video-service
    docker_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        labels:
          job: video-service
          service: video-service
    pipeline_stages:
      - json:
          expressions:
              level: level
              msg: message
              service: service
      - labels:
              level:
              service:
              hostname:
      - output:
          source: stdout

  # Storage Service logs - read from Docker
  - job_name: storage-service
    docker_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        labels:
          job: storage-service
          service: storage-service
    pipeline_stages:
      - json:
          expressions:
              level: level
              msg: message
              service: service
      - labels:
              level:
              service:
              hostname:
      - output:
          source: stdout
>>>>>>> REPLACE
```

**Update docker-compose.monitoring.yml to add Docker socket:**

```diff
<<<<<<< SEARCH
:start_line:95
-------
  # 4. PROMTAIL - Log Collector (Optional)
  promtail:
    image: grafana/promtail:2.9.2
    container_name: secureconnect_promtail
    volumes:
      - ./configs/promtail-config.yml:/etc/promtail/config.yml:ro
      - app_logs:/var/log/secureconnect:ro
=======
  # 4. PROMTAIL - Log Collector (Optional)
  promtail:
    image: grafana/promtail:2.9.2
    container_name: secureconnect_promtail
    volumes:
      - ./configs/promtail-config.yml:/etc/promtail/config.yml:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - app_logs:/var/log/secureconnect:ro
>>>>>>> REPLACE
```

**Validation:**
```bash
# Restart promtail with new config
docker-compose -f docker-compose.monitoring.yml restart promtail

# Verify logs appear in Loki
# Query in Grafana: Explore → Loki → {job="api-gateway"}
# Expected: Should see recent log entries
```

---

## RECOMMENDATION

**Use Option A (Change Promtail Volume Mount)**

### Rationale:

1. **Simpler Implementation**
   - Single line change in docker-compose.monitoring.yml
   - No promtail-config.yml modifications
   - Follows existing pattern

2. **Less Risky**
   - Doesn't introduce Docker socket access (security consideration)
   - Doesn't require complex promtail configuration changes
   - Uses Docker's proven volume sharing mechanism

3. **Production Best Practice**
   - Named Docker volumes are the standard way to share data between containers
   - Aligns with how services already use volumes

4. **Easier Debugging**
   - Logs can be inspected directly from volume if needed
   - Consistent with Docker's logging architecture

5. **Faster Recovery**
   - If promtail restarts, it automatically reconnects to the volume
   - No complex socket reconnection handling

### Implementation Steps:

1. Apply the fix to `docker-compose.monitoring.yml`
2. Restart promtail container:
   ```bash
   docker-compose -f docker-compose.monitoring.yml restart promtail
   ```
3. Verify logs appear in Loki:
   - Open Grafana: http://localhost:3000
   - Navigate to Explore → Loki
   - Query: `{job="api-gateway"}`
   - Should see recent log entries

### Expected Outcome:

- ✅ Loki receives logs from all services
- ✅ Grafana dashboards show log data
- ✅ Log aggregation works correctly
- ✅ Audit trails are complete

---

## VALIDATION CHECKLIST

After applying Option A:

- [ ] Promtail container restarts successfully
- [ ] No errors in promtail logs
- [ ] Logs appear in Grafana Loki Explore
- [ ] Query {job="api-gateway"} returns results
- [ ] Query {job="auth-service"} returns results
- [ ] Query {job="chat-service"} returns results
- [ ] Query {job="video-service"} returns results
- [ ] Query {job="storage-service"} returns results
- [ ] Log levels (info, warn, error) are correctly labeled
- [ ] Service labels are correctly applied

---

## ALTERNATIVE: Option B Considerations

If Option B (Docker socket) is preferred for your environment:

### Requirements:
- Docker socket must be mounted with read permissions
- Promtail must run in Docker network or have proper access
- More complex configuration management

### When to Use Option B:
- You need to inspect logs from multiple Docker Compose projects
- You prefer centralized log collection via Docker socket
- Your security policy allows Docker socket access

---

## FILES MODIFIED

| File | Change | Lines |
|-------|----------|--------|
| `docker-compose.monitoring.yml` | Change volume mount | 1 |

---

## COMPARISON

| Aspect | Option A (Volume) | Option B (Docker Socket) |
|--------|-------------------|------------------------|
| Complexity | LOW | HIGH |
| Risk | LOW | MEDIUM |
| Docker Changes | 1 line | 5+ lines |
| Config Changes | 0 lines | 50+ lines |
| Debugging | Easier | Harder |
| Production Ready | YES | YES |
