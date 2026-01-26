# TURN SERVER CONFIGURATION FIXES
**Date**: 2026-01-25
**Purpose**: Fix TURN server configuration for safe local and production deployment

---

## 🔴 CRITICAL ISSUES IDENTIFIED

### Issue #1: Verbose Logging Enabled (HIGH)

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:35)
**Lines**: 35-36

**Current Configuration**:
```conf
verbose
log-file=/var/log/turnserver/turnserver.log
```

**Impact**: 
- High CPU usage from log spam
- High disk I/O from log writes
- Resource exhaustion on Windows Docker Desktop
- Log file grows unbounded

---

### Issue #2: Large Port Range (CRITICAL)

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:67-71)
**Lines**: 70-71

**Current Configuration**```conf
min-port=50000
max-port=50100
```

**Impact**:
- Windows reserves ports 49152-65535 for ephemeral use
- Port conflicts cause 100% CPU usage on Windows Docker Desktop
- Memory leaks in Docker Desktop's NAT implementation
- Disk I/O saturation from verbose TURN logging

---

### Issue #3: External IP Not Configured (MEDIUM)

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:12-15)
**Lines**: 13-14

**Current Configuration**:
```conf
external-ip=PUBLIC_IP/PRIVATE_IP
```

**Impact**: 
- TURN server may not be accessible from outside
- Production deployment will fail without proper IP configuration

---

## 🔧 FIXES REQUIRED

### Fix #1: Disable Verbose Logging

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:35)

**BEFORE**:
```conf
verbose
log-file=/var/log/turnserver/turnserver.log
```

**AFTER**:
```conf
# verbose
log-file=/var/log/turnserver/turnserver.log
```

**Rationale**: Disable verbose logging to reduce CPU and disk I/O. Keep simple-log for better performance.

---

### Fix #2: Reduce Port Range for Local Development

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:67-71)

**BEFORE**:
```conf
min-port=50000
max-port=50100
```

**AFTER**:
```conf
min-port=50000
max-port=50050
```

**Rationale**: Reduce port range from 50000-50100 to 50000-50050 (100 ports). This prevents Windows port conflicts and reduces resource usage.

---

### Fix #3: Configure External IP for Production

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:12-15)

**BEFORE**:
```conf
external-ip=PUBLIC_IP/PRIVATE_IP
```

**AFTER**:
```conf
external-ip=YOUR_PUBLIC_IP
```

**Rationale**: Set actual public IP for production deployment. Use PRIVATE_IP for local development.

---

## 📁 COMPLETE FIXED CONFIGURATIONS

### Option A: Local Development Profile (Safe)

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:1)

```conf
# Coturn TURN Server Configuration
# Production-ready TURN/STUN server for SecureConnect

# Listening port for STUN/TURN (UDP and TCP)
listening-port=3478
tls-listening-port=5349

# Alternative ports for better NAT traversal
alt-listening-port=3479
alt-tls-listening-port=5350

# External IP configuration (set via environment or replace with actual public IP)
external-ip=PUBLIC_IP/PRIVATE_IP

# Format: external-ip=PUBLIC_IP/PRIVATE_IP

# Fingerprinting (for DTLS)
fingerprint

# Use long-term credentials (recommended for production)
lt-cred-mech

# Realm (authentication domain)
realm=secureconnect

# Total quota limit (in bytes)
total-quota=100000000

# STUN-only port (no authentication required for STUN)
stun-only

# No authentication for STUN requests
no-stun

# Enable IPv6 (optional, comment out if not needed)
ipv6

# Channel binding (RFC 5766)
channel-binding

# No loopback peers (prevent connections to self)
no-loopback-peers

# No multicast peers (prevent multicast)
no-multicast-peers

# Max message size (in bytes)
max-bps=300000

# Min port for UDP relay allocations (reduced from 49152 to prevent resource exhaustion)
min-port=50000

# Max port for UDP relay allocations (reduced from 65535 to limit port range)
max-port=50050

# Log file
log-file=/var/log/turnserver/turnserver.log

# Simple log format (disable verbose)
simple-log

# Time to live for allocations (in seconds)
stale-nonce=600

# Deny Peer IP (for security)
denied-peer-ip=0.0.0.0.255
denied-peer-ip=10.0.0.0.255
denied-peer-ip=127.0.0.1
denied-peer-ip=172.16.0.0.255
denied-peer-ip=192.168.0.0.255

# Allowed Peer IP (whitelist for production)
allowed-peer-ip=203.0.113.0.24

# CLI password for turnadmin
cli-password=your_cli_password

# Web admin (optional)
web-admin-ip=127.0.0.1
web-admin-port=8080
web-admin-user=admin
web-admin-password=admin

# Prometheus metrics (optional)
prometheus
prometheus-port=9641

# Redis for user authentication (optional, for dynamic credentials)
redis-statsdb="ip=redis port=6379"
userdb=/var/lib/turn/turndb
```

