# TURN Configuration Summary

## Overview

This document summarizes the TURN (Traversal Using Relays around NAT) server configuration changes made to prevent resource exhaustion on Windows development machines while maintaining production-ready capabilities.

## Problem Statement

The TURN server (coturn) was causing 100% CPU usage, memory exhaustion, and system freezes on Windows Docker Desktop due to:

1. **Port Range Conflicts:** TURN relay ports (50000-50100) overlapped with Windows reserved ports (49152-65535)
2. **Resource Intensive Operations:** Connection tracking and relay port allocation
3. **Docker Desktop NAT Issues:** Poor performance of UDP packet forwarding on Windows

## Solution Implemented

### Profile-Based Configuration

Created two distinct profiles for different deployment scenarios:

| Profile | File | TURN Status | Use Case |
|---------|------|-------------|----------|
| `local-dev` (default) | [`docker-compose.override.yml`](docker-compose.override.yml) | **DISABLED** | Windows local development |
| `production` | [`docker-compose.turn.yml`](docker-compose.turn.yml) | **ENABLED** | Production and Linux/macOS |

### Files Created/Modified

#### New Files Created

1. **[`docker-compose.override.yml`](docker-compose.override.yml)**
   - Disables TURN by default for local development
   - Uses STUN-only mode (Google Public STUN)
   - Removes TURN dependencies from services
   - Applies `local-dev` profile by default

2. **[`docker-compose.turn.yml`](docker-compose.turn.yml)**
   - Enables TURN with full relay capabilities
   - Applies `production` profile
   - Includes TLS configuration
   - Resource limits for production safety

3. **[`docs/deployment/TURN_SERVER_WINDOWS_SAFETY_GUIDE.md`](../docs/deployment/TURN_SERVER_WINDOWS_SAFETY_GUIDE.md)**
   - Comprehensive safety guide
   - Troubleshooting steps
   - Production deployment instructions
   - Monitoring and resource usage guidelines

#### Files Modified

1. **[`.env.local.example`](.env.local.example)**
   - Added `TURN_ENABLED=false` (default)
   - Added `STUN_SERVERS` for STUN-only mode
   - Updated `ICE_SERVERS` to use STUN-only by default
   - Added TURN Windows safety notes

## Environment Variables

### New Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TURN_ENABLED` | `false` | Enable/disable TURN server |
| `STUN_SERVERS` | `stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302` | Public STUN servers |
| `EXTERNAL_IP` | (none) | Public IP for production TURN |

### Existing Environment Variables (Unchanged)

| Variable | Default | Description |
|----------|---------|-------------|
| `TURN_USER` | `turnuser` | TURN authentication username |
| `TURN_PASSWORD` | `turnpassword` | TURN authentication password |
| `TURN_REALM` | `secureconnect` | TURN authentication realm |
| `TURN_SERVER_HOST` | `turn` | TURN server hostname |
| `TURN_SERVER_PORT` | `3478` | TURN server port |

## Usage Instructions

### Local Development (Windows) - TURN DISABLED

```bash
# Start all services WITHOUT TURN (default, safe for Windows)
docker-compose up

# Or explicitly specify the local-dev profile
docker-compose --profile local-dev up

# Start specific services without TURN
docker-compose up api-gateway auth-service chat-service video-service
```

**Result:**
- TURN server is NOT started
- Video service uses STUN-only (Google Public STUN)
- No resource exhaustion issues
- WebRTC works for same-network peers

### Production/Testing - TURN ENABLED

```bash
# Start with TURN using production profile
docker-compose --profile production up

# Or using override files
docker-compose -f docker-compose.yml -f docker-compose.turn.yml up

# Start with TURN on specific server
EXTERNAL_IP=203.0.113.1 docker-compose --profile production up

# Start with TURN using environment file
docker-compose --env-file .env.production --profile production up
```

**Result:**
- TURN server is started with full relay capabilities
- Video service uses STUN + TURN
- Suitable for NAT traversal testing
- **DO NOT run on Windows Docker Desktop**

## Safe Local Development Recommendations

### 1. Use STUN-Only Mode (Default)

