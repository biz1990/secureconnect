# SecureConnect Backend - Comprehensive Code Audit Report

**Audit Date:** 2026-01-10  
**Auditor:** Senior Software Architect  
**Scope:** Complete codebase review

---

## 1. Project Overview (from actual code)

### Detected Architecture Style
- **Microservices Architecture** with Clean Architecture principles
- **5 Independent Services:** API Gateway, Auth Service, Chat Service, Video Service, Storage Service
- **Layered Design:** Domain → Repository → Service → Handler

### Main Modules and Responsibilities

| Module | Responsibility | Port |
|--------|---------------|-------|
| `api-gateway` | Reverse proxy, routing, rate limiting | 8080 |
| `auth-service` | User registration, login, JWT management, E2EE keys | 8081 |
| `chat-service` | Real-time messaging, WebSocket, presence | 8082 |
| `video-service` | WebRTC signaling, call management | 8083 |
| `storage-service` | File upload/download via MinIO | 8084 |

### Key Data Flows

1. **Authentication Flow:** Client → API Gateway → Auth Service → CockroachDB + Redis
2. **Messaging Flow:** Client → WebSocket → Chat Service → Cassandra (messages) + Redis (pub/sub)
3. **Video Call Flow:** Client → WebSocket → Video Service → CockroachDB (call logs) + Redis (signaling)
4. **File Storage Flow:** Client → Storage Service → MinIO (presigned URLs) + CockroachDB (metadata)

---

## 2. Critical Errors (Must Fix)

### Error 1: Missing Database Config Structs

**File:** `secureconnect-backend/cmd/auth-service/main.go:38-45`  
**File:** `secureconnect-backend/cmd/storage-service/main.go:29-36`

**Original Code:**
```go
crdbConfig := &database.CockroachConfig{
    Host:     getEnv("DB_HOST", "localhost"),
    Port:     26257,
    User:     getEnv("DB_USER", "root"),
    Password: getEnv("DB_PASSWORD", ""),
    Database: getEnv("DB_NAME", "secureconnect_poc"),
    SSLMode:  "disable",
}

redisConfig := &database.RedisConfig{
    Host:     getEnv("REDIS_HOST", "localhost"),
    Port:     6379,
    Password: getEnv("REDIS_PASSWORD", ""),
    DB:       0,
    PoolSize: 10,
    Timeout:  5 * time.Second,
}
```

**Explanation:** The code references `database.CockroachConfig` and `database.RedisConfig` types that do NOT exist in the `database` package. The actual config types are in `pkg/config/config.go`. This will cause a **compilation error**.

**Fixed Code:**
```go
// Use pkg/config package instead
cfg, err := config.Load()
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Then use cfg.Database, cfg.Redis, etc.
```

---

### Error 2: Database Connection Function Mismatch

**File:** `secureconnect-backend/cmd/auth-service/main.go:47`

**Original Code:**
```go
crdb, err := database.NewCockroachDB(ctx, crdbConfig)
```

**Explanation:** The `NewCockroachDB` function in [`database/cockroachdb.go`](secureconnect-backend/internal/database/cockroachdb.go:15) expects a `connString` parameter, not a config struct.

**Fixed Code:**
```go
// Build connection string
connString := fmt.Sprintf(
    "postgresql://%s:%s@%s:%d/%s?sslmode=%s",
    cfg.Database.User,
    cfg.Database.Password,
    cfg.Database.Host,
    cfg.Database.Port,
    cfg.Database.Database,
    cfg.Database.SSLMode,
)

crdb, err := database.NewCockroachDB(ctx, connString)
```

---

### Error 3: Missing Conversation Title in INSERT

**File:** `secureconnect-backend/internal/repository/cockroach/conversation_repo.go:26-46`

**Original Code:**
```go
func (r *ConversationRepository) Create(ctx context.Context, conversation *domain.Conversation) error {
    query := `
        INSERT INTO conversations (
            conversation_id, title, type, created_by, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6)
    `
    
    _, err := r.pool.Exec(ctx, query,
        conversation.ConversationID,
        conversation.Title,  // BUG: Title field is passed but domain model doesn't have it
        conversation.Type,
        conversation.CreatedBy,
        conversation.CreatedAt,
        conversation.UpdatedAt,
    )
```

**Explanation:** The domain model [`Conversation`](secureconnect-backend/internal/domain/conversation.go:9-20) has `Title` field but INSERT query uses `title` (lowercase). Additionally, the schema has both `title` and `name` fields but domain only has `Title`. This causes **column mismatch**.

**Fixed Code:**
```go
// In domain/conversation.go - Update struct
type Conversation struct {
    ConversationID uuid.UUID `json:"conversation_id" db:"conversation_id"`
    Type           string    `json:"type" db:"type"`
    Title          string    `json:"title,omitempty" db:"title"`  // Optional for direct chats
    Name           *string   `json:"name,omitempty" db:"name"`   // For group chats
    AvatarURL      *string   `json:"avatar_url,omitempty" db:"avatar_url"`
    CreatedBy      uuid.UUID `json:"created_by" db:"created_by"`
    CreatedAt      time.Time `json:"created_at" db:"created_at"`
    UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// In conversation_repo.go - Update Create method
func (r *ConversationRepository) Create(ctx context.Context, conversation *domain.Conversation) error {
    query := `
        INSERT INTO conversations (
            conversation_id, type, title, name, created_by, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7)
    `
    
    _, err := r.pool.Exec(ctx, query,
        conversation.ConversationID,
        conversation.Type,
        conversation.Title,
        conversation.Name,
        conversation.CreatedBy,
        conversation.CreatedAt,
        conversation.UpdatedAt,
    )
```

---

### Error 4: Invalid UUID Parsing in GetConversations

**File:** `secureconnect-backend/internal/handler/http/conversation/handler.go:103-107`

**Original Code:**
```go
if limitStr := c.Query("limit"); limitStr != "" {
    if l, err := uuid.Parse(limitStr); err == nil {
        _ = l // Just to avoid unused var
    }
}
```

**Explanation:** The code tries to parse a numeric `limit` parameter as a **UUID**, which will always fail. This causes limit to always default to 20.

**Fixed Code:**
```go
if limitStr := c.Query("limit"); limitStr != "" {
    if l, err := strconv.Atoi(limitStr); err == nil {
        limit = l
    }
}
```

---

### Error 5: Missing Session Validation in Logout

**File:** `secureconnect-backend/internal/service/auth/service.go:278-302`

**Original Code:**
```go
func (s *Service) Logout(ctx context.Context, sessionID string, userID uuid.UUID, tokenString string) error {
    // 1. Delete session
    if err := s.sessionRepo.DeleteSession(ctx, sessionID, userID); err != nil {
        return fmt.Errorf("failed to delete session: %w", err)
    }
```

**Explanation:** The `Logout` function accepts a `sessionID` parameter but **never validates** that the session belongs to the user. A malicious user could provide any session ID and log out other users.

**Fixed Code:**
```go
func (s *Service) Logout(ctx context.Context, sessionID string, userID uuid.UUID, tokenString string) error {
    // 1. Validate session belongs to user
    session, err := s.sessionRepo.GetSession(ctx, sessionID)
    if err != nil {
        return fmt.Errorf("session not found: %w", err)
    }
    if session.UserID != userID {
        return fmt.Errorf("unauthorized: session does not belong to user")
    }

    // 2. Delete session
    if err := s.sessionRepo.DeleteSession(ctx, sessionID, userID); err != nil {
        return fmt.Errorf("failed to delete session: %w", err)
    }
```

---

### Error 6: CORS Allows All Origins

**File:** `secureconnect-backend/internal/middleware/cors.go:7-21`

**Original Code:**
```go
func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")  // CRITICAL SECURITY ISSUE
        c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    }
}
```

**Explanation:** Setting `Access-Control-Allow-Origin: *` with `Access-Control-Allow-Credentials: true` is **invalid and insecure**. Browsers will reject this combination, and it allows any origin to make requests.

