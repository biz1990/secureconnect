# Local TURN/STUN Validation Guide

## Overview

This guide provides step-by-step procedures for validating the TURN/STUN server deployment in a local Docker Desktop environment.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Health Check](#quick-health-check)
3. [TURN Server Validation](#turn-server-validation)
4. [WebRTC Validation](#webrtc-validation)
5. [Troubleshooting](#troubleshooting)
6. [Expected Results](#expected-results)

---

## Prerequisites

Before starting validation, ensure:

- [ ] Docker Desktop is running
- [ ] All containers are started: `docker-compose -f docker-compose.local.yml up -d`
- [ ] `.env.local` file exists (copy from `.env.local.example`)
- [ ] TURN credentials are set in environment variables

---

## Quick Health Check

### 1. Verify Container Status

```bash
docker ps --filter name=turn
```

**Expected Output:**
```
CONTAINER ID   IMAGE                COMMAND                  CREATED         STATUS         PORTS                                                                                                                  NAMES
abc123def456   coturn/coturn:latest  "turnserver -c /etc/..."   2 minutes ago   Up 2 minutes   0.0.0.0:3478->3478/tcp, 0.0.0.0:3479->3479/tcp, 0.0.0.0:49152-65535->49152-65535/udp   secureconnect_turn
```

### 2. Verify Network Connectivity

```bash
docker exec secureconnect_turn ping -c 3 redis
```

**Expected Output:**
```
PING redis (172.18.0.3): 56 data bytes
64 bytes from 172.18.0.3: icmp_seq=0 ttl=64 time=0.123 ms
64 bytes from 172.18.0.3: icmp_seq=1 ttl=64 time=0.089 ms
64 bytes from 172.18.0.3: icmp_seq=2 ttl=64 time=0.091 ms
--- redis ping statistics ---
3 packets transmitted, 3 packets received, 0.0% packet loss
```

### 3. Verify Port Listening

```bash
docker exec secureconnect_turn netstat -tuln | grep -E '3478|3479'
```

**Expected Output:**
```
udp        0      0 0.0.0.0:3478            0.0.0.0:*               LISTEN
tcp        0      0 0.0.0.0:3478            0.0.0.0:*               LISTEN
udp        0      0 0.0.0.0:3479            0.0.0.0:*               LISTEN
tcp        0      0 0.0.0.0:3479            0.0.0.0:*               LISTEN
```

---

## TURN Server Validation

### 1. STUN Binding Request Test

Test STUN binding without authentication:

```bash
docker exec secureconnect_turn turnutils_uclient -p 3478 -v
```

**Expected Output (Partial):**
```
0: IPv4. UDP. [::]:0 -> [172.18.0.5]:3478
0: connected to local 172.18.0.5
0: send STUN Binding request
0: received STUN Binding response
0: XOR-MAPPED-ADDRESS: 172.18.0.5:49152
0: done
```

### 2. TURN Allocation Test

Test TURN allocation with authentication:

```bash
docker exec secureconnect_turn turnutils_uclient -p 3478 -v -t -u turnuser -w turnpassword
```

**Expected Output (Partial):**
```
0: IPv4. UDP. [::]:0 -> [172.18.0.5]:3478
0: connected to local 172.18.0.5
0: allocate sent
0: allocation success
0: lifetime = 600
0: relay address = 172.18.0.5:49152
0: done
```

### 3. TURN Relay Test

Test TURN relay functionality:

```bash
# Terminal 1: Start a listener
docker exec -it secureconnect_turn turnutils_uclient -p 3478 -v -t -u turnuser -w turnpassword -L

# Terminal 2: Send data to the relay
docker exec -it secureconnect_turn turnutils_uclient -p 3478 -v -t -u turnuser -w turnpassword -c <relay-address>:<relay-port>
```

**Expected Output:**
```
# Terminal 1:
0: listening on 0.0.0.0:0
0: waiting for connections...

# Terminal 2:
0: connected to relay 172.18.0.5:49152
0: send data...
0: received data
```

### 4. Log Verification

Check TURN server logs for errors:

```bash
docker logs secureconnect_turn 2>&1 | grep -i error
```

**Expected Output:**
```
(no errors should be present)
```

Check for successful allocations:

```bash
docker logs secureconnect_turn 2>&1 | grep -i "allocation success"
```

**Expected Output:**
```
0x7f8c6c00: session 000000000000000001: allocation success
```

---

## WebRTC Validation

### 1. Video Service ICE Configuration Check

Verify the video service is configured with correct ICE servers:

```bash
docker exec video-service env | grep ICE_SERVERS
```

**Expected Output:**
```
ICE_SERVERS=[{"urls":["stun:turn:3478"]},{"urls":["turn:turn:3478"],"username":"turnuser","credential":"turnpassword"}]
```

### 2. Video Service Logs Check

Check video service logs for ICE-related messages:

```bash
docker logs video-service 2>&1 | grep -i ice
```

**Expected Output (Partial):**
```
INFO: ICE servers configured: [stun:turn:3478 turn:turn:3478]
INFO: ICE gathering started
INFO: ICE state: checking
INFO: ICE state: connected
INFO: ICE state: completed
```

### 3. Browser-Based Testing

#### Test 1: Simple STUN Test

Create a simple HTML file `test-stun.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>STUN Test</title>
</head>
<body>
    <h1>STUN Server Test</h1>
    <button onclick="testSTUN()">Test STUN</button>
    <pre id="results"></pre>

    <script>
        async function testSTUN() {
            const results = document.getElementById('results');
            results.textContent = 'Testing STUN server...\n';

            const pc = new RTCPeerConnection({
                iceServers: [
                    { urls: 'stun:localhost:3478' }
                ]
            });

            pc.onicecandidate = (event) => {
                if (event.candidate) {
                    results.textContent += `ICE Candidate: ${JSON.stringify(event.candidate)}\n`;
                } else {
                    results.textContent += 'ICE gathering complete\n';
                }
            };

            pc.oniceconnectionstatechange = () => {
                results.textContent += `ICE State: ${pc.iceConnectionState}\n`;
            };

            // Create a data channel to trigger ICE gathering
            pc.createDataChannel('test');

            const offer = await pc.createOffer();
            await pc.setLocalDescription(offer);
        }
    </script>
</body>
</html>
```

**Expected Results:**
- ICE candidates should be generated
- At least one `srflx` (server reflexive) candidate should appear
- ICE state should reach `completed`

#### Test 2: TURN Relay Test

Create a simple HTML file `test-turn.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>TURN Test</title>
</head>
<body>
    <h1>TURN Server Test</h1>
    <button onclick="testTURN()">Test TURN</button>
    <pre id="results"></pre>

    <script>
        async function testTURN() {
            const results = document.getElementById('results');
            results.textContent = 'Testing TURN server...\n';

            const pc = new RTCPeerConnection({
                iceServers: [
                    { urls: 'stun:localhost:3478' },
                    { urls: 'turn:localhost:3478', username: 'turnuser', credential: 'turnpassword' }
                ],
                iceTransportPolicy: 'relay'  // Force relay only
            });

            pc.onicecandidate = (event) => {
                if (event.candidate) {
                    results.textContent += `ICE Candidate: ${JSON.stringify(event.candidate)}\n`;
                } else {
                    results.textContent += 'ICE gathering complete\n';
                }
            };

            pc.oniceconnectionstatechange = () => {
                results.textContent += `ICE State: ${pc.iceConnectionState}\n`;
            };

            // Create a data channel to trigger ICE gathering
            pc.createDataChannel('test');

            const offer = await pc.createOffer();
            await pc.setLocalDescription(offer);
        }
    </script>
</body>
</html>
```

**Expected Results:**
- Only `relay` candidates should appear (no `host` or `srflx`)
- ICE state should reach `connected`

#### Test 3: Two-Peer Connection Test

Create a simple HTML file `test-webrtc.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>WebRTC Test</title>
</head>
<body>
    <h1>WebRTC Connection Test</h1>
    <div>
        <button onclick="startCall()">Start Call</button>
        <button onclick="endCall()">End Call</button>
    </div>
    <pre id="results"></pre>

    <script>
        let localPC, remotePC;
        const results = document.getElementById('results');

        async function startCall() {
            results.textContent = 'Starting WebRTC call...\n';

            // Local peer
            localPC = new RTCPeerConnection({
                iceServers: [
                    { urls: 'stun:localhost:3478' },
                    { urls: 'turn:localhost:3478', username: 'turnuser', credential: 'turnpassword' }
                ]
            });

            // Remote peer
            remotePC = new RTCPeerConnection({
                iceServers: [
                    { urls: 'stun:localhost:3478' },
                    { urls: 'turn:localhost:3478', username: 'turnuser', credential: 'turnpassword' }
                ]
            });

            // Track ICE candidates
            localPC.onicecandidate = (event) => {
                if (event.candidate) {
                    results.textContent += `Local ICE: ${event.candidate.type} - ${event.candidate.address}:${event.candidate.port}\n`;
                    remotePC.addIceCandidate(event.candidate);
                }
            };

            remotePC.onicecandidate = (event) => {
                if (event.candidate) {
                    results.textContent += `Remote ICE: ${event.candidate.type} - ${event.candidate.address}:${event.candidate.port}\n`;
                    localPC.addIceCandidate(event.candidate);
                }
            };

            // Track connection state
            localPC.oniceconnectionstatechange = () => {
                results.textContent += `Local ICE State: ${localPC.iceConnectionState}\n`;
            };

            remotePC.oniceconnectionstatechange = () => {
                results.textContent += `Remote ICE State: ${remotePC.iceConnectionState}\n`;
            };

            // Create data channels
            const localChannel = localPC.createDataChannel('chat');
            localChannel.onopen = () => {
                results.textContent += 'Local data channel opened\n';
                localChannel.send('Hello from local peer!');
            };

            remotePC.ondatachannel = (event) => {
                const remoteChannel = event.channel;
                remoteChannel.onopen = () => {
                    results.textContent += 'Remote data channel opened\n';
                };
                remoteChannel.onmessage = (event) => {
                    results.textContent += `Received: ${event.data}\n`;
                };
            };

            // Exchange SDP
            const localOffer = await localPC.createOffer();
            await localPC.setLocalDescription(localOffer);
            await remotePC.setRemoteDescription(localOffer);

            const remoteAnswer = await remotePC.createAnswer();
            await remotePC.setLocalDescription(remoteAnswer);
            await localPC.setRemoteDescription(remoteAnswer);
        }

        function endCall() {
            if (localPC) localPC.close();
            if (remotePC) remotePC.close();
            results.textContent += 'Call ended\n';
        }
    </script>
</body>
</html>
```

**Expected Results:**
- Both peers should gather ICE candidates
- Connection should be established
- Data channel should be opened
- Messages should be exchanged

---

## Troubleshooting

### Issue 1: Container Not Starting

**Symptoms:**
```bash
docker ps | grep turn
# (no output)
```

**Diagnosis:**
```bash
docker logs secureconnect_turn
```

**Common Causes:**
1. Port conflict - Port 3478 already in use
2. Configuration file error
3. Volume mount error

**Solutions:**
1. Check for port conflicts:
   ```bash
   netstat -tuln | grep 3478
   ```
2. Verify configuration file:
   ```bash
   docker exec secureconnect_turn cat /etc/coturn/turnserver.conf
   ```
3. Check volume mounts:
   ```bash
   docker inspect secureconnect_turn | grep -A 10 Mounts
   ```

### Issue 2: TURN Allocation Fails

**Symptoms:**
```
0: allocate sent
0: allocation failed (401)
```

**Diagnosis:**
1. Verify credentials in environment variables
2. Check TURN logs for authentication errors

**Solutions:**
1. Check environment variables:
   ```bash
   docker exec secureconnect_turn env | grep TURN
   ```
2. Verify credentials match:
   ```bash
   docker logs secureconnect_turn | grep -i "user"
   ```

### Issue 3: No ICE Candidates Generated

**Symptoms:**
- Browser console shows no ICE candidates
- WebRTC connection fails

**Diagnosis:**
1. Check browser console for errors
2. Verify TURN server is accessible from host

**Solutions:**
1. Test TURN accessibility from host:
   ```bash
   nc -zv localhost 3478
   ```
2. Check browser console for mixed content errors (if using HTTPS)
3. Verify firewall rules

### Issue 4: Relay Candidates Not Generated

**Symptoms:**
- Only `host` and `srflx` candidates appear
- No `relay` candidates

**Diagnosis:**
1. TURN authentication may be failing
2. Relay port range may be blocked

**Solutions:**
1. Verify TURN authentication:
   ```bash
   docker exec secureconnect_turn turnutils_uclient -p 3478 -v -t -u turnuser -w turnpassword
   ```
2. Check relay port range:
   ```bash
   docker exec secureconnect_turn netstat -tuln | grep 49152
   ```

### Issue 5: Connection Timeout

**Symptoms:**
- ICE state stuck at `checking`
- Connection never completes

**Diagnosis:**
1. Network connectivity issues
2. Firewall blocking TURN ports

**Solutions:**
1. Test network connectivity:
   ```bash
   docker exec secureconnect_turn ping -c 3 google.com
   ```
2. Check firewall rules:
   ```bash
   # Linux
   sudo iptables -L -n | grep 3478

   # macOS
   sudo pfctl -s rules | grep 3478

   # Windows
   netsh advfirewall firewall show rule name=all | findstr 3478
   ```

---

## Expected Results

### Validation Checklist

| Test | Expected Result | Status |
|------|----------------|--------|
| Container Running | `secureconnect_turn` container is running | ✅ |
| Port Listening | Ports 3478, 3479, 49152-65535 are open | ✅ |
| STUN Binding | STUN binding request succeeds | ✅ |
| TURN Allocation | TURN allocation succeeds | ✅ |
| TURN Relay | Data can be relayed via TURN | ✅ |
| No Errors | No errors in TURN logs | ✅ |
| ICE Configuration | Video service has correct ICE servers | ✅ |
| ICE Gathering | ICE candidates are generated | ✅ |
| Relay Candidates | `relay` candidates appear | ✅ |
| WebRTC Connection | Two peers can connect | ✅ |
| Data Channel | Data can be exchanged | ✅ |

### Performance Benchmarks (Reference)

| Metric | Expected Value | Notes |
|--------|----------------|-------|
| STUN Binding Time | < 100ms | Local network |
| TURN Allocation Time | < 200ms | Local network |
| ICE Gathering Time | < 1s | Local network |
| Connection Establishment | < 2s | Local network |
| Relay Latency | < 10ms | Local network |

---

## Advanced Testing

### 1. Load Testing

Test TURN server with multiple concurrent connections:

```bash
# Install turnutils_uclient if not available
apt-get install coturn

# Run multiple concurrent allocations
for i in {1..10}; do
    turnutils_uclient -p 3478 -v -t -u turnuser -w turnpassword &
done
```

### 2. Bandwidth Testing

Test TURN relay bandwidth:

```bash
# Use iperf3 to test bandwidth
# Terminal 1: Start TURN listener
docker exec -it secureconnect_turn turnutils_uclient -p 3478 -v -t -u turnuser -w turnpassword -L

# Terminal 2: Test bandwidth
iperf3 -c <relay-address> -p <relay-port> -t 30
```

### 3. Failover Testing

Test TURN server restart:

```bash
# Restart TURN server
docker restart secureconnect_turn

# Verify automatic reconnection
# (WebRTC should reconnect automatically)
```

---

## Conclusion

After completing all validation steps, you should have:

1. ✅ A running TURN server in Docker Desktop
2. ✅ Verified STUN and TURN functionality
3. ✅ Confirmed WebRTC connections work via TURN relay
4. ✅ Identified any configuration issues
5. ✅ Documented performance characteristics

If all tests pass, your local TURN deployment is ready for development and testing.

---

**Last Updated**: 2026-01-13
**Version**: 1.0.0
