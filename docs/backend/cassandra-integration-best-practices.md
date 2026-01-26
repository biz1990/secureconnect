# Backend Cassandra Integration Best Practices (Go)

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 13.1. Tổng quan

Cassandra (hoặc ScyllaDB - tối ưu hơn, tương thích driver) được chọn để lưu trữ tin nhắn vì khả năng ghi cực nhanh. Tuy nhiên, Cassandra là NoSQL rất "độc đáo", nếu code sai quy tắc có thể làm sập cụm (Cluster) hoặc query rất chậm.

### Các nguyên tắc vàng (Golden Rules)
1.  **Luôn dùng Prepared Statements:** Không bao giờ dùng Query string động (trừ Admin).
2.  **Tránh `ALLOW FILTERING`:** Đừng bao giờ dùng truy vấn quét toàn bộ bảng.
3.  **Sử dụng Batch cẩn thận:** Batch chỉ nên dùng khi ghi vào **cùng một Partition Key**.
4.  **Quản lý Connection Pool:** Đừng tạo/close connection cho mỗi request.

---

## 13.2. Cài đặt Driver (Go)

Chúng ta sử dụng thư viện chuẩn `gocql`.

```bash
go get github.com/gocql/gocql
```

*Lưu ý:* Nếu bạn dùng **ScyllaDB** (tốt hơn Cassandra cho hiệu năng), thư viện `gocql` hoàn toàn tương thích.

---

## 13.3. Thiết lập Connection Cluster & Session

Đừng tạo Session mới cho mỗi request. Hãy khởi tạo một **Singleton Session** và tái sử dụng nó.

### File: `internal/database/cassandra.go`

```go
package database

import (
    "log"
    "time"

    "github.com/gocql/gocql"
)

type CassandraDB struct {
    Session *gocql.Session
}

func NewCassandraDB(hosts []string, keyspace string) *CassandraDB {
    // 1. Cấu hình Cluster
    cluster := gocql.NewCluster(hosts...)
    cluster.Keyspace = keyspace
    cluster.Consistency = gocql.LocalQuorum // Đảm bảo nhất quán trong DC
    cluster.Timeout = 2 * time.Second        // Timeout cho query
    cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(hosts...)

    // 2. Số lượng kết nối tối đa tới mỗi node
    // Quan trọng để chịu tải cao
    cluster.NumConns = 10 
    cluster.RetryPolicy = &gocql.SimpleRetryPolicy{NumRetries: 3}

    // 3. Tạo Session
    session, err := cluster.CreateSession()
    if err != nil {
        log.Fatalf("Lỗi kết nối Cassandra: %v", err)
    }

    return &CassandraDB{Session: session}
}
```

---

## 13.4. Best Practice 1: Prepared Statements (Chuẩn bị trước)

**Vấn đề:** Nếu bạn thực hiện `session.Query("INSERT...").Exec()` trực tiếp, mỗi lần gửi request Cassandra phải parse lại câu query. Rất tốn CPU.
**Giải pháp:** Dùng `session.Prepare()` một lần khi khởi tạo service.

### Ví dụ Repository với Prepared Statements