**Fixed Code:**
```go
func CORSMiddleware() gin.HandlerFunc {
    allowedOrigins := map[string]bool{
        "http://localhost:3000": true,
        "http://localhost:8080": true,
        // Add production domains
    }

    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        if allowedOrigins[origin] {
            c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
            c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        }
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Writer.Header().Set("Access-Control-Max-Age", "86400")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    }
}
```

---

### Error 7: JWT Secret Not Validated in Production

**File:** `secureconnect-backend/cmd/auth-service/main.go:25-35`

**Original Code:**
```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    jwtSecret = "super-secret-key-change-in-production"  // INSECURE DEFAULT
}

jwtManager := jwt.NewJWTManager(
    jwtSecret,
    15*time.Minute,
    30*24*time.Hour,
)
```

**Explanation:** The code uses a weak default JWT secret when the environment variable is not set. This is a **critical security vulnerability** that allows attackers to forge JWT tokens.

**Fixed Code:**
```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    log.Fatal("JWT_SECRET environment variable is required")
}
if len(jwtSecret) < 32 {
    log.Fatal("JWT_SECRET must be at least 32 characters")
}

jwtManager := jwt.NewJWTManager(
    jwtSecret,
    15*time.Minute,
    30*24*time.Hour,
)
```

---

### Error 8: Missing Authorization Check in File Download

**File:** `secureconnect-backend/internal/service/storage/service.go:153-173`

**Original Code:**
```go
func (s *Service) GenerateDownloadURL(ctx context.Context, userID, fileID uuid.UUID) (string, error) {
    // Fetch file metadata from CockroachDB
    file, err := s.fileRepo.GetByID(ctx, fileID)
    if err != nil {
        return "", fmt.Errorf("file not found: %w", err)
    }

    // Verify user owns file
    if file.UserID != userID {
        return "", fmt.Errorf("unauthorized access to file")
    }
```

**Explanation:** The authorization check only verifies if the requesting user owns the file. It **doesn't check** if the user has permission to access files shared with them (e.g., in conversations).

**Fixed Code:**
```go
func (s *Service) GenerateDownloadURL(ctx context.Context, userID, fileID uuid.UUID) (string, error) {
    // Fetch file metadata from CockroachDB
    file, err := s.fileRepo.GetByID(ctx, fileID)
    if err != nil {
        return "", fmt.Errorf("file not found: %w", err)
    }

    // Check if user owns file OR has access via conversation
    if file.UserID != userID {
        // Check if file is shared with user via conversation
        hasAccess, err := s.fileRepo.CheckFileAccess(ctx, fileID, userID)
        if err != nil {
            return "", fmt.Errorf("failed to check file access: %w", err)
        }
        if !hasAccess {
            return "", fmt.Errorf("unauthorized access to file")
        }
    }

    // Generate presigned download URL (valid for 1 hour)
    presignedURL, err := s.storage.PresignedGetObject(ctx, s.bucketName, file.MinIOObjectKey, time.Hour, nil)
    if err != nil {
        return "", fmt.Errorf("failed to generate download URL: %w", err)
    }

    return presignedURL.String(), nil
}
```

---

### Error 9: Redis Client Not Closed in Video Service

**File:** `secureconnect-backend/cmd/video-service/main.go:64-75`

**Original Code:**
```go
redisClient := redis.NewClient(&redis.Options{
    Addr:     redisAddr,
    Password: getEnv("REDIS_PASSWORD", ""),
    DB:       0,
})

// Check Redis connection
if err := redisClient.Ping(ctx).Err(); err != nil {
    log.Printf("Warning: Failed to connect to Redis: %v", err)
} else {
    log.Println("✅ Connected to Redis")
}

// ... rest of code
```

**Explanation:** The Redis client is created but **never closed**, causing a resource leak. The defer statement is missing.

**Fixed Code:**
```go
redisClient := redis.NewClient(&redis.Options{
    Addr:     redisAddr,
    Password: getEnv("REDIS_PASSWORD", ""),
    DB:       0,
})
defer redisClient.Close()

// Check Redis connection
if err := redisClient.Ping(ctx).Err(); err != nil {
    log.Printf("Warning: Failed to connect to Redis: %v", err)
} else {
    log.Println("✅ Connected to Redis")
}
```

---

### Error 10: Missing Conversation Participant Validation

**File:** `secureconnect-backend/internal/handler/http/conversation/handler.go:35-81`

**Original Code:**
```go
func (h *Handler) CreateConversation(c *gin.Context) {
    // ... parsing code ...

    // Parse participant IDs
    participantUUIDs := make([]uuid.UUID, len(req.ParticipantIDs))
    for i, idStr := range req.ParticipantIDs {
        id, err := uuid.Parse(idStr)
        if err != nil {
            response.ValidationError(c, "Invalid participant ID: "+idStr)
            return
        }
        participantUUIDs[i] = id
    }

    // Create conversation
    conv, err := h.conversationService.CreateConversation(c.Request.Context(), &conversation.CreateConversationInput{
        Title:         req.Title,
        Type:          req.Type,
        CreatedBy:     creatorID,
        Participants:  participantUUIDs,
        IsE2EEEnabled: req.IsE2EEEnabled,
    })
```

**Explanation:** The code validates that participant IDs are valid UUIDs but **doesn't verify** that these users actually exist in the system. This allows creating conversations with non-existent users.

**Fixed Code:**
```go
func (h *Handler) CreateConversation(c *gin.Context) {
    // ... parsing code ...

    // Parse participant IDs and validate users exist
    participantUUIDs := make([]uuid.UUID, 0, len(req.ParticipantIDs))
    for _, idStr := range req.ParticipantIDs {
        id, err := uuid.Parse(idStr)
        if err != nil {
            response.ValidationError(c, "Invalid participant ID: "+idStr)
            return
        }

        // Check if user exists
        user, err := h.userService.GetUserByID(c.Request.Context(), id)
        if err != nil {
            response.NotFound(c, "User not found: "+idStr)
            return
        }

        participantUUIDs = append(participantUUIDs, id)
    }

    // Validate direct conversation has exactly 2 participants
    if req.Type == "direct" && len(participantUUIDs) != 2 {
        response.ValidationError(c, "Direct conversations must have exactly 2 participants")
        return
    }

    // Create conversation
    conv, err := h.conversationService.CreateConversation(c.Request.Context(), &conversation.CreateConversationInput{
        Title:         req.Title,
        Type:          req.Type,
        CreatedBy:     creatorID,
        Participants:  participantUUIDs,
        IsE2EEEnabled: req.IsE2EEEnabled,
    })
```

---

### Error 11: Duplicate Commented Code in Database File

**File:** `secureconnect-backend/internal/database/cockroachdb.go:41-70`

**Original Code:**
```go
func (db *DB) Close() {
    db.Pool.Close()
}

// package database

// import (
//     "context"
//     "fmt"

//     "github.com/jackc/pgx/v5/pgxpool"
// )

// type DB struct {
//     Pool *pgxpool.Pool
// }

// func NewCockroachDB(ctx context.Context, connString string) (*DB, error) {
//     config, err := pgxpool.ParseConfig(connString)
//     if err != nil {
//         return nil, fmt.Errorf("unable to parse database config: %w", err)
//     }

//     pool, err := pgxpool.NewWithConfig(ctx, config)
//     if err != nil {
//         return nil, fmt.Errorf("unable to create connection pool: %w", err)
//     }

//     return &DB{Pool: pool}, nil
// }

// func (db *DB) Close() {
//     db.Pool.Close()
// }
```

**Explanation:** The file contains **duplicate commented code** that should be removed for cleanliness.

**Fixed Code:**
```go
package database

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
    Pool *pgxpool.Pool
}

func NewCockroachDB(ctx context.Context, connString string) (*DB, error) {
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, fmt.Errorf("unable to parse database config: %w", err)
    }

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("unable to create connection pool: %w", err)
    }

    return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
    db.Pool.Close()
}
```

