# Backend E2EE Implementation Guide (Go)

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 10.1. Tổng quan về vai trò Backend trong E2EE

Trong kiến trúc **Signal Protocol**, Backend (Go) **không tham gia** vào quá trình mã hóa hay giải mã tin nhắn. Vai trò của Backend là:

1.  **Public Key Directory:** Lưu trữ khóa công khai (Identity Keys, Pre-keys) của người dùng.
2.  **Key Exchange Broker:** Cung cấp khóa công khai của User B cho User A để thiết lập phiên chat.
3.  **Key Lifecycle Manager:** Quản lý việc làm mới (rotate) các khóa trung hạn.

### Các thuật toán sử dụng
*   **Identity Key:** Ed25519 (Dùng để ký)
*   **Ephemeral Keys:** X25519 (Dùng để trao đổi khóa)
*   **Signature:** Ed25519 (Ký xác nhận Pre-keys)

---

## 10.2. Thư viện hỗ trợ trong Go

Chúng ta sử dụng thư viện thuần Go để xử lý Crypto, không phụ thuộc vào C (cgo) để dễ biên dịch cross-platform.

```bash
go get golang.org/x/crypto/curve25519
go get golang.org/x/crypto/ed25519
go get github.com/google/uuid
go get github.com/lib/pq  # Driver CockroachDB/Postgres
```

---

## 10.3. Data Models (Kiến trúc dữ liệu khóa)

Định nghĩa các cấu trúc dữ liệu đại diện cho các loại khóa trong hệ thống.

### File: `internal/crypto/models.go`

```go
package crypto

import (
    "encoding/base64"
    "errors"
    "time"
)

// IdentityKey đại diện cho khóa danh tính dài hạn của User
type IdentityKey struct {
    UserID    string
    PublicKey []byte // Ed25519 Public Key (32 bytes)
}

// SignedPreKey là khóa được ký bởi IdentityKey, dùng trung hạn (tuần)
type SignedPreKey struct {
    KeyID     int
    UserID    string
    PublicKey []byte // X25519 Public Key
    Signature []byte // Chữ ký bởi IdentityKey
    Timestamp time.Time
}

// OneTimePreKey là khóa dùng một lần để tăng tính bảo mật lần kết nối đầu
type OneTimePreKey struct {
    KeyID     int
    UserID    string
    PublicKey []byte // X25519 Public Key
    Used      bool
    CreatedAt time.Time
}

// KeyBundle là gói khóa mà Server trả về cho Client A khi A muốn nhắn tin cho B
type KeyBundle struct {
    IdentityKey      string `json:"identity_key"`      // Base64
    SignedPreKey     SignedPreKeyJSON `json:"signed_pre_key"`
    OneTimePreKey    *OneTimePreKeyJSON `json:"one_time_pre_key"` // Nullable
}

// Helper structs cho JSON serialization
type SignedPreKeyJSON struct {
    KeyID     int    `json:"key_id"`
    PublicKey string `json:"public_key"`
    Signature string `json:"signature"`
}

type OneTimePreKeyJSON struct {
    KeyID     int    `json:"key_id"`
    PublicKey string `json:"public_key"`
}

// Helper: Chuyển []byte thành String (Base64)
func BytesToString(b []byte) string {
    return base64.StdEncoding.EncodeToString(b)
}

// Helper: Chuyển String thành []byte
func StringToBytes(s string) ([]byte, error) {
    return base64.StdEncoding.DecodeString(s)
}
```

---

## 10.4. Database Implementation (PostgreSQL/CockroachDB)

Đây là phần Repository để lưu trữ khóa vào Database.

### File: `internal/crypto/repository.go`