```go
package repository

import (
    "context"
    "time"
    "github.com/gocql/gocql"
    "myproject/internal/models"
)

type MessageRepository struct {
    session *gocql.Session
    
    // Cache các Prepared Statements
    insertStmt *gocql.Prepared
    queryStmt  *gocql.Prepared
}

func NewMessageRepository(session *gocql.Session) *MessageRepository {
    repo := &MessageRepository{session: session}
    
    // Prepare các câu SQL một lần
    repo.prepareStatements()
    return repo
}

func (r *MessageRepository) prepareStatements() {
    // 1. Prepare INSERT
    r.insertStmt, _ = r.session.Prepare(`
        INSERT INTO messages (conversation_id, bucket, message_id, sender_id, content, is_encrypted, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `)
    
    // 2. Prepare SELECT (Phải include Partition Key trong WHERE)
    r.queryStmt, _ = r.session.Prepare(`
        SELECT message_id, sender_id, content, is_encrypted, created_at 
        FROM messages 
        WHERE conversation_id = ? AND bucket = ? 
        ORDER BY created_at DESC LIMIT ?
    `)
}
```

---

## 13.5. Best Practice 2: Ghi tin nhắn (Write Path)

Khi ghi dữ liệu, ưu tiên tốc độ. Dùng `Unlogged Batch` nếu cần ghi nhiều tin nhắn vào **cùng một cuộc hội thoại**.

```go
func (r *MessageRepository) SaveMessage(ctx context.Context, msg *models.Message) error {
    // 1. Tính Bucket (Ví dụ: 202310)
    bucket := msg.CreatedAt.Year()*100 + int(msg.CreatedAt.Month())

    // 2. Execute PreparedStatement
    // Lưu ý: Dùng context.Context để hủy request nếu client timeout
    err := r.insertStmt.Query(
        ctx, 
        msg.ConversationID, 
        bucket, 
        msg.MessageID, 
        msg.SenderID, 
        msg.Content, 
        msg.IsEncrypted, 
        msg.CreatedAt,
    ).Exec()

    return err
}

// GHI NHIỀU TIN NHẮN (Batch)
func (r *MessageRepository) SaveMessagesBatch(ctx context.Context, msgs []*models.Message) error {
    batch := gocql.NewBatch(gocql.UnloggedBatch) // Unlogged nhanh hơn Logged
    
    for _, msg := range msgs {
        bucket := msg.CreatedAt.Year()*100 + int(msg.CreatedAt.Month())
        batch.Query(
            r.insertStmt,
            msg.ConversationID, bucket, msg.MessageID, msg.SenderID, msg.Content, msg.IsEncrypted, msg.CreatedAt,
        )
    }
    
    return r.session.ExecuteBatch(batch)
}
```

---

## 13.6. Best Practice 3: Phân trang (Pagination) với PagingState

Cassandra không hỗ trợ `OFFSET` (trượt). Bạn phải dùng **Paging State Token**.
*   Client gửi `cursor` (base64 string từ lần load trước).
*   Server trả về `next_cursor` (nếu còn tin).

```go
type PaginationResult struct {
    Messages   []*models.Message
    NextCursor string // Base64 encoded paging state
    HasMore    bool
}

func (r *MessageRepository) GetMessages(ctx context.Context, convID string, bucket int, limit int, cursor string) (*PaginationResult, error) {
    query := r.queryStmt.Bind(ctx, convID, bucket, limit)
    
    // Nếu có cursor, giải mã nó và gán vào query
    if cursor != "" {
        state, _ := gocql.BytesFromBase64(cursor)
        query.PageState(state)
    }
    
    iter := query.Iter()
    
    var messages []*models.Message
    scan := func(msg *models.Message, cols []gocql.ColumnInfo) error {
        // Scan dữ liệu vào struct message
        return iter.StructScan(&msg)
    }
    
    // Dùng Iterator để scan
    for iter.Scan(&scan) {
        // Logic append message...
    }
    
    if err := iter.Close(); err != nil {
        return nil, err
    }
    
    // Lấy PagingState cho trang tiếp theo
    var nextCursor string
    if iter.PageState() != nil {
        nextCursor = gocql.Base64Bytes(iter.PageState())
    }
    
    return &PaginationResult{
        Messages:   messages,
        NextCursor: nextCursor,
        HasMore:    nextCursor != "",
    }, nil
}
```

---

## 13.7. Best Practice 4: Quản lý TTL (Time To Live)

Bạn có thể tự động xóa dữ liệu cũ bằng TTL ngay trong câu lệnh INSERT.

```go
// Ghi tin nhắn tự xóa sau 30 ngày
func (r *MessageRepository) SaveMessageWithTTL(ctx context.Context, msg *models.Message) error {
    // Dùng INSERT với USING TTL
    stmt := `
        INSERT INTO messages (conversation_id, bucket, message_id, sender_id, content, created_at)
        VALUES (?, ?, ?, ?, ?, ?)
        USING TTL 2592000
    ` // 2592000 giây = 30 ngày
    
    return r.session.Query(
        stmt, 
        msg.ConversationID, 
        msg.Bucket(), 
        msg.MessageID, 
        msg.SenderID, 
        msg.Content, 
        msg.CreatedAt,
    ).Exec()
}
```

---

## 13.8. Best Practice 5: Retry Policy & Consistency

Để xử lý lỗi mạng tạm thời hoặc node down, cấu hình Retry Policy.

```go
// Custom Retry Policy: Thử lại 3 lần
type MyRetryPolicy struct {}

func (p *MyRetryPolicy) Attempt(q gocql.QueryInfo) bool {
    // Thử lại nếu lỗi là Timeout hoặc Unavailable
    return q.Attempts < 3
}
func (p *MyRetryPolicy) GetRetryType(q gocql.QueryInfo) gocql.RetryType {
    return gocql.Retry // Thử lại query cũ
}

// Áp dụng khi tạo cluster
cluster.RetryPolicy = &MyRetryPolicy{}
```

**Consistency Level:**
*   `LOCAL_QUORUM`: An toàn nhất cho ghi/đọc trong cùng Datacenter.
*   `ONE`: Nhanh nhất nhưng có thể đọc được dữ liệu cũ (Eventual Consistency). Dùng cho thống kê (Analytics) hoặc log.
*   `QUORUM`: Chậm hơn, nhưng nhất quán qua nhiều DC (ít dùng trừ khi cần global consistency ngay lập tức).

---

## 13.9. Chiến lược Migration (Đổi Schema)

Thay đổi Schema Cassandra (ví dụ: đổi Primary Key) rất nguy hiểm vì dữ liệu đã phân mảnh.

**Quy trình thay đổi Cột:**
1.  **Thêm cột mới:** `ALTER TABLE messages ADD COLUMN new_field text;` (An toàn, không khóa hệ thống).
2.  **Migrate Data:** Viết script Go để copy data từ cột cũ sang cột mới (chạy song song).
3.  **Thay đổi Code:** Switch app Go sang đọc cột mới.
4.  **Xóa cột cũ:** `ALTER TABLE messages DROP COLUMN old_field;`

**Quy trình thay đổi Keyspace/Table:**
*   Cần tạo Table mới, migrate toàn bộ data, đổi tên app, xóa table cũ. (Rất phức tạp).
*   **Lời khuyên:** Hãy thiết kế Schema đúng ngay từ đầu (tham khảo file `04-database-sharding-strategy.md`).

---

## 13.10. Debugging & Monitoring

### 1. Query Tracing
Nếu một query quá chậm, hãy bật tracing để xem Cassandra mất bao lâu để xử lý.

```go
iter := r.queryStmt.Bind(ctx, convID, bucket, 10).Trace().Iter()
// Sau khi iter.Close(), check iter.TraceInfo()
```

### 2. Gocql Metrics
Gocql có tích hợp sẵn metrics. Bạn nên export nó ra Prometheus để monitor số lượng query bị lỗi (ClosedPool, Timedout).

```go
import "github.com/gocql/gocql"
import "github.com/prometheus/client_golang/prometheus"

// Hook metrics vào Session
cluster.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.DCAwareRoundRobinPolicy("dc1"))
```

---

*Liên kết đến tài liệu tiếp theo:* `backend/project-structure.md` (đã hoàn thành) hoặc `flutter/architecture-state-management.md`