---

### Error 12: Missing Storage Quota Validation

**File:** `secureconnect-backend/internal/service/storage/service.go:108-146`

**Original Code:**
```go
func (s *Service) GenerateUploadURL(ctx context.Context, userID uuid.UUID, input *GenerateUploadURLInput) (*GenerateUploadURLOutput, error) {
    // Generate file ID
    fileID := uuid.New()

    // Generate object key (path in MinIO)
    objectKey := fmt.Sprintf("users/%s/%s", userID, fileID)

    // Generate presigned URL (valid for 15 minutes)
    presignedURL, err := s.storage.PresignedPutObject(ctx, s.bucketName, objectKey, 15*time.Minute)
    if err != nil {
        return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
    }

    // Save file metadata to CockroachDB
    file := &domain.File{
        FileID:           fileID,
        UserID:           userID,
        FileName:         input.FileName,
        FileSize:         input.FileSize,
        ContentType:      input.ContentType,
        MinIOObjectKey:   objectKey,
        IsEncrypted:      input.IsEncrypted,
        Status:           "uploading",
        StorageQuotaUsed: input.FileSize,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }
```

**Explanation:** The service generates upload URLs **without checking** if the user has sufficient storage quota. Users can exceed their quota.

**Fixed Code:**
```go
func (s *Service) GenerateUploadURL(ctx context.Context, userID uuid.UUID, input *GenerateUploadURLInput) (*GenerateUploadURLOutput, error) {
    // 1. Check storage quota before allowing upload
    used, quota, err := s.fileRepo.GetUserStorageUsage(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to check storage usage: %w", err)
    }

    if used+input.FileSize > quota {
        return nil, fmt.Errorf("storage quota exceeded: %d/%d bytes used", used, quota)
    }

    // 2. Validate file size (max 100MB per file)
    const maxFileSize int64 = 100 * 1024 * 1024
    if input.FileSize > maxFileSize {
        return nil, fmt.Errorf("file size exceeds maximum limit of %d bytes", maxFileSize)
    }

    // 3. Validate content type
    allowedTypes := map[string]bool{
        "image/jpeg": true,
        "image/png":  true,
        "image/gif":  true,
        "image/webp": true,
        "video/mp4":  true,
        "video/webm":  true,
        "application/pdf": true,
        "text/plain": true,
    }
    if !allowedTypes[input.ContentType] {
        return nil, fmt.Errorf("file type not allowed: %s", input.ContentType)
    }

    // 4. Generate file ID
    fileID := uuid.New()

    // 5. Generate object key (path in MinIO)
    objectKey := fmt.Sprintf("users/%s/%s", userID, fileID)

    // 6. Generate presigned URL (valid for 15 minutes)
    presignedURL, err := s.storage.PresignedPutObject(ctx, s.bucketName, objectKey, 15*time.Minute)
    if err != nil {
        return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
    }

    // 7. Save file metadata to CockroachDB
    file := &domain.File{
        FileID:           fileID,
        UserID:           userID,
        FileName:         input.FileName,
        FileSize:         input.FileSize,
        ContentType:      input.ContentType,
        MinIOObjectKey:   objectKey,
        IsEncrypted:      input.IsEncrypted,
        Status:           "uploading",
        StorageQuotaUsed: input.FileSize,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }

    if err := s.fileRepo.Create(ctx, file); err != nil {
        return nil, fmt.Errorf("failed to save file metadata: %w", err)
    }

    return &GenerateUploadURLOutput{
        FileID:    fileID,
        UploadURL: presignedURL.String(),
        ExpiresAt: time.Now().Add(15 * time.Minute),
    }, nil
}
```

---

### Error 13: Missing Call Participant Authorization

**File:** `secureconnect-backend/internal/service/video/service.go:118-146`

**Original Code:**
```go
func (s *Service) JoinCall(ctx context.Context, callID uuid.UUID, userID uuid.UUID) error {
    // Verify call exists
    call, err := s.callRepo.GetByID(ctx, callID)
    if err != nil {
        return fmt.Errorf("call not found: %w", err)
    }

    // Check if call is still active
    if call.Status == "ended" {
        return fmt.Errorf("call has ended")
    }

    // Add user to participants
    if err := s.callRepo.AddParticipant(ctx, callID, userID); err != nil {
        return fmt.Errorf("failed to add participant: %w", err)
    }
```

**Explanation:** The `JoinCall` function allows any user to join any call **without verifying** if the user is a participant in the conversation or has permission to join.

**Fixed Code:**
```go
func (s *Service) JoinCall(ctx context.Context, callID uuid.UUID, userID uuid.UUID) error {
    // 1. Verify call exists
    call, err := s.callRepo.GetByID(ctx, callID)
    if err != nil {
        return fmt.Errorf("call not found: %w", err)
    }

    // 2. Check if call is still active
    if call.Status == "ended" {
        return fmt.Errorf("call has ended")
    }

    // 3. Verify user is authorized to join (member of conversation)
    isMember, err := s.conversationRepo.IsUserInConversation(ctx, call.ConversationID, userID)
    if err != nil {
        return fmt.Errorf("failed to verify conversation membership: %w", err)
    }
    if !isMember {
        return fmt.Errorf("unauthorized: user is not a member of this conversation")
    }

    // 4. Check if user is already in call
    participants, err := s.callRepo.GetParticipants(ctx, callID)
    if err != nil {
        return fmt.Errorf("failed to get participants: %w", err)
    }
    for _, p := range participants {
        if p.UserID == userID && p.LeftAt == nil {
            return fmt.Errorf("user is already in the call")
        }
    }

    // 5. Add user to participants
    if err := s.callRepo.AddParticipant(ctx, callID, userID); err != nil {
        return fmt.Errorf("failed to add participant: %w", err)
    }

    // 6. Update call status to active if it was ringing
    if call.Status == "ringing" {
        if err := s.callRepo.UpdateStatus(ctx, callID, "active"); err != nil {
            return fmt.Errorf("failed to update status: %w", err)
        }
    }

    return nil
}
```

---

### Error 14: Missing User Status Update on Logout

**File:** `secureconnect-backend/internal/service/auth/service.go:278-302`

**Original Code:**
```go
func (s *Service) Logout(ctx context.Context, sessionID string, userID uuid.UUID, tokenString string) error {
    // 1. Delete session
    if err := s.sessionRepo.DeleteSession(ctx, sessionID, userID); err != nil {
        return fmt.Errorf("failed to delete session: %w", err)
    }

    // 2. Extract JTI and blacklist token
    // ... token blacklisting code ...
```

**Explanation:** When a user logs out, their status is **not updated** to "offline". Other users will still see them as online.

**Fixed Code:**
```go
func (s *Service) Logout(ctx context.Context, sessionID string, userID uuid.UUID, tokenString string) error {
    // 1. Delete session
    if err := s.sessionRepo.DeleteSession(ctx, sessionID, userID); err != nil {
        return fmt.Errorf("failed to delete session: %w", err)
    }

    // 2. Update user status to offline
    if err := s.userRepo.UpdateStatus(ctx, userID, "offline"); err != nil {
        // Log but don't fail - session is already deleted
        log.Printf("Failed to update user status: %v", err)
    }

    // 3. Remove from presence
    if err := s.presenceRepo.SetUserOffline(ctx, userID); err != nil {
        log.Printf("Failed to remove presence: %v", err)
    }

    // 4. Extract JTI and blacklist token
    claims, err := s.jwtManager.ValidateToken(tokenString)
    if err == nil && claims.ID != "" {
        // Calculate remaining time
        expiresIn := time.Until(claims.ExpiresAt.Time)
        if expiresIn > 0 {
            if err := s.sessionRepo.BlacklistToken(ctx, claims.ID, expiresIn); err != nil {
                // Log but don't fail, session is already deleted
                log.Printf("Failed to blacklist token: %v", err)
            }
        }
    }

    return nil
}
```

---

### Error 15: Missing WebSocket Origin Validation