```go
package crypto

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"
)

type KeyRepository interface {
    SaveIdentityKey(ctx context.Context, userID string, publicKey []byte) error
    SaveSignedPreKey(ctx context.Context, key *SignedPreKey) error
    SaveOneTimePreKeys(ctx context.Context, keys []*OneTimePreKey) error
    GetKeyBundle(ctx context.Context, userID string) (*KeyBundle, error)
    MarkOneTimePreKeyUsed(ctx context.Context, userID string, keyID int) error
}

type PostgresKeyRepository struct {
    db *sql.DB
}

func NewPostgresKeyRepository(db *sql.DB) *PostgresKeyRepository {
    return &PostgresKeyRepository{db: db}
}

// SaveIdentityKey (Upsert): Ghi đè nếu đã tồn tại
func (r *PostgresKeyRepository) SaveIdentityKey(ctx context.Context, userID string, publicKey []byte) error {
    query := `
        INSERT INTO identity_keys (user_id, public_key)
        VALUES ($1, $2)
        ON CONFLICT (user_id) DO UPDATE SET public_key = EXCLUDED.public_key
    `
    _, err := r.db.ExecContext(ctx, query, userID, publicKey)
    return err
}

// SaveSignedPreKey: Lưu khóa Signed
func (r *PostgresKeyRepository) SaveSignedPreKey(ctx context.Context, key *SignedPreKey) error {
    query := `
        INSERT INTO signed_pre_keys (key_id, user_id, public_key, signature, timestamp)
        VALUES ($1, $2, $3, $4, $5)
    `
    _, err := r.db.ExecContext(ctx, query, key.KeyID, key.UserID, key.PublicKey, key.Signature, key.Timestamp)
    return err
}

// SaveOneTimePreKeys: Lưu hàng loạt One-time keys
func (r *PostgresKeyRepository) SaveOneTimePreKeys(ctx context.Context, keys []*OneTimePreKey) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    stmt, err := tx.Prepare(`
        INSERT INTO one_time_pre_keys (key_id, user_id, public_key, used, created_at)
        VALUES ($1, $2, $3, false, NOW())
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, key := range keys {
        if _, err := stmt.ExecContext(ctx, key.KeyID, key.UserID, key.PublicKey); err != nil {
            return err
        }
    }

    return tx.Commit()
}

// GetKeyBundle: Lấy đầy đủ khóa để gửi cho người khác
func (r *PostgresKeyRepository) GetKeyBundle(ctx context.Context, userID string) (*KeyBundle, error) {
    var bundle KeyBundle
    var identityKeyBytes []byte
    var sigKeyID int
    var sigPubKey []byte
    var sigSignature []byte
    var sigTS time.Time

    // 1. Lấy Identity Key
    err := r.db.QueryRowContext(ctx, "SELECT public_key FROM identity_keys WHERE user_id = $1", userID).
        Scan(&identityKeyBytes)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, fmt.Errorf("identity key not found")
        }
        return nil, err
    }
    bundle.IdentityKey = BytesToString(identityKeyBytes)

    // 2. Lấy Signed Pre-Key mới nhất
    err = r.db.QueryRowContext(ctx, `
        SELECT key_id, public_key, signature, timestamp 
        FROM signed_pre_keys 
        WHERE user_id = $1 
        ORDER BY timestamp DESC 
        LIMIT 1
    `, userID).Scan(&sigKeyID, &sigPubKey, &sigSignature, &sigTS)
    
    if err != nil {
        return nil, fmt.Errorf("signed pre-key not found")
    }
    bundle.SignedPreKey = SignedPreKeyJSON{
        KeyID:     sigKeyID,
        PublicKey: BytesToString(sigPubKey),
        Signature: BytesToString(sigSignature),
    }

    // 3. Lấy một One-Time Pre-Key chưa dùng
    var otpKeyID int
    var otpPubKey []byte
    err = r.db.QueryRowContext(ctx, `
        SELECT key_id, public_key 
        FROM one_time_pre_keys 
        WHERE user_id = $1 AND used = false 
        LIMIT 1
        FOR UPDATE SKIP LOCKED -- Quan trọng để tránh race condition khi nhiều người request cùng lúc
    `, userID).Scan(&otpKeyID, &otpPubKey)

    if err == nil {
        bundle.OneTimePreKey = &OneTimePreKeyJSON{
            KeyID:     otpKeyID,
            PublicKey: BytesToString(otpPubKey),
        }
        // Đánh dấu đã dùng ngay lập tức để người khác không lấy trùng
        r.MarkOneTimePreKeyUsed(ctx, userID, otpKeyID)
    }

    return &bundle, nil
}

// MarkOneTimePreKeyUsed: Đánh dấu khóa đã bị dùng
func (r *PostgresKeyRepository) MarkOneTimePreKeyUsed(ctx context.Context, userID string, keyID int) error {
    _, err := r.db.ExecContext(ctx, `
        UPDATE one_time_pre_keys 
        SET used = true 
        WHERE user_id = $1 AND key_id = $2
    `, userID, keyID)
    return err
}
```

---

## 10.5. API Handlers (Xử lý Request)

Dưới đây là cách triển khai các API Endpoint để giao tiếp với Flutter.

### File: `internal/crypto/handler.go`

```go
package crypto

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "golang.org/x/crypto/ed25519" // Để verify signature
)

type KeyHandler struct {
    repo KeyRepository
}

func NewKeyHandler(repo KeyRepository) *KeyHandler {
    return &KeyHandler{repo: repo}
}

// Request DTO
type UploadKeysRequest struct {
    IdentityKey    string                `json:"identity_key" binding:"required"`
    SignedPreKey   SignedPreKeyJSON      `json:"signed_pre_key" binding:"required"`
    OneTimePreKeys []OneTimePreKeyJSON   `json:"one_time_pre_keys" binding:"required"`
}

