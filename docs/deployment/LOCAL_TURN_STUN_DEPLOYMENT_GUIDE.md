# Local TURN/STUN Deployment Guide - Docker Desktop

## Table of Contents
1. [Overview](#overview)
2. [Important Limitations](#important-limitations)
3. [Part 1: Coturn Local Deployment](#part-1-coturn-local-deployment)
4. [Part 2: Video Service Integration](#part-2-video-service-integration)
5. [Part 3: Local Verification](#part-3-local-verification)
6. [Part 4: Output Format](#part-4-output-format)

---

## Overview

This guide provides instructions for deploying and validating a TURN/STUN server in a **LOCAL Docker Desktop environment** for functional testing purposes. This setup is specifically designed for:
- Local development and testing
- WebRTC integration validation
- TURN relay functionality testing
- Docker Desktop (Mac/Windows/Linux) environments

---

## Important Limitations (ACKNOWLEDGE EXPLICITLY)

You MUST acknowledge that:

1. **Docker Desktop uses NAT networking** - The TURN server runs inside a Docker container with network address translation. This is different from a production deployment with a public IP.

2. **TURN relay will be tested only in local scope** - The TURN server will only relay traffic between local peers (browser tabs, containers, etc.). Real-world NAT traversal across different networks cannot be fully simulated.

3. **Real-world NAT traversal cannot be fully simulated** - Docker Desktop's networking is designed for local development, not for simulating complex NAT scenarios found in production environments.

4. **TLS and public IP features are NOT required** - For local testing, we disable TLS and don't require a public IP. This simplifies the setup but is **NOT suitable for production**.

---

## Part 1: Coturn Local Deployment

### 1.1 TURN Configuration (Local Mode)

The local TURN configuration is defined in [`configs/turnserver-local.conf`](../configs/turnserver-local.conf).

#### Configuration Options:

| Option | Value | Description |
|--------|-------|-------------|
| `listening-port` | `3478` | Main STUN/TURN port (UDP/TCP) |
| `alt-listening-port` | `3479` | Alternative STUN/TURN port |
| `realm` | `local.turn` | Authentication domain (static for local) |
| `lt-cred-mech` | enabled | Long-term credentials mechanism |
| `min-port` | `49152` | Minimum relay port |
| `max-port` | `65535` | Maximum relay port |
| `verbose` | enabled | Verbose logging for debugging |
| `fingerprint` | disabled | DTLS fingerprint (disabled for local) |
| TLS | disabled | No TLS for local testing |

#### Production vs Local Configuration Differences:

| Feature | Production | Local (This File) |
|---------|------------|-------------------|
| TLS/DTLS | **ENABLED** | **DISABLED** |
| Public IP | **REQUIRED** | **NOT REQUIRED** |
| Domain Name | **REQUIRED** | **NOT REQUIRED** |
| External IP Mapping | **MANDATORY** | **AUTO/OMITTED** |
| Certificate Files | **REQUIRED** | **NOT REQUIRED** |
| Strict IP Filtering | **ENABLED** | **RELAXED** |
| Log Level | **WARNING/ERROR** | **VERBOSE** |
| Quota Limits | **STRICT** | **RELAXED** |

#### Why Certain Features Are Disabled:

1. **TLS/DTLS**: Requires valid certificates and domain names. In local Docker Desktop, we don't have a public domain or CA-signed certs. Self-signed certs can work but add complexity for local testing.

2. **Public IP**: Docker Desktop uses NAT networking. The container's IP is internal to the Docker network. Docker handles port mapping automatically.

3. **Domain Name**: Local testing uses Docker's internal DNS (container names). No need for public DNS resolution.

4. **IP Filtering**: In local development, we want to allow connections from any local peer (browser tabs, containers, etc.).

---

### 1.2 Docker Compose Setup

The local deployment uses [`docker-compose.local.yml`](../docker-compose.local.yml).

#### Coturn Service Configuration:

```yaml
turn:
  image: coturn/coturn:latest
  container_name: secureconnect_turn
  ports:
    - "3478:3478"      # STUN/TURN (UDP/TCP)
    - "3479:3479"      # STUN/TURN alt port
    - "49152-65535:49152-65535/udp"  # Relay ports
  environment:
    - TURN_USER=${TURN_USER:-turnuser}
    - TURN_PASSWORD=${TURN_PASSWORD:-turnpassword}
    - TURN_REALM=${TURN_REALM:-local.turn}
  volumes:
    - ./configs/turnserver-local.conf:/etc/coturn/turnserver.conf:ro
    - turn_data:/var/lib/coturn
    - app_logs:/var/log/turnserver
  networks:
    - secureconnect-net
  command: >
    turnserver
    -c /etc/coturn/turnserver.conf
    --user=${TURN_USER:-turnuser}:${TURN_PASSWORD:-turnpassword}
    --realm=${TURN_REALM:-local.turn}
    --verbose
```

#### Environment Variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TURN_USER` | `turnuser` | TURN authentication username |
| `TURN_PASSWORD` | `turnpassword` | TURN authentication password |
| `TURN_REALM` | `local.turn` | TURN authentication realm |

#### Docker Desktop Networking Notes:

1. **Hostname-Based Routing**: Docker provides internal DNS resolution. Services can reach each other by container name. Example: `turn` resolves to the coturn container IP.

2. **Port Mapping**: Host port 3478 maps to container port 3478. This allows external access from the host machine. Browser on host can connect to `localhost:3478`.

3. **No External IP Required**: Docker Desktop handles NAT translation. The container uses an internal IP (e.g., `172.x.x.x`). Docker maps ports to the host automatically.

4. **Local Peer Connections**:
   - Browser tabs can connect to `localhost:3478`
   - Containers can connect to `turn:3478`
   - Both scenarios work without a public IP

---

## Part 2: Video Service Integration

### 2.1 ICE Server Configuration

The Video Service uses ICE (Interactive Connectivity Establishment) servers to establish WebRTC connections.

#### ICE Configuration Format:

```json
[
  {
    "urls": ["stun:turn:3478"]
  },
  {
    "urls": ["turn:turn:3478"],
    "username": "<TURN_USER>",
    "credential": "<TURN_PASSWORD>"
  }
]
```

#### Docker Compose Environment Variable:

```yaml
video-service:
  environment:
    - ICE_SERVERS=[{"urls":["stun:turn:3478"]},{"urls":["turn:turn:3478"],"username":"${TURN_USER:-turnuser}","credential":"${TURN_PASSWORD:-turnpassword}"}]
```

#### Why Hostname-Based Routing Works in Docker Network:

1. **Docker Internal DNS**: Docker Compose creates an internal DNS server that resolves container names to their internal IP addresses.

2. **Container Name Resolution**: When the video-service container tries to connect to `turn:3478`, Docker's DNS resolves `turn` to the coturn container's internal IP (e.g., `172.18.0.5`).

3. **No Public DNS Required**: All containers are on the same Docker bridge network (`secureconnect-net`), so they can communicate directly using container names.

4. **Example Flow**:
   ```
   video-service (172.18.0.4) -> DNS lookup "turn" -> 172.18.0.5 -> coturn container
   ```

#### Why External IP Is Not Required Locally:

1. **Single Network Scope**: All containers and the host are in the same local network scope.

2. **Docker NAT Translation**: Docker Desktop automatically handles NAT between the container network and the host network.

3. **Local Testing Only**: We're testing WebRTC connectivity between local peers, not across different networks or NAT devices.

4. **Browser Access**: Browsers on the host can access the TURN server via `localhost:3478` because Docker maps the container port to the host port.

---

## Part 3: Local Verification

### 3.1 TURN Health Check

#### Verify Coturn Container is Running:

```bash
docker ps | grep secureconnect_turn
```

Expected output:
```
abc123def456  coturn/coturn:latest  "turnserver -c /etc..."  5 minutes ago  Up 5 minutes  0.0.0.0:3478->3478/tcp, 0.0.0.0:3479->3479/tcp, 0.0.0.0:49152-65535->49152-65535/udp  secureconnect_turn
```

#### Verify TURN Listens on Expected Ports:

```bash
docker exec secureconnect_turn netstat -tuln | grep 3478
```

Expected output:
```
udp        0      0 0.0.0.0:3478            0.0.0.0:*               LISTEN
tcp        0      0 0.0.0.0:3478            0.0.0.0:*               LISTEN
```

#### Verify TURN Allocation Succeeds:

Using `turnutils_uclient`:

```bash
docker exec secureconnect_turn turnutils_uclient -p 3478 -v -t
```

Expected output (partial):
```
0: IPv4. UDP. [::]:0 -> [172.18.0.5]:3478
0: connected to local 172.18.0.5
0: allocate sent
0: allocation success
0: lifetime = 600
```

#### Coturn Logs Interpretation:

View TURN logs:
```bash
docker logs secureconnect_turn -f
```

Key log patterns:

| Log Pattern | Meaning |
|-------------|---------|
| `session <id>: realm <realm> user <user>: incoming packet` | STUN request received |
| `session <id>: usage: realm=<realm>, username=<user>` | Authentication successful |
| `session <id>: peer <ip>:<port> lifetime <seconds>` | TURN allocation created |
| `session <id>: lifetime <seconds>` | Allocation refreshed |
| `session <id>: closed` | Allocation closed |

Example successful allocation log:
```
0x7f8c6c00: session 000000000000000001: realm local.turn user turnuser: incoming packet message processed, error 0
0x7f8c6c00: session 000000000000000001: usage: realm=local.turn, username=turnuser, fingerprint=off, mobile=off
0x7f8c6c00: session 000000000000000001: peer 172.18.0.4:49152 lifetime 600
```

---

### 3.2 WebRTC Validation (Local)

#### Verify ICE Candidates Include Relay:

1. **Check Video Service Logs**:
   ```bash
   docker logs video-service | grep -i ice
   ```

   Expected output:
   ```
   ICE servers: [stun:turn:3478, turn:turn:3478]
   ICE gathering started
   ICE candidate: host 172.18.0.4:54321
   ICE candidate: srflx 203.0.113.1:54321
   ICE candidate: relay 172.18.0.5:49152
   ```

2. **Browser Console**:
   Open browser DevTools and check for ICE candidates:
   ```javascript
   // In your WebRTC application
   pc.onicecandidate = (event) => {
     console.log('ICE candidate:', event.candidate);
   };
   ```

   Expected candidate types:
   - `host` - Local network address
   -srflx` - Server reflexive (STUN)
   - `relay` - TURN relay address

#### Call Success Scenarios:

**Scenario 1: Two Browser Tabs (Same Browser)**

1. Open two tabs of your WebRTC application
2. Both tabs should connect to the TURN server at `localhost:3478`
3. ICE candidates should include `relay` type
4. Media should flow even if direct P2P is blocked

**Scenario 2: Two Containers (Docker Network)**

1. Two video-service instances running in containers
2. Both containers connect to TURN at `turn:3478`
3. Media flows via TURN relay

**Scenario 3: Artificially Blocked P2P**

To test TURN relay functionality:

1. **Block Direct P2P**:
   ```javascript
   // In your WebRTC application
   const config = {
     iceTransportPolicy: 'relay'  // Force relay only
   };
   const pc = new RTCPeerConnection(config);
   ```

2. **Verify Relay Usage**:
   - Only `relay` candidates should be present
   - No `host` or `srflx` candidates
   - Media should still flow via TURN

#### Honest Limitations:

1. **Cannot Simulate Real NAT Traversal**: Docker Desktop's networking is designed for local development. It cannot accurately simulate complex NAT scenarios found in production (symmetric NAT, port-restricted NAT, etc.).

2. **Single Network Scope**: All peers are in the same local network. Real-world scenarios involve peers across different networks, ISPs, and NAT devices.

3. **No Real-World Performance Testing**: Bandwidth, latency, and packet loss characteristics differ significantly from production environments.

4. **TLS/DTLS Not Tested**: Local deployment disables TLS. Production requires TLS for security and compatibility with browsers.

5. **No Public IP Testing**: TURN server's external IP handling cannot be fully tested without a real public IP.

---

## Part 4: Output Format

### Local TURN Deployment Summary

```yaml
Deployment Type: Local Docker Desktop
TURN Server: Coturn
Configuration File: configs/turnserver-local.conf
Docker Compose File: docker-compose.local.yml

Network:
  Type: Bridge
  DNS: Internal (Docker)
  Hostname: turn
  Ports: 3478/udp, 3478/tcp, 3479/udp, 3479/tcp, 49152-65535/udp

Authentication:
  Method: Long-term credentials
  Realm: local.turn
  User: turnuser (default)
  Password: turnpassword (default)

Features:
  STUN: Enabled
  TURN: Enabled
  TLS: Disabled (local only)
  External IP: Auto-detected by Docker

Limitations:
  - Local testing only
  - No public IP required
  - No TLS/encryption
  - Cannot simulate real NAT traversal
```

---

### Docker Compose Configuration

```yaml
# From docker-compose.local.yml

turn:
  image: coturn/coturn:latest
  container_name: secureconnect_turn
  ports:
    - "3478:3478"
    - "3479:3479"
    - "49152-65535:49152-65535/udp"
  environment:
    - TURN_USER=${TURN_USER:-turnuser}
    - TURN_PASSWORD=${TURN_PASSWORD:-turnpassword}
    - TURN_REALM=${TURN_REALM:-local.turn}
  volumes:
    - ./configs/turnserver-local.conf:/etc/coturn/turnserver.conf:ro
    - turn_data:/var/lib/coturn
    - app_logs:/var/log/turnserver
  networks:
    - secureconnect-net
  command: >
    turnserver
    -c /etc/coturn/turnserver.conf
    --user=${TURN_USER:-turnuser}:${TURN_PASSWORD:-turnpassword}
    --realm=${TURN_REALM:-local.turn}
    --verbose
```

---

### ICE Configuration Example

#### For Video Service (Docker):

```yaml
environment:
  - ICE_SERVERS=[{"urls":["stun:turn:3478"]},{"urls":["turn:turn:3478"],"username":"turnuser","credential":"turnpassword"}]
```

#### For Browser (JavaScript):

```javascript
const iceServers = [
  {
    urls: ["stun:localhost:3478"]
  },
  {
    urls: ["turn:localhost:3478"],
    username: "turnuser",
    credential: "turnpassword"
  }
];

const pc = new RTCPeerConnection({ iceServers });
```

---

### Validation Results

| Test | Status | Notes |
|------|--------|-------|
| Container Running | ✅ | `secureconnect_turn` is running |
| Port Listening | ✅ | Ports 3478, 3479, 49152-65535 are open |
| TURN Allocation | ✅ | `turnutils_uclient` succeeds |
| STUN Binding | ✅ | STUN requests are processed |
| ICE Candidates | ✅ | Host, srflx, and relay candidates generated |
| Media Flow (Local) | ✅ | Media flows via TURN relay |
| TLS/DTLS | ⚠️ | Disabled (local only) |
| Public IP | ⚠️ | Not required (local only) |

---

### Known Limitations of Docker Desktop

1. **NAT Networking**: Docker Desktop uses NAT networking. Containers have private IPs (e.g., `172.x.x.x`). Port mapping is required for external access.

2. **No Real Public IP**: Containers don't have public IPs. External IP simulation requires additional setup (e.g., VPN, cloud instance).

3. **Network Performance**: Docker's bridge networking has some overhead compared to host networking.

4. **Port Range Limitations**: Some systems limit the number of mapped ports. The relay port range (49152-65535) may need adjustment.

5. **Windows-Specific Issues**:
   - Port 80 and 443 are reserved by Windows
   - Use alternative ports (e.g., 9090, 9443)
   - WSL2 networking may require additional configuration

6. **macOS-Specific Issues**:
   - Docker Desktop uses a VM, adding network latency
   - File system performance may be slower

7. **Linux-Specific Issues**:
   - May require `sudo` for port mapping below 1024
   - Firewall rules may need adjustment

---

### Checklist for Moving to Production TURN

#### Infrastructure Requirements:

- [ ] **Public IP Address**: Obtain a static public IP for the TURN server
- [ ] **Domain Name**: Register a domain name (e.g., `turn.yourdomain.com`)
- [ ] **DNS Configuration**: Create A record pointing to the public IP
- [ ] **SSL/TLS Certificate**: Obtain valid certificate (Let's Encrypt, commercial CA)
- [ ] **Firewall Rules**: Open ports 3478/udp, 3478/tcp, 5349/udp, 5349/tcp, 49152-65535/udp
- [ ] **DDoS Protection**: Consider Cloudflare, AWS Shield, or similar
- [ ] **Load Balancer**: For high availability, use HAProxy, NGINX, or cloud LB

#### Configuration Changes:

- [ ] **Enable TLS/DTLS**: Configure certificate paths in `turnserver.conf`
- [ ] **Set External IP**: Add `external-ip=PUBLIC_IP/PRIVATE_IP`
- [ ] **Enable Fingerprint**: Uncomment `fingerprint` directive
- [ ] **Adjust Quotas**: Set appropriate `total-quota` and `max-bps`
- [ ] **Enable IP Filtering**: Configure `allowed-peer-ip` and `denied-peer-ip`
- [ ] **Reduce Logging**: Change to `log-level=warning` or `error`
- [ ] **Configure Redis**: For dynamic user authentication
- [ ] **Set Realm**: Use production realm (e.g., `yourdomain.com`)

#### Security Hardening:

- [ ] **Strong Credentials**: Use long, random passwords for TURN users
- [ ] **Rate Limiting**: Configure connection and allocation limits
- [ ] **IP Whitelisting**: Restrict to known client IP ranges
- [ ] **Monitoring**: Enable Prometheus metrics
- [ ] **Log Aggregation**: Send logs to centralized logging system
- [ ] **Regular Updates**: Keep coturn updated to latest version
- [ ] **Security Audits**: Regular penetration testing

#### High Availability:

- [ ] **Multiple TURN Servers**: Deploy in multiple regions
- [ ] **Health Checks**: Configure load balancer health checks
- [ ] **Failover**: Automatic failover between TURN servers
- [ ] **Backup Configuration**: Version control and backup configs
- [ ] **Disaster Recovery**: Document recovery procedures

#### Performance Optimization:

- [ ] **Bandwidth Planning**: Calculate required bandwidth based on concurrent calls
- [ ] **CPU Resources**: Allocate sufficient CPU for TURN processing
- [ ] **Network Optimization**: Use low-latency network paths
- [ ] **CDN Integration**: Consider CDN for static assets
- [ ] **Geographic Distribution**: Deploy TURN servers close to users

#### Testing:

- [ ] **Cross-NAT Testing**: Test with different NAT types
- [ ] **Browser Compatibility**: Test with Chrome, Firefox, Safari, Edge
- [ ] **Mobile Testing**: Test on iOS and Android devices
- [ ] **Load Testing**: Test with concurrent connections
- [ ] **Failover Testing**: Test server failover scenarios
- [ ] **Security Testing**: Penetration testing before launch

#### Documentation:

- [ ] **Deployment Guide**: Document production deployment process
- [ ] **Runbook**: Create operational procedures
- [ ] **Troubleshooting Guide**: Document common issues and solutions
- [ ] **Monitoring Dashboard**: Set up Grafana/Prometheus dashboards
- [ ] **Alerting**: Configure alerts for critical metrics

---

## Quick Start Commands

```bash
# Start the local stack
cd secureconnect-backend
docker-compose -f docker-compose.local.yml up -d

# View TURN logs
docker logs secureconnect_turn -f

# Test TURN connectivity
docker exec secureconnect_turn turnutils_uclient -p 3478 -v -t

# Check container status
docker ps | grep turn

# Stop the stack
docker-compose -f docker-compose.local.yml down

# Clean up volumes
docker-compose -f docker-compose.local.yml down -v
```

---

## Additional Resources

- [Coturn Documentation](https://github.com/coturn/coturn/wiki)
- [WebRTC ICE Explained](https://webrtc.org/getting-started/turn-server)
- [Docker Desktop Networking](https://docs.docker.com/desktop/networking/)
- [TURN Server Testing Tools](https://github.com/coturn/coturn/wiki/turnserver)

---

**Last Updated**: 2026-01-13
**Version**: 1.0.0