**File:** `secureconnect-backend/internal/handler/ws/chat_handler.go:71-77`

**Original Code:**
```go
var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        return true // Allow all origins in dev, restrict in production
    },
}
```

**Explanation:** The WebSocket upgrader accepts connections from **any origin**, which is a critical security vulnerability. Malicious sites can establish WebSocket connections to this server.

**Fixed Code:**
```go
var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        if origin == "" {
            return true // Allow non-browser clients
        }

        // Allowed origins from config
        allowedOrigins := []string{
            "http://localhost:3000",
            "http://localhost:8080",
            // Add production domains
        }

        for _, allowed := range allowedOrigins {
            if origin == allowed {
                return true
            }
        }
        return false
    },
}
```

---

## 3. Logical & Design Issues

### Issue 1: Inconsistent Error Handling

**Description:** Error handling is inconsistent across the codebase. Some functions return wrapped errors with `fmt.Errorf`, others return plain errors, and some use `log.Printf` for errors.

**Impact:** Makes debugging difficult and error messages inconsistent for clients.

**Recommended Fix:**
Create a centralized error handling package:

```go
// pkg/errors/errors.go
package errors

import (
    "fmt"
    "net/http"
)

type AppError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    StatusCode int    `json:"-"`
    Err        error  `json:"-"`
}

func (e *AppError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}

func (e *AppError) Unwrap() error {
    return e.Err
}

// Common error constructors
func NewBadRequest(message string) *AppError {
    return &AppError{
        Code:       "BAD_REQUEST",
        Message:    message,
        StatusCode: http.StatusBadRequest,
    }
}

func NewUnauthorized(message string) *AppError {
    return &AppError{
        Code:       "UNAUTHORIZED",
        Message:    message,
        StatusCode: http.StatusUnauthorized,
    }
}

func NewNotFound(resource string) *AppError {
    return &AppError{
        Code:       "NOT_FOUND",
        Message:    fmt.Sprintf("%s not found", resource),
        StatusCode: http.StatusNotFound,
    }
}

func NewInternal(err error) *AppError {
    return &AppError{
        Code:       "INTERNAL_ERROR",
        Message:    "An internal error occurred",
        StatusCode: http.StatusInternalServerError,
        Err:        err,
    }
}
```

---

### Issue 2: No Context Timeout on Database Operations

**Description:** Most database operations don't use context with timeout, which can cause requests to hang indefinitely if the database is slow.

**Impact:** Can lead to resource exhaustion and poor user experience.

**Recommended Fix:**
```go
// In handlers, use context with timeout
func (h *Handler) GetProfile(c *gin.Context) {
    ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    defer cancel()

    user, err := h.userService.GetByID(ctx, userID)
    // ...
}
```

---

### Issue 3: Missing Transaction Rollback on Error

