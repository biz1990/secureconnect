# Backend WebRTC SFU Implementation Guide (Go)

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 11.1. Tổng quan

SFU (Selective Forwarding Unit) là kiến trúc nơi Server chỉ đóng vai trò "điều phối viên" chuyển tiếp luồng video/audio (RTP packets) giữa các người tham gia, thay vì xử lý/mix lại chúng (như MCU). Điều này giúp giảm tải CPU Server và tăng chất lượng video.

### Tại sao chọn Pion WebRTC?
*   **Pure Go:** Không cần cài đặt C/C++ dependencies, dễ deploy trên Docker/Alpine.
*   **Hiệu suất:** Được tối ưu hóa cho Goroutines, xử lý hàng nghìn kết nối peer connection.
*   **Low-level Control:** Cho phép tùy biến sâu logic forwarding và recording.

---

## 11.2. Kiến trúc Video Service

Video Service bao gồm 2 thành phần chính, nhưng thường được đóng gói trong 1 Go binary (hoặc 2 microservices giao tiếp qua gRPC):

1.  **Signaling Controller (WebSocket):** Đã đề cập ở file `06-websocket-signaling-protocol.md`. Nhiệm vụ trao đổi SDP/ICE.
2.  **SFU Engine (Pion):** Nơi thực sự xử lý luồng media.

---

## 11.3. Cài đặt thư viện (Dependencies)

```bash
go get github.com/pion/webrtc/v3
go get github.com/pion/interceptor
go get github.com/pion/rtcp
go get github.com/google/uuid
```

---

## 11.4. Cấu trúc thư mục SFU

```bash
internal/video/
├── sfu.go           # Quản lý logic chung của SFU
├── room.go          # Quản lý một phòng họp (Room)
├── peer.go          # Quản lý một PeerConnection (Người dùng)
└── recording.go     # Logic ghi âm (nếu Opt-out E2EE)
```

---

## 11.5. Logic Quản lý Phòng (Room Management)

Mỗi cuộc gọi (Call ID) tương ứng với một `Room`. Room chứa danh sách các `Peer` (người tham gia).

### File: `internal/video/room.go`

```go
package video

import (
    "sync"
    "github.com/google/uuid"
)

type Room struct {
    ID        string
    Peers     map[string]*Peer // map[peerID] -> Peer
    mu        sync.RWMutex
    VideoSFU  *SFUManager     // Tham chiếu đến manager
}

// Join: Người dùng tham gia phòng
func (r *Room) Join(peerID string) *Peer {
    r.mu.Lock()
    defer r.mu.Unlock()

    peer := NewPeer(peerID, r)
    r.Peers[peerID] = peer
    return peer
}

// Leave: Người dùng rời phòng
func (r *Room) Leave(peerID string) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if peer, ok := r.Peers[peerID]; ok {
        peer.Close() // Đóng WebRTC connection
        delete(r.Peers, peerID)
    }
}

// BroadcastSignal: Gửi tín hiệu signaling (SDP/ICE) cho tất cả trong phòng
// (Lưu ý: Trong kiến trúc thực tế, signaling thường qua WebSocket riêng, SFU không tự push signaling trừ khi dùng gRPC)
```

---

## 11.6. Logic Quản lý Peer (PeerConnection)

Mỗi `Peer` đại diện cho một kết nối WebRTC tới một Client.

### File: `internal/video/peer.go`