// POST /keys/upload
func (h *KeyHandler) UploadKeys(c *gin.Context) {
    userID := c.GetString("user_id") // Lấy từ JWT Middleware
    if userID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    var req UploadKeysRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 1. Decode Base64 strings to Bytes
    identityKey, _ := StringToBytes(req.IdentityKey)
    signedPubKey, _ := StringToBytes(req.SignedPreKey.PublicKey)
    signature, _ := StringToBytes(req.SignedPreKey.Signature)

    // 2. VERIFY SIGNATURE (Quan trọng)
    // Server cần kiểm tra xem SignedPreKey có thực sự được ký bởi IdentityKey của User này không
    if !ed25519.Verify(identityKey, signedPubKey, signature) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature on signed pre-key"})
        return
    }

    // 3. Lưu Identity Key
    if err := h.repo.SaveIdentityKey(c.Request.Context(), userID, identityKey); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save identity key"})
        return
    }

    // 4. Lưu Signed Pre-Key
    spk := &SignedPreKey{
        KeyID:     req.SignedPreKey.KeyID,
        UserID:    userID,
        PublicKey: signedPubKey,
        Signature: signature,
        Timestamp: time.Now(),
    }
    if err := h.repo.SaveSignedPreKey(c.Request.Context(), spk); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save signed pre-key"})
        return
    }

    // 5. Lưu One-Time Pre-Keys (Batch)
    var otps []*OneTimePreKey
    for _, k := range req.OneTimePreKeys {
        pub, _ := StringToBytes(k.PublicKey)
        otps = append(otps, &OneTimePreKey{
            KeyID:     k.KeyID,
            UserID:    userID,
            PublicKey: pub,
            CreatedAt: time.Now(),
        })
    }
    if len(otps) > 0 {
        if err := h.repo.SaveOneTimePreKeys(c.Request.Context(), otps); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save one-time pre-keys"})
            return
        }
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "message": "keys uploaded successfully"})
}

// GET /keys/:user_id
func (h *KeyHandler) GetKeys(c *gin.Context) {
    targetUserID := c.Param("user_id")
    
    // Kiểm tra xem user target có tồn tại không (Optimization: check Redis Directory trước)
    // ...

    bundle, err := h.repo.GetKeyBundle(c.Request.Context(), targetUserID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "keys not found"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "data":    bundle,
    })
}
```

---

## 10.6. Tích hợp vào Service Layer

Trong `cmd/chat-service/main.go` hoặc `internal/chat/handler.go`, bạn sẽ tích hợp `KeyHandler`.

```go
package main

import (
    "myproject/internal/crypto"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()
    
    // Init DB
    db := initDB() // Kết nối CockroachDB
    
    // Init Repos & Handlers
    keyRepo := crypto.NewPostgresKeyRepository(db)
    keyHandler := crypto.NewKeyHandler(keyRepo)
    
    // Routes
    v1 := r.Group("/v1")
    v1.Use(AuthMiddleware()) // Giả định bạn có middleware này
    
    keysGroup := v1.Group("/keys")
    {
        keysGroup.POST("/upload", keyHandler.UploadKeys)
        keysGroup.GET("/:user_id", keyHandler.GetKeys)
    }
    
    r.Run(":8080")
}
```

---

## 10.7. Chiến lược Rotate Keys (Tự động)

Việc thay thế khóa là bắt buộc để bảo mật an toàn. Backend nên có một cron job để kiểm tra và thông báo cho client nếu thiếu keys.

### Cron Job Logic (Ví dụ):
```go
func CheckAndNotifyReplenishment(repo KeyRepository, userID string) {
    // 1. Đếm số One-Time Pre-Key còn lại
    count, _ := repo.CountRemainingOneTimePreKeys(userID)
    
    // 2. Nếu < 20, gửi sự kiện "keys_low" qua WebSocket hoặc Firebase Push
    if count < 20 {
        SendPushNotification(userID, "Please replenish your encryption keys")
    }
}

// Signed Pre-Key Rotation: Client chịu trách nhiệm upload mới hàng tuần.
// Server chỉ cần lưu trữ, nhưng nên xóa các khóa quá cũ (>30 ngày) để tiết kiệm DB.
```

---

## 10.8. Bảo mật

1.  **Verify Signature:** **BẮT BUỘC** phải xác minh chữ ký của `SignedPreKey` bằng `IdentityKey` trước khi lưu vào DB. Điều này ngăn chặn việc kẻ tấn công thay thế khóa công khai của người khác (Man-in-the-Middle).
2.  **TLS:** Mọi API Key Management phải chạy qua HTTPS.
3.  **Rate Limiting:** Giới hạn số lần `GET /keys/:user_id` để quét dữ liệu người dùng (Scraping).

---

*Liên kết đến tài liệu tiếp theo:* `backend/webrtc-sfu-guide.md`