For most local development scenarios, STUN-only mode is sufficient:

```json
{
  "iceServers": [
    { "urls": "stun:stun.l.google.com:19302" },
    { "urls": "stun:stun1.l.google.com:19302" }
  ]
}
```

**When STUN-Only Works:**
- Peers on the same local network
- Direct peer-to-peer connections
- Testing WebRTC signaling flow
- Development without NAT traversal

### 2. Test on Same Network

For WebRTC testing:
- Use multiple devices on the same Wi-Fi network
- Test peer-to-peer connections directly
- No TURN server needed for local network peers

### 3. Use Production TURN for Real-World Testing

When testing NAT traversal:
- Deploy TURN server on Linux/Cloud
- Test from external networks
- Use production TURN credentials
- Monitor resource usage on production server

### 4. Alternative TURN Solutions for Windows

If you must run TURN on Windows:

#### Option A: Use WSL2 (Windows Subsystem for Linux)

```bash
# Install WSL2 Ubuntu
wsl --install

# Run Docker inside WSL2
# TURN server will run in Linux environment
# Better performance and fewer issues
```

#### Option B: Use Cloud TURN Service

Instead of running local TURN:
- Use Twilio Network Traversal
- Use Xirsys TURN service
- Use metered.ca TURN service
- These services handle TURN infrastructure

## Resource Usage Comparison

### TURN Disabled (STUN-Only Mode)

| Resource | Usage | Impact |
|----------|--------|--------|
| CPU | Minimal | No impact |
| RAM | Minimal | No impact |
| Disk | Minimal | No impact |
| Ports | None | No conflicts |

### TURN Enabled (Production Mode)

| Resource | Usage | Impact |
|----------|--------|--------|
| CPU | Moderate | Packet forwarding |
| RAM | Low-Moderate | Connection tracking |
| Disk | Moderate | Logging |
| Ports | 100 (50000-50100) | Potential conflicts on Windows |

## High Resource Usage Settings Identified

### 1. Relay Ports

**Production Configuration:**
- File: [`configs/turnserver.conf`](configs/turnserver.conf)
- Lines 69-75: `min-port=50000`, `max-port=50100`
- Range: 100 ports

**Local Configuration:**
- File: [`configs/turnserver-local.conf`](configs/turnserver-local.conf)
- Lines 88-96: `min-port=40000`, `max-port=40020`
- Range: 20 ports

**Windows Reserved Ports:**
- Range: 49152-65535 (ephemeral port range)
- Conflict: Production TURN ports (50000-50100) overlap with Windows range

### 2. Bandwidth Settings

**Production Configuration:**
- File: [`configs/turnserver.conf`](configs/turnserver.conf)
- Line 67: `max-bps=3000000` (3 Mbps)

**Local Configuration:**
- File: [`configs/turnserver-local.conf`](configs/turnserver-local.conf)
- Line 86: `max-bps=5000000` (5 Mbps)

### 3. No Port Range Limit in Production

- Production config uses 100 ports which can be resource-intensive
- Local config uses 20 ports to prevent resource exhaustion

## Monitoring TURN Resource Usage

### Check CPU Usage

```bash
# Check TURN container CPU usage
docker stats secureconnect_turn

# Check system CPU usage
# Windows: Task Manager
# Linux: top or htop
```

### Check Memory Usage

```bash
# Check TURN container memory usage
docker stats secureconnect_turn --format "table {{.Container}}\t{{.MemUsage}}"

# Check system memory
# Windows: Task Manager
# Linux: free -h
```

### Check Port Usage

```bash
# Check which ports are in use
# Windows: netstat -an | findstr "3478"
# Linux: netstat -tuln | grep 3478

# Check relay port usage
# Windows: netstat -an | findstr "50000"
# Linux: netstat -tuln | grep 50000
```

### Check Disk Usage

```bash
# Check TURN log file size
docker exec secureconnect_turn ls -lh /var/log/turnserver/

# Check Docker volume usage
docker system df -v
```

## Troubleshooting

### Issue: 100% CPU Usage on Windows