**Description:** In [`keys_repo.go`](secureconnect-backend/internal/repository/cockroach/keys_repo.go:115-140), the transaction uses `defer tx.Rollback(ctx)` but if the transaction succeeds, rollback is still called (though it's a no-op in pgx).

**Impact:** While pgx handles this correctly, it's better practice to only rollback on error.

**Recommended Fix:**
```go
func (r *KeysRepository) SaveOneTimePreKeys(ctx context.Context, userID uuid.UUID, keys []domain.OneTimePreKey) error {
    if len(keys) == 0 {
        return nil
    }

    // Use transaction for batch insert
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    query := `
        INSERT INTO one_time_pre_keys (key_id, user_id, public_key, used)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (user_id, key_id) DO NOTHING
    `

    for _, key := range keys {
        _, err := tx.Exec(ctx, query, key.KeyID, userID, key.PublicKey, false)
        if err != nil {
            tx.Rollback(ctx) // Explicit rollback on error
            return fmt.Errorf("failed to save one-time pre-key: %w", err)
        }
    }

    if err := tx.Commit(ctx); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}
```

---

### Issue 4: No Graceful Shutdown

**Description:** None of the services implement graceful shutdown. When the process receives a termination signal, it immediately kills all connections.

**Impact:** Can cause data loss, incomplete transactions, and poor user experience.

**Recommended Fix:**
```go
// In main.go
func main() {
    // ... initialization code ...

    // Create server
    server := &http.Server{
        Addr:    addr,
        Handler: router,
    }

    // Start server in goroutine
    go func() {
        log.Printf("🚀 Auth Service starting on port %s\n", port)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Failed to start server: %v", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down server...")

    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Printf("Server forced to shutdown: %v", err)
    }

    // Close database connections
    crdb.Close()
    redisDB.Close()

    log.Println("Server exited")
}
```

---

### Issue 5: Missing Conversation Service Implementation

**Description:** The [`conversation/service.go`](secureconnect-backend/internal/service/conversation/service.go) file exists but is not implemented. The handler references methods that don't exist.

**Impact:** The conversation endpoints will fail at runtime.

**Recommended Fix:**
Implement the conversation service (see Section 8.5 for full implementation).

---

### Issue 6: Inconsistent Pagination

**Description:** Pagination is implemented inconsistently. Some endpoints use `limit` and `offset`, others use cursor-based pagination, and some have no pagination at all.

**Impact:** Inconsistent API behavior and potential performance issues with large datasets.

**Recommended Fix:**
Implement a standardized pagination approach:

```go
// pkg/pagination/pagination.go
package pagination

import (
    "encoding/base64"
    "encoding/json"
    "errors"
)

type PageRequest struct {
    Limit     int
    Cursor    string // Base64 encoded cursor
    Direction string // "next" or "prev"
}

type PageResponse struct {
    Data       interface{}
    NextCursor string
    PrevCursor string
    HasMore    bool
}

type Cursor struct {
    ID        string `json:"id"`
    CreatedAt string `json:"created_at"`
}

func ParseCursor(cursorStr string) (*Cursor, error) {
    if cursorStr == "" {
        return nil, nil
    }
    
    decoded, err := base64.StdEncoding.DecodeString(cursorStr)
    if err != nil {
        return nil, errors.New("invalid cursor")
    }
    
    var cursor Cursor
    if err := json.Unmarshal(decoded, &cursor); err != nil {
        return nil, errors.New("invalid cursor")
    }
    
    return &cursor, nil
}

func EncodeCursor(cursor Cursor) (string, error) {
    data, err := json.Marshal(cursor)
    if err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(data), nil
}

func ValidateLimit(limit int) int {
    if limit <= 0 {
        return 20 // Default
    }
    if limit > 100 {
        return 100 // Max
    }
    return limit
}
```

---

### Issue 7: No Input Sanitization

**Description:** User inputs are not sanitized before being stored or used in queries. While parameterized queries prevent SQL injection, other attacks like XSS are still possible.

**Impact:** Potential XSS attacks through stored data.

**Recommended Fix:**
```go
// pkg/sanitize/sanitize.go
package sanitize

import (
    "html"
    "regexp"
    "strings"
)

var (
    // HTML tags that are allowed
    allowedTags = map[string]bool{
        "b": true, "i": true, "u": true, "em": true, "strong": true,
    }
)

func HTML(input string) string {
    // First escape HTML
    escaped := html.EscapeString(input)
    return escaped
}

func Username(input string) string {
    // Remove special characters, keep alphanumeric and underscore
    re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
    return re.ReplaceAllString(input, "")
}

func Email(input string) string {
    // Trim whitespace and lowercase
    return strings.TrimSpace(strings.ToLower(input))
}

func Message(input string) string {
    // Escape HTML but preserve line breaks
    escaped := html.EscapeString(input)
    return escaped
}
```

---

## 4. Code Quality Improvements

### 4.1 Naming

**Issue:** Some variable names are unclear or inconsistent.

**Examples:**
- `crdb` → `cockroachDB` or `db`
- `r` for repository methods → more descriptive names

**Fix:**
```go
// Before
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {

// After
func (repo *UserRepository) Create(ctx context.Context, user *domain.User) error {
```

---

### 4.2 Structure

**Issue:** The project structure has some inconsistencies:
- Empty directories: `internal/handler/ws/chat/` and `internal/handler/ws/signaling/`
- Missing `pkg/response` package (it's referenced but file exists)

**Fix:**
Remove empty directories and ensure all referenced packages exist.

---

### 4.3 Duplication

**Issue:** The `getEnv` helper function is duplicated in multiple main.go files.

**Fix:**
Create a shared utility package:

```go
// pkg/env/env.go
package env

import (
    "os"
    "strconv"
    "time"
)

func GetString(key, defaultValue string) string {
    value := os.Getenv(key)
    if value == "" {
        return defaultValue
    }
    return value
}

func GetInt(key string, defaultValue int) int {
    valueStr := os.Getenv(key)
    if valueStr == "" {
        return defaultValue
    }
    value, err := strconv.Atoi(valueStr)
    if err != nil {
        return defaultValue
    }
    return value
}

func GetBool(key string, defaultValue bool) bool {
    valueStr := os.Getenv(key)
    if valueStr == "" {
        return defaultValue
    }
    value, err := strconv.ParseBool(valueStr)
    if err != nil {
        return defaultValue
    }
    return value
}

func GetDuration(key string, defaultValue time.Duration) time.Duration {
    valueStr := os.Getenv(key)
    if valueStr == "" {
        return defaultValue
    }
    value, err := time.ParseDuration(valueStr)
    if err != nil {
        return defaultValue
    }
    return value
}
```

---

### 4.4 Consistency

**Issue:** Inconsistent use of logging:
- Some places use `log.Printf`
- Others use `fmt.Printf`
- The `logger` package exists but is not consistently used

**Fix:**
Standardize on using the `logger` package:

```go
// Instead of:
log.Printf("Failed to connect: %v", err)
fmt.Printf("Error: %v", err)

// Use:
logger.Error("Failed to connect", 
    zap.Error(err),
    zap.String("service", "auth-service"),
)
```

---

## 5. Performance & Scalability Review

### 5.1 Bottlenecks

**1. N+1 Query Problem in Message Retrieval**

**File:** `secureconnect-backend/internal/service/chat/service.go:142-178`

**Issue:** When retrieving messages, sender name is joined but this requires additional queries.

**Fix:** Use a single query with JOIN or cache user data.

---

**2. Missing Database Connection Pool Configuration**

**File:** `secureconnect-backend/internal/database/cockroachdb.go:15-29`

**Issue:** The connection pool uses default settings which may not be optimal for production.

**Fix:**
```go
func NewCockroachDB(ctx context.Context, config *Config) (*DB, error) {
    // Build connection string
    connString := fmt.Sprintf(
        "postgresql://%s:%s@%s:%d/%s?sslmode=%s",
        config.User,
        config.Password,
        config.Host,
        config.Port,
        config.Database,
        config.SSLMode,
    )

    // Parse configuration
    poolConfig, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, fmt.Errorf("unable to parse database config: %w", err)
    }

    // Configure connection pool
    poolConfig.MaxConns = config.MaxConns
    poolConfig.MinConns = config.MinConns
    poolConfig.MaxConnLifetime = 1 * time.Hour
    poolConfig.MaxConnIdleTime = 30 * time.Minute
    poolConfig.HealthCheckPeriod = 1 * time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
    if err != nil {
        return nil, fmt.Errorf("unable to create connection pool: %w", err)
    }

    return &DB{Pool: pool}, nil
}
```

---

**3. No Caching for Frequently Accessed Data**

**Issue:** User data, conversation data, and keys are fetched from the database on every request.

**Fix:** Implement caching layer:

```go
// pkg/cache/cache.go
package cache

import (
    "context"
    "encoding/json"
    "time"

    "github.com/redis/go-redis/v9"
)

type Cache struct {
    client *redis.Client
}

func NewCache(client *redis.Client) *Cache {
    return &Cache{client: client}
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
    data, err := c.client.Get(ctx, key).Result()
    if err != nil {
        return err
    }
    return json.Unmarshal([]byte(data), dest)
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, key string) error {
    return c.client.Del(ctx, key).Err()
}
```

---

### 5.2 Inefficient Patterns

**1. Sequential Database Queries**

**Issue:** In [`keys_repo.go`](secureconnect-backend/internal/repository/cockroach/keys_repo.go:192-220), the `GetPreKeyBundle` function makes 3 sequential database queries.

**Fix:** Use a single query with JOIN or parallel queries with goroutines.

---

**2. Missing Pagination in User Lists**

**Issue:** The `GetParticipants` and `GetUserConversations` methods don't implement pagination.

**Fix:** Add pagination parameters.

---

### 5.3 Optimized Alternatives

**1. Use Redis Streams for Pub/Sub**

**Issue:** Current pub/sub implementation uses basic Redis pub/sub which doesn't persist messages.

**Fix:** Use Redis Streams for message persistence and replay capability.

---

**2. Implement Connection Pooling for Cassandra**

**Issue:** The Cassandra session is created without proper configuration.

**Fix:**
```go
func NewCassandraDB(hosts []string, keyspace string) (*CassandraDB, error) {
    cluster := gocql.NewCluster(hosts...)
    cluster.Keyspace = keyspace
    cluster.Consistency = gocql.Quorum
    
    // Configure connection pool
    cluster.NumConns = 4
    cluster.Timeout = 600 * time.Millisecond
    
    session, err := cluster.CreateSession()
    if err != nil {
        return nil, err
    }
    return &CassandraDB{Session: session}, nil
}
```

---

## 6. Security & Stability Review

### 6.1 Vulnerabilities

**1. Weak Default JWT Secret**

**Severity:** CRITICAL

**File:** All `main.go` files

**Issue:** Default JWT secret "super-secret-key-change-in-production" is weak and hardcoded.

**Fix:** Require JWT_SECRET in production and validate minimum length.

---

**2. CORS Misconfiguration**

**Severity:** HIGH

**File:** `internal/middleware/cors.go`

**Issue:** `Access-Control-Allow-Origin: *` with credentials is invalid and allows any origin.

**Fix:** Implement proper origin validation.

---

**3. WebSocket Origin Not Validated**

**Severity:** HIGH

**File:** `internal/handler/ws/chat_handler.go`, `internal/handler/ws/signaling_handler.go`

**Issue:** WebSocket connections accepted from any origin.

**Fix:** Implement `CheckOrigin` function with whitelist.

---

**4. Missing Rate Limiting per User**

**Severity:** MEDIUM

**File:** `internal/middleware/ratelimit.go`

**Issue:** Rate limiting only by IP, not by authenticated user.

**Fix:** Implement user-based rate limiting.

---

**5. No Account Lockout**

**Severity:** MEDIUM

**File:** `internal/service/auth/service.go`

**Issue:** No limit on failed login attempts, enabling brute force attacks.

**Fix:**
```go
type FailedLoginAttempt struct {
    UserID    uuid.UUID
    IP        string
    Attempts  int
    LockedUntil time.Time
}

func (s *Service) Login(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
    // Check if account is locked
    locked, err := s.checkAccountLocked(ctx, input.Email)
    if err != nil {
        return nil, fmt.Errorf("failed to check account status: %w", err)
    }
    if locked {
        return nil, fmt.Errorf("account is temporarily locked due to too many failed attempts")
    }

    // ... existing login logic ...

    // On successful login, clear failed attempts
    s.clearFailedAttempts(ctx, input.Email)
}

func (s *Service) checkAccountLocked(ctx context.Context, email string) (bool, error) {
    key := fmt.Sprintf("failed_login:%s", email)
    attempts, err := s.redis.Get(ctx, key).Int()
    if err != nil && err != redis.Nil {
        return false, err
    }

    if attempts >= 5 {
        lockedUntil, err := s.redis.Get(ctx, key+":locked").Time()
        if err != nil {
            return false, err
        }
        return time.Now().Before(lockedUntil), nil
    }

    return false, nil
}
```

---

### 6.2 Unsafe Patterns

**1. Using fmt.Printf for Error Logging**

**Issue:** Error logging uses `fmt.Printf` instead of structured logging.

**Fix:** Use the `logger` package consistently.

---

**2. No Input Validation for File Types**

**Issue:** File upload doesn't validate content types.

**Fix:** Implement whitelist of allowed MIME types.

---

**3. Missing Content Security Headers**

**Issue:** No security headers like CSP, HSTS, X-Frame-Options.

**Fix:**
```go
// internal/middleware/security.go
package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent clickjacking
        c.Writer.Header().Set("X-Frame-Options", "DENY")
        
        // Prevent MIME type sniffing
        c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
        
        // XSS Protection
        c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
        
        // HSTS
        c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        
        // Referrer Policy
        c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        
        // Content Security Policy
        c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
        
        c.Next()
    }
}
```

---

## 7. Missing or Weak Features

### Feature 1: User Registration Email Verification

**Status:** Missing  
**Purpose & Value:** Verify user email addresses to prevent spam and ensure users own their email addresses.  
**Related Modules:** Auth Service  
**Design Overview:**
1. Generate verification token on registration
2. Send email with verification link
3. User clicks link to verify email
4. Mark user as verified

**Files to Add / Modify:**
- `internal/domain/user.go` - Add `EmailVerified` and `EmailVerificationToken` fields
- `internal/service/auth/service.go` - Add `SendVerificationEmail` and `VerifyEmail` methods
- `internal/handler/http/auth/handler.go` - Add `/verify-email` endpoint

**Implementation:**
```go
// In domain/user.go
type User struct {
    // ... existing fields ...
    EmailVerified          bool       `json:"email_verified" db:"email_verified"`
    EmailVerificationToken *string    `json:"-" db:"email_verification_token"`
    EmailVerifiedAt        *time.Time `json:"email_verified_at" db:"email_verified_at"`
}

// In service/auth/service.go
func (s *Service) SendVerificationEmail(ctx context.Context, userID uuid.UUID) error {
    user, err := s.userRepo.GetByID(ctx, userID)
    if err != nil {
        return fmt.Errorf("user not found: %w", err)
    }

    // Generate verification token
    token := generateRandomToken()
    
    // Store token in database
    if err := s.userRepo.UpdateVerificationToken(ctx, userID, token); err != nil {
        return fmt.Errorf("failed to update verification token: %w", err)
    }

    // Send email (implement email service)
    verificationURL := fmt.Sprintf("https://app.example.com/verify-email?token=%s", token)
    return s.emailService.Send(user.Email, "Verify your email", verificationURL)
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
    user, err := s.userRepo.GetByVerificationToken(ctx, token)
    if err != nil {
        return fmt.Errorf("invalid verification token: %w", err)
    }

    now := time.Now()
    return s.userRepo.MarkEmailVerified(ctx, user.UserID, now)
}
```

**Integration Notes:**
- Need to implement email service (SMTP or third-party like SendGrid)
- Add verification token expiration (24 hours)
- Update registration flow to require verification

**Validation & Testing:**
- Test email sending with real SMTP server
- Test verification link expiration
- Test re-sending verification email
- Test login rejection for unverified users

---

### Feature 2: Password Reset

**Status:** Missing  
**Purpose & Value:** Allow users to reset their password when forgotten, improving user experience and security.  
**Related Modules:** Auth Service  
**Design Overview:**
1. User requests password reset with email
2. Generate reset token with expiration
3. Send email with reset link
4. User submits new password with token
5. Validate token and update password

**Files to Add / Modify:**
- `internal/domain/user.go` - Add `PasswordResetToken` and `PasswordResetExpiresAt` fields
- `internal/service/auth/service.go` - Add `RequestPasswordReset` and `ResetPassword` methods
- `internal/handler/http/auth/handler.go` - Add `/forgot-password` and `/reset-password` endpoints

**Implementation:**
```go
// In service/auth/service.go
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
    user, err := s.userRepo.GetByEmail(ctx, email)
    if err != nil {
        // Don't reveal if email exists or not
        return nil
    }

    // Generate reset token
    token := generateSecureToken()
    expiresAt := time.Now().Add(1 * time.Hour)

    // Store token
    if err := s.userRepo.SetPasswordResetToken(ctx, user.UserID, token, expiresAt); err != nil {
        return fmt.Errorf("failed to set reset token: %w", err)
    }

    // Send email
    resetURL := fmt.Sprintf("https://app.example.com/reset-password?token=%s", token)
    return s.emailService.Send(user.Email, "Reset your password", resetURL)
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
    // Validate token
    user, err := s.userRepo.GetByPasswordResetToken(ctx, token)
    if err != nil {
        return fmt.Errorf("invalid or expired reset token: %w", err)
    }

    // Check if token is expired
    if user.PasswordResetExpiresAt != nil && time.Now().After(*user.PasswordResetExpiresAt) {
        return fmt.Errorf("reset token has expired")
    }

    // Hash new password
    passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
    if err != nil {
        return fmt.Errorf("failed to hash password: %w", err)
    }

    // Update password
    if err := s.userRepo.UpdatePassword(ctx, user.UserID, string(passwordHash)); err != nil {
        return fmt.Errorf("failed to update password: %w", err)
    }

    // Clear reset token
    return s.userRepo.ClearPasswordResetToken(ctx, user.UserID)
}
```

**Integration Notes:**
- Integrate with email service for sending reset emails
- Store reset tokens in Redis for faster access and expiration handling
- Implement rate limiting for password reset requests

**Validation & Testing:**
- Test with real SMTP server
- Test token expiration logic
- Test password strength validation
- Test that reset token can only be used once

---

### Feature 3: Two-Factor Authentication (2FA)

**Status:** Missing  
**Purpose & Value:** Add an extra layer of security by requiring a second factor (TOTP or SMS) for sensitive operations.  
**Related Modules:** Auth Service  
**Design Overview:**
1. User enables 2FA and receives secret key
2. User configures authenticator app
3. Login requires TOTP code
4. Recovery codes provided for backup

**Files to Add / Modify:**
- `internal/domain/user.go` - Add `TwoFactorEnabled`, `TwoFactorSecret`, `RecoveryCodes` fields
- `internal/service/auth/service.go` - Add 2FA methods
- `internal/handler/http/auth/handler.go` - Add `/2fa/enable`, `/2fa/disable`, `/2fa/verify` endpoints

**Implementation:**
```go
// In domain/user.go
type User struct {
    // ... existing fields ...
    TwoFactorEnabled bool       `json:"two_factor_enabled" db:"two_factor_enabled"`
    TwoFactorSecret *string    `json:"-" db:"two_factor_secret"` // Encrypted
    RecoveryCodes   []string   `json:"-" db:"recovery_codes"` // Encrypted
}

// In service/auth/service.go
func (s *Service) EnableTwoFactor(ctx context.Context, userID uuid.UUID) (*TwoFactorSetup, error) {
    // Generate TOTP secret
    secret, err := totp.GenerateSecret()
    if err != nil {
        return nil, fmt.Errorf("failed to generate secret: %w", err)
    }

    // Generate recovery codes
    recoveryCodes := generateRecoveryCodes(10)

    // Encrypt and store
    encryptedSecret, err := s.encryptSecret(secret)
    if err != nil {
        return nil, fmt.Errorf("failed to encrypt secret: %w", err)
    }

    encryptedCodes, err := s.encryptRecoveryCodes(recoveryCodes)
    if err != nil {
        return nil, fmt.Errorf("failed to encrypt codes: %w", err)
    }

    if err := s.userRepo.SetTwoFactorData(ctx, userID, encryptedSecret, encryptedCodes); err != nil {
        return nil, fmt.Errorf("failed to save 2FA data: %w", err)
    }

    return &TwoFactorSetup{
        Secret:        secret,
        QRCode:        generateQRCode(secret),
        RecoveryCodes:  recoveryCodes,
    }, nil
}

func (s *Service) VerifyTwoFactor(ctx context.Context, userID uuid.UUID, code string) error {
    user, err := s.userRepo.GetByID(ctx, userID)
    if err != nil {
        return fmt.Errorf("user not found: %w", err)
    }

    if !user.TwoFactorEnabled {
        return fmt.Errorf("2FA is not enabled for this user")
    }

    // Decrypt secret
    secret, err := s.decryptSecret(user.TwoFactorSecret)
    if err != nil {
        return fmt.Errorf("failed to decrypt secret: %w", err)
    }

    // Verify TOTP code
    if !totp.ValidateCode(secret, code) {
        return fmt.Errorf("invalid 2FA code")
    }

    return nil
}
```

**Integration Notes:**
- Use `github.com/pquerna/otp/totp` for TOTP generation/validation
- Use `github.com/skip2/go-qrcode` for QR code generation
- Encrypt 2FA secrets using AES-256

**Validation & Testing:**
- Test TOTP code generation and validation
- Test QR code generation
- Test recovery code redemption
- Test 2FA bypass prevention

---

### Feature 4: Message Read Receipts

**Status:** Missing  
**Purpose & Value:** Allow users to see when their messages have been read by recipients.  
**Related Modules:** Chat Service  
**Design Overview:**
1. Mark message as read when user views it
2. Broadcast read receipt to sender
3. Track read status per recipient

**Files to Add / Modify:**
- `internal/domain/message.go` - Add read receipt structures
- `internal/repository/cassandra/message_repo.go` - Add read receipt storage
- `internal/service/chat/service.go` - Add read receipt methods
- `internal/handler/http/chat/handler.go` - Add `/messages/:id/read` endpoint

**Implementation:**
```go
// In domain/message.go
type MessageReadReceipt struct {
    MessageID   uuid.UUID `json:"message_id"`
    ReaderID    uuid.UUID `json:"reader_id"`
    ReadAt      time.Time `json:"read_at"`
}

// In service/chat/service.go
func (s *Service) MarkMessageRead(ctx context.Context, messageID, readerID uuid.UUID) error {
    // Store read receipt
    receipt := &MessageReadReceipt{
        MessageID: messageID,
        ReaderID:  readerID,
        ReadAt:   time.Now(),
    }

    if err := s.messageRepo.SaveReadReceipt(ctx, receipt); err != nil {
        return fmt.Errorf("failed to save read receipt: %w", err)
    }

    // Get message to find sender
    message, err := s.messageRepo.GetByID(ctx, messageID)
    if err != nil {
        return fmt.Errorf("message not found: %w", err)
    }

    // Broadcast read receipt to sender via Redis pub/sub
    channel := fmt.Sprintf("read_receipts:%s", message.SenderID)
    receiptJSON, _ := json.Marshal(receipt)
    s.publisher.Publish(ctx, channel, receiptJSON)

    return nil
}
```

**Integration Notes:**
- Store read receipts in Cassandra for persistence
- Use Redis pub/sub for real-time delivery
- Add read status to message responses

**Validation & Testing:**
- Test read receipt storage
- Test real-time read notification delivery
- Test read status aggregation
- Test privacy controls (opt-out of read receipts)

---

### Feature 5: Message Editing and Deletion

**Status:** Missing  
**Purpose & Value:** Allow users to edit or delete their messages after sending.  
**Related Modules:** Chat Service  
**Design Overview:**
1. Edit message content within time window
2. Soft delete messages with retention
3. Notify participants of edits/deletions

**Files to Add / Modify:**
- `internal/domain/message.go` - Add `EditedAt`, `DeletedAt` fields
- `internal/repository/cassandra/message_repo.go` - Add edit/delete methods
- `internal/service/chat/service.go` - Add edit/delete methods
- `internal/handler/http/chat/handler.go` - Add `/messages/:id/edit`, `/messages/:id/delete` endpoints

**Implementation:**
```go
// In service/chat/service.go
func (s *Service) EditMessage(ctx context.Context, messageID, userID uuid.UUID, newContent string) error {
    // Get message
    message, err := s.messageRepo.GetByID(ctx, messageID)
    if err != nil {
        return fmt.Errorf("message not found: %w", err)
    }

    // Verify ownership
    if message.SenderID != userID {
        return fmt.Errorf("unauthorized: can only edit own messages")
    }

    // Check time window (e.g., 10 minutes)
    if time.Since(message.CreatedAt) > 10*time.Minute {
        return fmt.Errorf("message can no longer be edited")
    }

    // Update message
    if err := s.messageRepo.EditMessage(ctx, messageID, newContent, time.Now()); err != nil {
        return fmt.Errorf("failed to edit message: %w", err)
    }

    // Broadcast edit notification
    s.broadcastEditNotification(messageID, newContent)

    return nil
}

func (s *Service) DeleteMessage(ctx context.Context, messageID, userID uuid.UUID) error) {
    // Get message
    message, err := s.messageRepo.GetByID(ctx, messageID)
    if err != nil {
        return fmt.Errorf("message not found: %w", err)
    }

    // Verify ownership or admin
    if message.SenderID != userID {
        return fmt.Errorf("unauthorized: can only delete own messages")
    }

    // Soft delete
    if err := s.messageRepo.DeleteMessage(ctx, messageID, time.Now()); err != nil {
        return fmt.Errorf("failed to delete message: %w", err)
    }

    // Broadcast delete notification
    s.broadcastDeleteNotification(messageID)

    return nil
}
```

**Integration Notes:**
- Use soft delete for audit trail
- Add edit history tracking
- Implement time-based editing restrictions

**Validation & Testing:**
- Test edit time window enforcement
- Test ownership verification
- Test soft delete behavior
- Test real-time edit/delete notifications

---

### Feature 6: Contact Management

**Status:** Missing  
**Purpose & Value:** Allow users to manage their contact list for easy communication.  
**Related Modules:** Auth Service, Conversation Service  
**Design Overview:**
1. Add/remove contacts
2. Search contacts
3. Block contacts
4. Contact suggestions

**Files to Add / Modify:**
- `internal/service/contact/service.go` - New service file
- `internal/repository/cockroach/contact_repo.go` - New repository file
- `internal/handler/http/contact/handler.go` - New handler file

**Implementation:**
```go
// internal/service/contact/service.go
package contact

import (
    "context"
    "fmt"

    "github.com/google/uuid"

    "secureconnect-backend/internal/domain"
    "secureconnect-backend/internal/repository/cockroach"
)

type Service struct {
    contactRepo *cockroach.ContactRepository
    userRepo    *cockroach.UserRepository
}

type AddContactInput struct {
    ContactID uuid.UUID
}

type BlockContactInput struct {
    ContactID uuid.UUID
}

func NewService(contactRepo *cockroach.ContactRepository, userRepo *cockroach.UserRepository) *Service {
    return &Service{
        contactRepo: contactRepo,
        userRepo:    userRepo,
    }
}

func (s *Service) AddContact(ctx context.Context, userID, contactID uuid.UUID) error {
    // Verify contact exists
    _, err := s.userRepo.GetByID(ctx, contactID)
    if err != nil {
        return fmt.Errorf("user not found: %w", err)
    }

    // Check if already contacts
    exists, err := s.contactRepo.Exists(ctx, userID, contactID)
    if err != nil {
        return fmt.Errorf("failed to check contact status: %w", err)
    }
    if exists {
        return fmt.Errorf("already in contacts")
    }

    // Add contact
    contact := &domain.Contact{
        UserID:         userID,
        ContactUserID:   contactID,
        Status:         "pending",
    }

    return s.contactRepo.Create(ctx, contact)
}

func (s *Service) AcceptContact(ctx context.Context, userID, contactID uuid.UUID) error {
    return s.contactRepo.UpdateStatus(ctx, contactID, userID, "accepted")
}

func (s *Service) BlockContact(ctx context.Context, userID, contactID uuid.UUID) error) {
    return s.contactRepo.UpdateStatus(ctx, contactID, userID, "blocked")
}

func (s *Service) GetContacts(ctx context.Context, userID uuid.UUID, status string) ([]*domain.UserResponse, error) {
    // Get contact IDs
    contacts, err := s.contactRepo.GetByUser(ctx, userID, status)
    if err != nil {
        return nil, fmt.Errorf("failed to get contacts: %w", err)
    }

    // Get user details
    users := make([]*domain.UserResponse, len(contacts))
    for i, contact := range contacts {
        user, err := s.userRepo.GetByID(ctx, contact.ContactUserID)
        if err != nil {
            return nil, fmt.Errorf("failed to get user: %w", err)
        }
        users[i] = *user.ToResponse()
    }

    return users, nil
}
```

**Integration Notes:**
- Add contact status notifications
- Implement contact search functionality
- Implement contact suggestions based on mutual connections

**Validation & Testing:**
- Test contact request/accept flow
- Test blocking functionality
- Test contact search
- Test privacy controls

---

### Feature 7: User Search

**Status:** Missing  
**Purpose & Value:** Allow users to search for other users by email, username, or display name.  
**Related Modules:** Auth Service  
**Design Overview:**
1. Search by email/username
2. Paginated results
3. Privacy controls (opt-out of search)

**Files to Add / Modify:**
- `internal/repository/cockroach/user_repo.go` - Add search methods
- `internal/service/auth/service.go` - Add search methods
- `internal/handler/http/auth/handler.go` - Add `/users/search` endpoint

**Implementation:**
```go
// In repository/cockroach/user_repo.go
func (r *UserRepository) Search(ctx context.Context, query string, limit, offset int) ([]*domain.User, error) {
    searchQuery := `
        SELECT user_id, email, username, display_name, avatar_url, status, created_at, updated_at
        FROM users
        WHERE username ILIKE $1 
           OR email ILIKE $1 
           OR display_name ILIKE $1
           AND searchable = true
        ORDER BY username ASC
        LIMIT $2 OFFSET $3
    `

    searchTerm := "%" + query + "%"
    rows, err := r.pool.Query(ctx, searchQuery, searchTerm, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("failed to search users: %w", err)
    }
    defer rows.Close()

    var users []*domain.User
    for rows.Next() {
        user := &domain.User{}
        err := rows.Scan(
            &user.UserID,
            &user.Email,
            &user.Username,
            &user.DisplayName,
            &user.AvatarURL,
            &user.Status,
            &user.CreatedAt,
            &user.UpdatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan user: %w", err)
        }
        users = append(users, user)
    }

    return users, nil
}
```

**Integration Notes:**
- Add `searchable` flag to user table for privacy
- Implement rate limiting for search requests
- Cache popular search results

**Validation & Testing:**
- Test search by email
- Test search by username
- Test search by display name
- Test pagination
- Test privacy opt-out

---

### Feature 8: Conversation Search

**Status:** Missing  
**Purpose & Value:** Allow users to search within their conversations for specific messages.  
**Related Modules:** Chat Service  
**Design Overview:**
1. Full-text search on message content
2. Search within specific conversation or all conversations
3. Paginated results

**Files to Add / Modify:**
- `internal/repository/cassandra/message_repo.go` - Add search methods
- `internal/service/chat/service.go` - Add search methods
- `internal/handler/http/chat/handler.go` - Add `/messages/search` endpoint

**Implementation:**
```go
// In repository/cassandra/message_repo.go
func (r *MessageRepository) Search(ctx context.Context, userID uuid.UUID, query string, limit int) ([]*domain.Message, error) {
    // Get user's conversations
    // For each conversation, search messages
    // Aggregate and return results

    // This is a simplified implementation
    // For production, consider using a search engine like Elasticsearch
    searchQuery := `
        SELECT conversation_id, bucket, message_id, sender_id, content,
               is_encrypted, message_type, metadata, created_at
        FROM messages
        WHERE content ILIKE ?
        ALLOW FILTERING
        LIMIT ?
    `

    searchTerm := "%" + query + "%"
    iter := r.session.Query(searchQuery, searchTerm, limit)
    defer iter.Close()

    var messages []*domain.Message
    for {
        message := &domain.Message{}
        if !iter.Scan(
            &message.ConversationID,
            &message.Bucket,
            &message.MessageID,
            &message.SenderID,
            &message.Content,
            &message.IsEncrypted,
            &message.MessageType,
            &message.Metadata,
            &message.CreatedAt,
        ) {
            break
        }
        messages = append(messages, message)
    }

    return messages, nil
}
```

**Integration Notes:**
- For production, implement Elasticsearch or similar search engine
- Add search result highlighting
- Implement search filters (by date, by sender)

**Validation & Testing:**
- Test search functionality
- Test search across multiple conversations
- Test search performance with large datasets
- Test encrypted message handling

---

### Feature 9: Push Notifications

**Status:** Risky (TODO comments exist but no implementation)  
**Purpose & Value:** Notify users of new messages, calls, and other events when they are offline.  
**Related Modules:** Chat Service, Video Service  
**Design Overview:**
1. Register device tokens (FCM/APNS)
2. Queue notifications when user is offline
3. Send push notifications via FCM/APNS

**Files to Add / Modify:**
- `internal/domain/device.go` - New domain file
- `internal/repository/cockroach/device_repo.go` - New repository file
- `internal/service/notification/service.go` - New service file
- `internal/handler/http/notification/handler.go` - New handler file

**Implementation:**
```go
// internal/domain/device.go
package domain

import (
    "time"

    "github.com/google/uuid"
)

type Device struct {
    DeviceID    uuid.UUID `json:"device_id" db:"device_id"`
    UserID      uuid.UUID `json:"user_id" db:"user_id"`
    Platform    string    `json:"platform" db:"platform"` // ios, android, web
    Token       string    `json:"-" db:"token"` // FCM/APNS token
    Active      bool      `json:"active" db:"active"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    LastUsedAt  time.Time `json:"last_used_at" db:"last_used_at"`
}

type Notification struct {
    NotificationID uuid.UUID `json:"notification_id"`
    UserID       uuid.UUID `json:"user_id"`
    Type         string    `json:"type"` // message, call, system
    Title        string    `json:"title"`
    Body         string    `json:"body"`
    Data         map[string]interface{} `json:"data"`
    Read         bool      `json:"read"`
    CreatedAt    time.Time `json:"created_at"`
}

// internal/service/notification/service.go
package notification

import (
    "context"
    "fmt"

    "github.com/google/uuid"

    "secureconnect-backend/internal/domain"
)

type Service struct {
    deviceRepo DeviceRepository
    fcmClient  *firebase.Client
    apnsClient *apns2.Client
}

func (s *Service) RegisterDevice(ctx context.Context, userID uuid.UUID, platform, token string) error {
    device := &domain.Device{
        DeviceID:   uuid.New(),
        UserID:     userID,
        Platform:   platform,
        Token:      token,
        Active:     true,
        CreatedAt:   time.Now(),
        LastUsedAt:  time.Now(),
    }

    return s.deviceRepo.Create(ctx, device)
}

func (s *Service) SendPushNotification(ctx context.Context, userID uuid.UUID, notification *domain.Notification) error {
    // Get user's active devices
    devices, err := s.deviceRepo.GetActiveByUser(ctx, userID)
    if err != nil {
        return fmt.Errorf("failed to get devices: %w", err)
    }

    if len(devices) == 0 {
        return nil // No devices to notify
    }

    // Send to each device
    for _, device := range devices {
        switch device.Platform {
        case "ios":
            err = s.sendAPNS(ctx, device.Token, notification)
        case "android":
            err = s.sendFCM(ctx, device.Token, notification)
        case "web":
            err = s.sendWebPush(ctx, device.Token, notification)
        }

        if err != nil {
            // Log error but continue to other devices
            fmt.Printf("Failed to send to device %s: %v", device.DeviceID, err)
        }
    }

    return nil
}
```

**Integration Notes:**
- Integrate Firebase Cloud Messaging (FCM)
