### **1. Kiến trúc Signaling:**

**Peer-to-Peer (1-1 calls):**
- Trực tiếp giữa 2 clients
- Bandwidth thấp nhất
- Latency thấp nhất
- Không scale cho group calls

**SFU - Selective Forwarding Unit (Group calls):**
- Server forward streams giữa các participants
- Bandwidth hiệu quả hơn mesh
- CPU overhead thấp trên server
- **Recommended cho production**

**MCU - Multipoint Control Unit:**
- Server mix tất cả streams thành 1
- Bandwidth thấp nhất cho client
- CPU overhead cao trên server
- Latency cao hơn

### **2. ICE, STUN, TURN:**

```typescript
// ICE (Interactive Connectivity Establishment)
// - Tìm đường đi tốt nhất giữa 2 peers
// - Thử các candidates: host, srflx (STUN), relay (TURN)

// STUN (Session Traversal Utilities for NAT)
// - Discover public IP và port
// - Free servers available
// - Không đủ cho symmetric NAT

// TURN (Traversal Using Relays around NAT)
// - Relay traffic khi P2P không thể
// - Cần infrastructure riêng
// - Bandwidth intensive
// - Fallback cuối cùng
```

### **3. Codec Selection:**

```javascript
// Audio Codecs
Opus: {
  quality: 'Excellent',
  bitrate: '6-510 Kbps',
  usage: 'Primary choice'
}

// Video Codecs
VP8: {
  quality: 'Good',
  hardware: 'Software only',
  patent: 'Free'
}

VP9: {
  quality: 'Better than VP8',
  hardware: 'Limited support',
  patent: 'Free'
}

H.264: {
  quality: 'Good',
  hardware: 'Wide support',
  patent: 'Licensed',
  recommended: true
}
```

### **4. Adaptive Bitrate:**

```typescript
// Automatically adjust quality based on network
class AdaptiveBitrate {
  async adjustQuality(stats: RTCStatsReport) {
    const packetLoss = this.calculatePacketLoss(stats);
    const rtt = this.getRoundTripTime(stats);
    
    if (packetLoss > 5 || rtt > 300) {
      // Reduce quality
      await this.setVideoConstraints({
        width: 640,
        height: 480,
        frameRate: 15
      });
    } else if (packetLoss < 1 && rtt < 100) {
      // Increase quality
      await this.setVideoConstraints({
        width: 1280,
        height: 720,
        frameRate: 30
      });
    }
  }
}
```

### **5. Call Recording:**

```typescript
class CallRecorder {
  private mediaRecorder: MediaRecorder | null = null;
  private recordedChunks: Blob[] = [];

  async startRecording(stream: MediaStream) {
    this.mediaRecorder = new MediaRecorder(stream, {
      mimeType: 'video/webm;codecs=vp9,opus',
      videoBitsPerSecond: 2500000
    });

    this.mediaRecorder.ondataavailable = (event) => {
      if (event.data.size > 0) {
        this.recordedChunks.push(event.data);
      }
    };

    this.mediaRecorder.onstop = async () => {
      const blob = new Blob(this.recordedChunks, {
        type: 'video/webm'
      });
      await this.uploadRecording(blob);
    };

    this.mediaRecorder.start(1000); // Collect data every second
  }

  stopRecording() {
    this.mediaRecorder?.stop();
  }
}
```

### **6. Network Quality Detection:**

```typescript
class NetworkQualityDetector {
  async detectQuality(): Promise<NetworkQuality> {
    // Test download speed
    const downloadSpeed = await this.testDownloadSpeed();
    
    // Test upload speed
    const uploadSpeed = await this.testUploadSpeed();
    
    // Test latency
    const latency = await this.testLatency();
    
    if (downloadSpeed > 5 && uploadSpeed > 2 && latency < 50) {
      return 'excellent';
    } else if (downloadSpeed > 2 && uploadSpeed > 1 && latency < 150) {
      return 'good';
    } else if (downloadSpeed > 1 && uploadSpeed > 0.5 && latency < 300) {
      return 'fair';
    } else {
      return 'poor';
    }
  }
}
```

### **7. Browser Compatibility:**

```
Chrome/Edge: ✓ Full support
Firefox: ✓ Full support
Safari: ✓ Full support (iOS 11+)
Opera: ✓ Full support

Mobile:
- iOS Safari: ✓ (requires HTTPS)
- Android Chrome: ✓
- Android Firefox: ✓
```

### **8. Production Checklist:**

```
Infrastructure:
☐ Signaling server deployed (2+ instances)
☐ TURN servers deployed (2+ per region)
☐ SFU servers for group calls
☐ Load balancers configured
☐ SSL certificates installed
☐ Monitoring setup (Prometheus/Grafana)

Security:
☐ DTLS-SRTP enabled
☐ WSS for signaling
☐ Token authentication
☐ Rate limiting
☐ IP whitelisting for TURN

Features:
☐ Audio/Video toggle
☐ Screen sharing
☐ Camera switching
☐ Quality adaptation
☐ Call recording
☐ Statistics monitoring
☐ Reconnection logic

Testing:
☐ Cross-browser testing
☐ Mobile testing
☐ Network simulation (slow 3G, etc.)
☐ Load testing
☐ Security audit
```