```go
package video

import (
    "errors"
    "log"
    "sync"

    "github.com/pion/webrtc/v3"
)

type Peer struct {
    ID           string
    Room         *Room
    PeerConn     *webrtc.PeerConnection
    VideoTrack   *webrtc.TrackLocalStaticRTP // Track của user này để forward cho người khác
    AudioTrack   *webrtc.TrackLocalStaticRTP
    mu           sync.Mutex
}

func NewPeer(id string, room *Room) *Peer {
    return &Peer{
        ID:   id,
        Room: room,
    }
}

// CreatePeerConnection: Khởi tạo kết nối WebRTC
func (p *Peer) CreatePeerConnection(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
    // 1. Tạo API WebEngine với các codec hỗ trợ (VP8, VP9, H264, Opus)
    m := &webrtc.MediaEngine{}
    if err := m.RegisterDefaultCodecs(); err != nil {
        return webrtc.SessionDescription{}, err
    }

    // 2. Tạo API
    api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

    // 3. Cấu hình ICE Servers (STUN/TURN)
    config := webrtc.Configuration{
        ICEServers: []webrtc.ICEServer{
            {URLs: []string{"stun:stun.l.google.com:19302"}},
            {URLs: []string{"turn:your-turn-server.com:3478"}, Username: "user", Credential: "pass"},
        },
    }

    // 4. Tạo PeerConnection
    pc, err := api.NewPeerConnection(config)
    if err != nil {
        return webrtc.SessionDescription{}, err
    }
    p.PeerConn = pc

    // 5. Xử lý ICE Candidates (Khi tìm thấy đường đi mạng)
    pc.OnICECandidate(func(i *webrtc.ICECandidate) {
        if i == nil {
            return
        }
        // Gửi ICE candidate này về Client thông qua Signaling Channel
        // Ví dụ: p.Room.VideoSFU.SendCandidate(p.ID, i.ToJSON())
    })

    // 6. Xử lý khi Peer Connection thay đổi trạng thái (Connected/Disconnected)
    pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
        log.Printf("Peer %s state: %s", p.ID, s.String())
        if s == webrtc.PeerConnectionStateDisconnected || s == webrtc.PeerConnectionStateFailed {
            p.Room.Leave(p.ID)
        }
    })

    // 7. Xử lý Tracks (Quan trọng nhất - Forwarding)
    pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
        // Khi nhận được track từ Client này (Video/Audio)
        log.Printf("Peer %s received track: %s", p.ID, track.Codec().MimeType)

        // Tạo Track Local để forward cho người khác trong phòng
        var localTrack *webrtc.TrackLocalStaticRTP
        
        if track.Kind() == webrtc.RTPCodecTypeVideo {
            // Nếu track chưa tạo thì tạo mới
            if p.VideoTrack == nil {
                p.VideoTrack, _ = webrtc.NewTrackLocalStaticRTP(track.Codec().RTPCodecCapability, "video", "pion")
            }
            localTrack = p.VideoTrack
        } else {
            if p.AudioTrack == nil {
                p.AudioTrack, _ = webrtc.NewTrackLocalStaticRTP(track.Codec().RTPCodecCapability, "audio", "pion")
            }
            localTrack = p.AudioTrack
        }

        // Logic: Ghi (Read RTP packet) -> Chuyển tiếp (Write to others)
        rtpBuf := make([]byte, 1400)
        for {
            i, readErr := track.Read(rtpBuf)
            if readErr != nil {
                return
            }
            
            // Lấy bản copy RTP packet
            packet := rtpBuf[:i]

            // Gửi cho TẤT CẢ các peer khác trong phòng
            p.Room.mu.RLock()
            for peerID, otherPeer := range p.Room.Peers {
                if peerID != p.ID {
                    otherPeer.AddTrack(localTrack, packet)
                }
            }
            p.Room.mu.RUnlock()
        }
    })

    // 8. Set Remote Description (Nhận Offer từ Client)
    if err := pc.SetRemoteDescription(offer); err != nil {
        return webrtc.SessionDescription{}, err
    }

    // 9. Create Answer
    answer, err := pc.CreateAnswer(nil)
    if err != nil {
        return webrtc.SessionDescription{}, err
    }

    // 10. Set Local Description
    if err := pc.SetLocalDescription(answer); err != nil {
        return webrtc.SessionDescription{}, err
    }

    return answer, nil
}

// AddTrack: Hàm helper để ghi packet vào track local của peer này
// (Peer này sẽ tự động read và gửi qua mạng cho Client tương ứng)
func (p *Peer) AddTrack(track *webrtc.TrackLocalStaticRTP, packet []byte) {
    if p.PeerConn == nil {
        return
    }
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // Ghi RTP packet vào track local -> WebRTC tự động gửi qua SRTP xuống Client
    if err := track.WriteRTP(packet); err != nil {
        log.Println("Error writing RTP:", err)
    }
}

func (p *Peer) Close() {
    if p.PeerConn != nil {
        p.PeerConn.Close()
    }
}
```

---

## 11.7. Tích hợp Signaling vào SFU

Khi Client gửi `offer` qua WebSocket, Controller sẽ gọi tới SFU Engine để xử lý.

### File: `internal/video/sfu.go` (Snippet)

```go
type SFUManager struct {
    Rooms map[string]*Room
    mu    sync.RWMutex
}

func NewSFUManager() *SFUManager {
    return &SFUManager{
        Rooms: make(map[string]*Room),
    }
}

// HandleOffer: Xử lý khi Client gửi Offer
func (m *SFUManager) HandleOffer(roomID string, peerID string, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
    m.mu.Lock()
    room, exists := m.Rooms[roomID]
    if !exists {
        room = &Room{ID: roomID, Peers: make(map[string]*Peer)}
        m.Rooms[roomID] = room
    }
    m.mu.Unlock()

    // Tham gia vào phòng
    peer := room.Join(peerID)

    // Tạo PeerConnection và trả về Answer
    answer, err := peer.CreatePeerConnection(offer)
    return answer, err
}

// HandleICECandidate: Xử lý khi Client gửi ICE Candidate
func (m *SFUManager) HandleICECandidate(roomID string, peerID string, candidate webrtc.ICECandidateInit) error {
    m.mu.RLock()
    room, exists := m.Rooms[roomID]
    m.mu.RUnlock()

    if !exists {
        return errors.New("room not found")
    }
    
    room.mu.RLock()
    peer, exists := room.Peers[peerID]
    room.mu.RUnlock()

    if !exists {
        return errors.New("peer not found")
    }

    // Add ICE Candidate vào PeerConnection
    return peer.PeerConn.AddICECandidate(candidate)
}
```

