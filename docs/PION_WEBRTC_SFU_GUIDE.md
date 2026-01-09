# Pion WebRTC SFU Implementation Guide

## Overview
This document provides implementation guidelines for integrating Pion WebRTC SFU (Selective Forwarding Unit) into SecureConnect's Video Service.

## Architecture

### SFU vs MCU vs Mesh
**SecureConnect uses SFU**:
- **SFU (Selective Forwarding Unit)**: Server forwards streams without decoding/encoding
- ✅ Low latency
- ✅ High scalability
- ✅ Lower server CPU usage
- ✅ Better for group calls

## Dependencies

### Add to go.mod
```bash
go get github.com/pion/webrtc/v3
go get github.com/pion/rtcp
go get github.com/pion/rtp
go get github.com/pion/interceptor
```

## Basic SFU Implementation

### 1. SFU Manager

Create `internal/sfu/manager.go`:

```go
package sfu

import (
	"sync"
	
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
)

type Room struct {
	ID           uuid.UUID
	Peers        map[uuid.UUID]*Peer
	mu           sync.RWMutex
}

type Peer struct {
	ID             uuid.UUID
	PeerConnection *webrtc.PeerConnection
	Tracks         []*webrtc.TrackLocalStaticRTP
}

type Manager struct {
	rooms map[uuid.UUID]*Room
	mu    sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		rooms: make(map[uuid.UUID]*Room),
	}
}

func (m *Manager) CreateRoom(roomID uuid.UUID) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	room := &Room{
		ID:    roomID,
		Peers: make(map[uuid.UUID]*Peer),
	}
	
	m.rooms[roomID] = room
	return room
}

func (m *Manager) GetRoom(roomID uuid.UUID) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rooms[roomID]
}
```

### 2. WebRTC Configuration

Create `internal/sfu/config.go`:

```go
package sfu

import (
	"github.com/pion/webrtc/v3"
)

func GetWebRTCConfig() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
			// TURN servers (see TURN integration section)
			{
				URLs:       []string{"turn:turn.secureconnect.io:3478"},
				Username:   "username",
				Credential: "password",
			},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}
}
```

### 3. Peer Connection Handler

Create `internal/sfu/peer.go`:

```go
package sfu

import (
	"fmt"
	"io"
	"log"
	
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
)

func (r *Room) AddPeer(peerID uuid.UUID) (*Peer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Create PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(GetWebRTCConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	
	peer := &Peer{
		ID:             peerID,
		PeerConnection: peerConnection,
		Tracks:         make([]*webrtc.TrackLocalStaticRTP, 0),
	}
	
	// Handle incoming tracks
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Received track: %s", track.ID())
		
		// Forward track to other peers
		r.forwardTrack(track, peerID)
	})
	
	// Handle ICE connection state
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("Peer %s ICE state: %s", peerID, state.String())
		
		if state == webrtc.ICEConnectionStateFailed ||
		   state == webrtc.ICEConnectionStateClosed {
			r.RemovePeer(peerID)
		}
	})
	
	r.Peers[peerID] = peer
	return peer, nil
}

func (r *Room) forwardTrack(track *webrtc.TrackRemote, senderID uuid.UUID) {
	// Create local track
	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		track.Codec().RTPCodecCapability,
		track.ID(),
		track.StreamID(),
	)
	if err != nil {
		log.Printf("Failed to create local track: %v", err)
		return
	}
	
	// Read from incoming track and write to local track
	go func() {
		buf := make([]byte, 1500)
		for {
			i, _, err := track.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				log.Printf("Track read error: %v", err)
				return
			}
			
			if _, err = localTrack.Write(buf[:i]); err != nil {
				log.Printf("Track write error: %v", err)
				return
			}
		}
	}()
	
	// Add track to all other peers
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for peerID, peer := range r.Peers {
		if peerID != senderID {
			_, err := peer.PeerConnection.AddTrack(localTrack)
			if err != nil {
				log.Printf("Failed to add track to peer %s: %v", peerID, err)
			}
		}
	}
}

func (r *Room) RemovePeer(peerID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if peer, exists := r.Peers[peerID]; exists {
		peer.PeerConnection.Close()
		delete(r.Peers, peerID)
	}
}
```

### 4. Integration with Video Service