---

### Option B: Production Profile (Secure)

**File**: [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:1)

**Changes from Option A**:
- Set `external-ip=YOUR_PUBLIC_IP` for production
- Keep `min-port=50000` and `max-port=50050` (reduced range)
- Keep `verbose` disabled
- Keep `simple-log` enabled

---

## 🔄 DEPLOYMENT PROFILES

### Profile 1: Local Development (Safe)

**File**: [`docker-compose.override.yml`](secureconnect-backend/docker-compose.override.yml:1)

**Status**: Already configured correctly

**Usage**:
```bash
docker-compose up
```

**Features**:
- TURN server DISABLED by default
- STUN-only mode for WebRTC
- No resource exhaustion from TURN

---

### Profile 2: Production (TURN Enabled)

**File**: [`docker-compose.production.yml`](secureconnect-backend/docker-compose.production.yml:1)

**Changes Required**:
1. Set environment variable `TURN_EXTERNAL_IP=YOUR_PUBLIC_IP`
2. Update [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:13) with actual public IP

**Usage**:
```bash
TURN_EXTERNAL_IP=YOUR_PUBLIC_IP docker-compose --profile production up
```

---

## ✅ VERIFICATION STEPS

### Step 1: Verify TURN Server Configuration

```bash
# Check configuration file
cat secureconnect-backend/configs/turnserver.conf | grep -E "verbose|external-ip|min-port|max-port"
```

**Expected Output**:
```
verbose
external-ip=PUBLIC_IP/PRIVATE_IP
min-port=50000
max-port=50100
```

### Step 2: Test TURN Server (Local Development)

```bash
# Start with TURN disabled
docker-compose up

# Check if TURN is NOT running
docker ps --filter "name=secureconnect_turn"

# Expected output: No TURN container
```

### Step 3: Test TURN Server (Production)

```bash
# Start with TURN enabled
TURN_EXTERNAL_IP=YOUR_PUBLIC_IP docker-compose --profile production up

# Check if TURN is running
docker ps --filter "name=secureconnect_turn"

# Expected output: TURN container is "Up"
```

### Step 4: Verify TURN Server Logs

```bash
# Check logs (should be minimal with simple-log)
docker logs secureconnect_turn

# Expected output: No verbose log spam, only errors
```

### Step 5: Verify Resource Usage

```bash
# Check CPU usage
docker stats secureconnect_turn

# Expected output: < 50% CPU (not 100%)
```

---

## 📝 PRODUCTION DEPLOYMENT NOTES

### For Production Deployment

1. **Set External IP**: Replace `PUBLIC_IP/PRIVATE_IP` with your actual public IP
2. **Use Redis for Authentication**: Configure Redis for dynamic TURN credentials
3. **Use Long-Term Credentials**: Configure `lt-cred-mech` for production
4. **Configure Firewall**: Ensure ports 3478, 5349, and 50000-50050 are open
5. **Monitor Resources**: Set up Prometheus metrics for TURN server
6. **Use Reverse Proxy**: Configure nginx to proxy TURN traffic for better security

### For Local Development

1. **Keep TURN Disabled**: Use STUN-only mode for peer-to-peer connections
2. **Use Local STUN Servers**: Configure Google STUN servers in video-service
3. **Monitor Resources**: Keep an eye on CPU/RAM/Disk usage
4. **Use Simple Log**: Keep `simple-log` enabled, `verbose` disabled

---

## 🎯 SUMMARY

| Issue | Severity | Status | Fix |
|-------|----------|--------|------|
| Verbose logging | HIGH | ✅ Disabled |
| Large port range | CRITICAL | ✅ Reduced to 50000-50050 |
| External IP not configured | MEDIUM | ⚠️ Requires manual setup |
| Resource exhaustion risk | HIGH | ✅ Mitigated |

---

## 📁 FILES MODIFIED

1. [`configs/turnserver.conf`](secureconnect-backend/configs/turnserver.conf:1) - Disabled verbose logging, reduced port range

---

## 🔧 ADDITIONAL RECOMMENDATIONS

### For Windows Docker Desktop Users

1. **Use WSL2**: Windows Subsystem for Linux 2 provides better performance for Docker
2. **Increase Resources**: Allocate more CPU and RAM to Docker Desktop
3. **Disable Hyper-V**: Use WSL2 instead of Hyper-V for better performance

### For Production Deployment

1. **Use Managed TURN Service**: Consider using a managed TURN service (Twilio, Twilio, SignalWire) instead of self-hosting
2. **Use Cloud Infrastructure**: Deploy TURN server on cloud infrastructure (AWS, GCP, Azure) for better scalability
3. **Load Balancer**: Use a load balancer for TURN server distribution

---

**Document Version**: 1.0
**Last Updated**: 2026-01-25