**Symptoms:**
- System becomes unresponsive
- Docker Desktop shows high CPU usage
- Applications freeze

**Solution:**
```bash
# Stop TURN server immediately
docker-compose stop turn

# Or stop all services
docker-compose down

# Verify TURN is disabled
docker-compose ps  # TURN should not be listed
```

### Issue: Port Already in Use

**Symptoms:**
- TURN container fails to start
- Error: "bind: address already in use"

**Solution:**
```bash
# Find which process is using the port
# Windows: netstat -ano | findstr "3478"
# Linux: lsof -i :3478

# Kill the process or change TURN ports
# Edit configs/turnserver.conf:
listening-port=3479  # Use alternative port
```

### Issue: WebRTC Connection Fails

**Symptoms:**
- Video call fails to connect
- ICE connection state: failed

**Solution:**
```bash
# Check if TURN is running
docker-compose ps turn

# Check TURN logs
docker logs secureconnect_turn

# Test TURN connectivity
docker exec secureconnect_turn turnutils_uclient -v -t

# If TURN is disabled, check STUN servers are accessible
# Browser console: Check ICE candidates
```

## Production Deployment

### Prerequisites

1. **Linux Server:** Ubuntu 20.04+ or similar
2. **Public IP:** Static IP address with proper DNS
3. **TLS Certificates:** Valid SSL certificates for TURN
4. **Firewall Rules:** Proper port forwarding

### Firewall Configuration

```bash
# Allow STUN/TURN ports
sudo ufw allow 3478/udp
sudo ufw allow 3478/tcp
sudo ufw allow 3479/udp
sudo ufw allow 3479/tcp

# Allow TURN TLS ports
sudo ufw allow 5349/tcp
sudo ufw allow 5350/tcp

# Allow relay ports
sudo ufw allow 50000:50100/udp
```

### TLS Certificate Setup

```bash
# Create certs directory
mkdir -p secureconnect-backend/certs

# Generate self-signed certificate (for testing)
openssl req -x509 -newkey rsa:2048 -keyout certs/turnserver_key.pem \
  -out certs/turnserver_cert.pem -days 365 -nodes \
  -subj "/CN=turn.yourdomain.com"

# Or use Let's Encrypt (for production)
certbot certonly --standalone -d turn.yourdomain.com
```

### Deploy Production TURN

```bash
# Copy environment file
cp .env.production.example .env.production

# Edit production settings
nano .env.production

# Start with production profile
docker-compose --profile production up -d

# Verify TURN is running
docker-compose ps turn
docker logs secureconnect_turn
```

## Summary

| Scenario | TURN Status | Command | Platform |
|----------|-------------|---------|----------|
| Local Windows Dev | Disabled | `docker-compose up` | Windows |
| Local Linux Dev | Optional | `docker-compose up` | Linux |
| Production | Enabled | `docker-compose --profile production up` | Linux/Cloud |
| WebRTC Testing | Optional | `docker-compose --profile production up` | Linux/Cloud |

## Key Takeaways

1. **TURN is DISABLED by default** on Windows to prevent resource exhaustion
2. **Use STUN-only mode** for most local development scenarios
3. **Enable TURN only when needed** for NAT traversal testing
4. **Do NOT run TURN on Windows Docker Desktop** for extended periods
5. **Use Linux/Cloud for production TURN** deployment
6. **Monitor resource usage** when TURN is enabled
7. **Use production TURN services** instead of running local TURN if possible

## Additional Resources

- [Coturn Documentation](https://github.com/coturn/coturn)
- [WebRTC ICE Configuration](https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/RTCPeerConnection)
- [Docker Compose Profiles](https://docs.docker.com/compose/profiles/)
- [TURN Server Best Practices](https://webrtc.org/getting-started/turn-server)
- [TURN Server Windows Safety Guide](../docs/deployment/TURN_SERVER_WINDOWS_SAFETY_GUIDE.md)

## Rules Compliance

✅ **DO NOT remove TURN from production** - TURN is fully enabled in production profile
✅ **DO NOT enable TURN on Windows dev** - TURN is disabled by default for Windows safety