Update `internal/service/video/service.go`:

```go
type Service struct {
	callRepo *cockroach.CallRepository
	sfuManager *sfu.Manager
}

func NewService(callRepo *cockroach.CallRepository) *Service {
	return &Service{
		callRepo:   callRepo,
		sfuManager: sfu.NewManager(),
	}
}

func (s *Service) InitiateCall(ctx context.Context, input *InitiateCallInput) (*InitiateCallOutput, error) {
	// ... existing code ...
	
	// Create SFU room
	s.sfuManager.CreateRoom(callID)
	
	return output, nil
}
```

### 5. Signaling Integration

Update `internal/handler/ws/signaling_handler.go` to handle WebRTC signaling:

```go
func (h *SignalingHub) handleOffer(msg *SignalingMessage, client *SignalingClient) {
	// Get SFU room
	room := h.sfuManager.GetRoom(msg.CallID)
	if room == nil {
		log.Printf("Room not found: %s", msg.CallID)
		return
	}
	
	// Get or create peer
	peer, err := room.AddPeer(client.userID)
	if err != nil {
		log.Printf("Failed to add peer: %v", err)
		return
	}
	
	// Set remote description
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  msg.SDP,
	}
	
	if err := peer.PeerConnection.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to set remote description: %v", err)
		return
	}
	
	// Create answer
	answer, err := peer.PeerConnection.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create answer: %v", err)
		return
	}
	
	if err := peer.PeerConnection.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		return
	}
	
	// Send answer back to client
	h.broadcast <- &SignalingMessage{
		Type:      SignalTypeAnswer,
		CallID:    msg.CallID,
		SenderID:  client.userID,
		TargetID:  msg.SenderID,
		SDP:       answer.SDP,
		Timestamp: time.Now(),
	}
}
```

## TURN Server Integration

### Why TURN?
- Required for peer connections behind symmetric NAT
- ~10-20% of users need TURN
- Improves connection success rate from ~85% to ~99%

### Popular TURN Servers
1. **coturn** (recommended)
   - Open source
   - Production-ready
   - Easy to deploy

2. **Cloudflare TURN**
   - Managed service
   - Global CDN

### coturn Setup

#### Docker Deployment
```yaml
# docker-compose.yml
coturn:
  image: coturn/coturn:latest
  ports:
    - "3478:3478/tcp"
    - "3478:3478/udp"
    - "5349:5349/tcp"
    - "5349:5349/udp"
    - "49152-65535:49152-65535/udp"
  environment:
    - REALM=turn.secureconnect.io
    - LISTEN_ON_IP=0.0.0.0
    - EXTERNAL_IP=YOUR_PUBLIC_IP
  volumes:
    - ./turnserver.conf:/etc/coturn/turnserver.conf
```

#### turnserver.conf
```ini
listening-port=3478
tls-listening-port=5349
listening-ip=0.0.0.0

realm=turn.secureconnect.io
server-name=turn.secureconnect.io

# Authentication
lt-cred-mech
user=username:password

# Fingerprinting
fingerprint

# Security
no-multicast-peers
no-loopback-peers

# Logging
log-file=/var/log/turnserver.log
```

### Dynamic TURN Credentials

Generate temporary credentials:

```go
package sfu

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"time"
)

func GenerateTURNCredentials(username string, secret string) (string, string) {
	// Expiry timestamp (24 hours from now)
	expiryTime := time.Now().Add(24 * time.Hour).Unix()
	
	// Username format: timestamp:username
	tempUsername := fmt.Sprintf("%d:%s", expiryTime, username)
	
	// Generate password using HMAC-SHA1
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(tempUsername))
	password := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	
	return tempUsername, password
}
```

## Call Recording

### Architecture
1. **SFU records streams** to disk
2. **FFmpeg converts** to final format
3. **Upload to MinIO** for storage
4. **Update call record** with recording URL

### Basic Recording Implementation