---

## 11.8. Tích hợp Logic HTTP Controller

### File: `internal/video/handler.go`

```go
package video

import (
    "encoding/json"
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/pion/webrtc/v3"
)

type VideoHandler struct {
    SFU *SFUManager
}

// POST /video/signal (Hoặc qua WS, nhưng ví dụ này dùng HTTP để demo đơn giản)
func (h *VideoHandler) HandleSignal(c *gin.Context) {
    var req struct {
        RoomID  string               `json:"room_id"`
        PeerID  string               `json:"peer_id"`
        SDP     webrtc.SessionDescription `json:"sdp"`
        Type    string               `json:"type"` // "offer" or "answer" (SFU chỉ nhận offer từ client)
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, err.Error())
        return
    }

    if req.Type == "offer" {
        answer, err := h.SFU.HandleOffer(req.RoomID, req.PeerID, req.SDP)
        if err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        
        c.JSON(200, gin.H{"sdp": answer})
    }
}

// POST /video/ice
func (h *VideoHandler) HandleICE(c *gin.Context) {
    var req struct {
        RoomID    string                 `json:"room_id"`
        PeerID    string                 `json:"peer_id"`
        Candidate webrtc.ICECandidateInit `json:"candidate"`
    }
    
    c.ShouldBindJSON(&req)
    
    err := h.SFU.HandleICECandidate(req.RoomID, req.PeerID, req.Candidate)
    if err != nil {
        c.JSON(404, gin.H{"error": err.Error()})
    }
    c.JSON(200, gin.H{"status": "ok"})
}
```

---

## 11.9. Logic Ghi âm/ghi hình (Recording - Opt-out E2EE)

Nếu cuộc gọi ở chế độ `is_encrypted = false` (User bật ghi âm), SFU cần đóng vai trò là một "người dùng đặc biệt" để nhận stream.

### File: `internal/video/recording.go`

```go
func (p *Peer) StartRecording(filename string) {
    // Lưu ý: Logic này chỉ hoạt động nếu Stream KHÔNG được mã hóa E2EE ở mức Insertable Streams
    // Nếu là SRTP chuẩn của WebRTC, SFU có thể decrypt nếu có DTLS keys (Nhưng rất phức tạp).
    // Cách thực tế: SFU tạo một RecordingPeer tham gia Room.
    
    // Giả sử ta đã tạo một Peer nhận stream:
    // recordingPeer := p.Room.Join("recorder_bot")
    
    // Khi recordingPeer nhận track -> Ghi ra file .webm
    // ...
}
```

**Quan trọng về bảo mật:**
*   **Nếu E2EE ON:** SFU **không thể** ghi hình. `track.Read()` chỉ ra được byte rác (ciphertext).
*   **Nếu E2EE OFF:** SFU giải mã được SRTP stream (vì nó tham gia DTLS handshake). Bạn có thể ghi stream ra file sử dụng Pion `WriterTo` hoặc thư viện `pkg/webm`.

---

## 11.10. Tối ưu hóa hiệu năng (Scalability)

1.  **Media Engine Registration:** Chỉ đăng ký codec cần thiết (ví dụ: VP8 và Opus) để giảm bộ nhớ.
2.  **Interceptor:** Sử dụng `pion/interceptor` để bật NACK (Retransmission packets), TWCC (Transport Wide Congestion Control) giúp hình ảnh mượt mà hơn khi mạng kém.
    ```go
    api := webrtc.NewAPI(
        webrtc.WithMediaEngine(m),
        webrtc.WithInterceptorRegistry(&interceptor.Registry{}),
    )
    ```
3.  **CPU Affinity:** Trên Linux, gán Goroutine xử lý Video vào các core vật lý để tránh context switching.
4.  **Multiple Instances:** Chạy nhiều Video Service instance và dùng Load Balancer để phân bổ `room_id` (ví dụ: Hash room_id -> Instance ID).

---

*Liên kết đến tài liệu tiếp theo:* `backend/ai-service-integration.md`