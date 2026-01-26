# TURN Server Windows Safety Guide

## Overview

This guide explains how to safely use the TURN (Traversal Using Relays around NAT) server in SecureConnect while preventing resource exhaustion issues on Windows development machines.

## Problem: TURN Server Freezing Windows Machines

### Symptoms
- 100% CPU usage on Windows Docker Desktop
- Memory exhaustion leading to system freezes
- Disk I/O saturation from verbose logging
- Port conflicts with Windows reserved port ranges

### Root Causes

#### 1. Port Range Conflicts
- **Windows Reserved Ports:** 49152-65535 (ephemeral port range)
- **TURN Relay Ports:** 50000-50100 (100 ports) - overlaps with Windows range
- **Result:** Port allocation conflicts causing 100% CPU usage

#### 2. Resource Intensive Operations
- **Relay Port Allocation:** Each TURN connection requires a UDP port
- **Connection Tracking:** TURN maintains state for all relayed connections
- **Logging:** Verbose logging writes to disk continuously

#### 3. Docker Desktop NAT Issues
- Docker Desktop's NAT implementation on Windows has known issues
- High packet throughput causes memory leaks
- UDP packet forwarding is less efficient than on Linux/macOS

## Solution: Profile-Based Configuration

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    docker-compose.yml                        │
│                     (Base Configuration)                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
           ┌───────────────┴───────────────┐
           │                               │
           ▼                               ▼
┌──────────────────────────┐    ┌──────────────────────────┐
│ docker-compose.override  │    │ docker-compose.turn.yml  │
│     (local-dev)          │    │     (production)         │
│                          │    │                          │
│ - TURN: DISABLED         │    │ - TURN: ENABLED          │
│ - STUN: Google Public    │    │ - STUN: Google Public    │
│ - Ports: None            │    │ - Ports: 50000-50100     │
│ - Profile: default       │    │ - Profile: production     │
└──────────────────────────┘    └──────────────────────────┘
```

### Configuration Files

| File | Purpose | TURN Status | Profile |
|------|---------|-------------|---------|
| [`docker-compose.yml`](../secureconnect-backend/docker-compose.yml) | Base configuration | Defined but not started | - |
| [`docker-compose.override.yml`](../secureconnect-backend/docker-compose.override.yml) | Local development override | Disabled | `local-dev` (default) |
| [`docker-compose.turn.yml`](../secureconnect-backend/docker-compose.turn.yml) | Production TURN server | Enabled | `production` |

## Environment Variables

### TURN Configuration Variables

| Variable | Default | Description | Required |
|----------|---------|-------------|----------|
| `TURN_ENABLED` | `false` | Enable/disable TURN server | No |
| `TURN_SERVER_HOST` | `turn` | TURN server hostname | Yes (if enabled) |
| `TURN_SERVER_PORT` | `3478` | TURN server port | No |
| `TURN_USER` | `turnuser` | TURN authentication username | Yes (if enabled) |
| `TURN_PASSWORD` | `turnpassword` | TURN authentication password | Yes (if enabled) |
| `TURN_REALM` | `secureconnect` | TURN authentication realm | No |
| `EXTERNAL_IP` | (none) | Public IP for production | Yes (production) |
| `STUN_SERVERS` | `stun:stun.l.google.com:19302` | STUN server URLs | No |

### Environment File Examples

#### `.env.local` (Windows Development)
```bash
# TURN Server - DISABLED for Windows local development
TURN_ENABLED=false

# STUN Servers (free, no resource impact)
STUN_SERVERS=stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302

# TURN credentials (not used when TURN_ENABLED=false)
TURN_SERVER_HOST=
TURN_SERVER_PORT=3478
TURN_USER=
TURN_PASSWORD=
TURN_REALM=local.turn
```

#### `.env.production` (Linux/Cloud)
```bash
# TURN Server - ENABLED for production
TURN_ENABLED=true

# STUN Servers (always available)
STUN_SERVERS=stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302

# TURN credentials (REQUIRED for production)
TURN_SERVER_HOST=turn
TURN_SERVER_PORT=3478
TURN_USER=prod_turn_user
TURN_PASSWORD=strong_secure_password_here
TURN_REALM=secureconnect

# External IP (REQUIRED for production)
EXTERNAL_IP=203.0.113.1
```

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

### Enable TURN on Windows (NOT RECOMMENDED)

```bash
# WARNING: This may cause 100% CPU usage and system freeze
# Only use if absolutely necessary for testing
TURN_ENABLED=true docker-compose up

# Or create .env.local with TURN_ENABLED=true
# Then run: docker-compose up
```

**Risks:**
- 100% CPU usage
- Memory exhaustion
- System freeze
- Port conflicts

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

#### Option C: Use Minimal TURN Configuration

If running TURN on Windows is absolutely necessary:

```bash
# Create minimal TURN configuration
# Reduce relay ports to 10 instead of 100
# Disable verbose logging
# Limit bandwidth

# Edit configs/turnserver-local.conf:
min-port=40000
max-port=40010  # Only 10 ports
max-bps=1000000  # Limit to 1 Mbps
# verbose  # Comment out verbose logging
```

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

### Issue: High Disk Usage

**Symptoms:**
- Docker volumes filling up
- Large log files

**Solution:**
```bash
# Check log file size
docker exec secureconnect_turn du -sh /var/log/turnserver/

# Truncate log files
docker exec secureconnect_turn truncate -s 0 /var/log/turnserver/turnserver.log

# Disable verbose logging
# Edit configs/turnserver.conf:
# verbose  # Comment out this line
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