```go
package recording

import (
	"fmt"
	"io"
	"os"
	
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

type Recorder struct {
	audioWriter *oggwriter.OggWriter
	videoFile   *os.File
}

func NewRecorder(callID string) (*Recorder, error) {
	audioPath := fmt.Sprintf("/tmp/recordings/%s_audio.ogg", callID)
	videoPath := fmt.Sprintf("/tmp/recordings/%s_video.ivf", callID)
	
	audioWriter, err := oggwriter.New(audioPath, 48000, 2)
	if err != nil {
		return nil, err
	}
	
	videoFile, err := os.Create(videoPath)
	if err != nil {
		return nil, err
	}
	
	return &Recorder{
		audioWriter: audioWriter,
		videoFile:   videoFile,
	}, nil
}

func (r *Recorder) RecordTrack(track *webrtc.TrackRemote) {
	go func() {
		buf := make([]byte, 1500)
		for {
			i, _, err := track.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				return
			}
			
			// Write to appropriate writer based on track kind
			if track.Kind() == webrtc.RTPCodecTypeAudio {
				r.audioWriter.WriteRTP(&rtp.Packet{Payload: buf[:i]})
			} else if track.Kind() == webrtc.RTPCodecTypeVideo {
				r.videoFile.Write(buf[:i])
			}
		}
	}()
}

func (r *Recorder) Stop() error {
	if err := r.audioWriter.Close(); err != nil {
		return err
	}
	return r.videoFile.Close()
}
```

### Post-processing with FFmpeg

```go
func ConvertRecording(callID string) error {
	audioPath := fmt.Sprintf("/tmp/recordings/%s_audio.ogg", callID)
	videoPath := fmt.Sprintf("/tmp/recordings/%s_video.ivf", callID)
	outputPath := fmt.Sprintf("/tmp/recordings/%s_final.mp4", callID)
	
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-i", audioPath,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-strict", "experimental",
		outputPath,
	)
	
	return cmd.Run()
}
```

## Testing

### Manual Testing with Browser

```javascript
// Client-side WebRTC code
const pc = new RTCPeerConnection({
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    {
      urls: 'turn:turn.secureconnect.io:3478',
      username: 'user',
      credential: 'pass'
    }
  ]
});

// Get local stream
const stream = await navigator.mediaDevices.getUserMedia({
  video: true,
  audio: true
});

stream.getTracks().forEach(track => {
  pc.addTrack(track, stream);
});

// Create offer
const offer = await pc.createOffer();
await pc.setLocalDescription(offer);

// Send offer via WebSocket
ws.send(JSON.stringify({
  type: 'offer',
  sdp: offer.sdp
}));
```

## Performance Optimization

### 1. Simulcast
Enable multiple quality streams:

```go
pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
	webrtc.RtpTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
		SendEncodings: []webrtc.RTPEncodingParameters{
			{RID: "high", MaxBitrate: 1000000},
			{RID: "mid", MaxBitrate: 500000, ScaleResolutionDownBy: 2},
			{RID: "low", MaxBitrate: 150000, ScaleResolutionDownBy: 4},
		},
	},
)
```

### 2. Bandwidth Estimation

```go
import "github.com/pion/interceptor/pkg/twcc"

pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	// Enable TWCC for bandwidth estimation
	if err := pc.WriteRTCP([]rtcp.Packet{
		&rtcp.TransportLayerNack{},
	}); err != nil {
		log.Printf("RTCP write error: %v", err)
	}
})
```

## Monitoring

### Metrics to Track
- Active calls
- Peer count per room
- Bandwidth usage per peer
- Packet loss rate
- Round-trip time (RTT)
- Connection success rate

```go
type Metrics struct {
	ActiveCalls     int64
	TotalPeers      int64
	BytesSent       int64
	BytesReceived   int64
	PacketLoss      float64
	AverageRTT      time.Duration
}
```

## Security Considerations

1. **DTLS encryption** - Enabled by default in Pion
2. **SRTP for media** - Automatic
3. **Authentication** - Verify JWT before WebRTC handshake
4. **Rate limiting** - Limit calls per user
5. **TURN authentication** - Use temporary credentials

## Next Steps

1. Implement basic SFU manager
2. Test with 2-person call
3. Add simulcast support
4. Deploy TURN server
5. Implement recording
6. Performance testing
7. Production deployment

## Resources

- [Pion WebRTC Documentation](https://github.com/pion/webrtc)
- [WebRTC for the Curious](https://webrtcforthecurious.com/)
- [coturn Documentation](https://github.com/coturn/coturn)
