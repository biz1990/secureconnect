# 7. Database Schema & Data Models

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 7.1. Tổng quan

Hệ thống sử dụng kiến trúc **Polyglot Persistence**. Mỗi loại dữ liệu sẽ được lưu trữ trong môi trường tối ưu nhất cho nó.
*   **CockroachDB:** Dữ liệu có cấu trúc quan hệ, cần tính toàn vẹn (ACID).
*   **Cassandra:** Dữ liệu dạng time-series (tin nhắn), cần ghi chép cực nhanh.
*   **Redis:** Dữ liệu tạm thời, tra cứu nhanh, và trạng thái real-time.

---

## 7.2. CockroachDB Schema (Relational Data)

Dữ liệu người dùng, cài đặt, danh bạ, và khóa mã hóa được lưu tại đây.

### 7.2.1. Users (Người dùng)
Lưu thông tin đăng ký và xác thực.

```sql
CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email STRING UNIQUE,
    username STRING UNIQUE,
    password_hash STRING, -- bcrypt hash
    display_name STRING,
    avatar_url STRING,
    status STRING DEFAULT 'offline', -- online, offline, busy
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index để tìm kiếm nhanh theo username (phục vụ autocomplete search)
CREATE INDEX idx_users_username ON users (username);
```

### 7.2.2. User Keys (Quản lý khóa E2EE)
Lưu Public Keys để server phân phối cho client. Private Keys **không bao giờ** xuất hiện ở đây.

```sql
-- Identity Key (Dài hạn)
CREATE TABLE identity_keys (
    user_id UUID PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    public_key_ed25519 STRING NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Signed Pre-Key (Trung hạn - Rotate hàng tuần)
CREATE TABLE signed_pre_keys (
    key_id INT PRIMARY KEY,
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    public_key STRING NOT NULL,
    signature STRING NOT NULL, -- Chữ ký bởi Identity Key
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (user_id, key_id)
);

-- One-Time Pre-Keys (Dùng một lần)
CREATE TABLE one_time_pre_keys (
    key_id INT PRIMARY KEY,
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    public_key STRING NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (user_id, key_id)
);
```

### 7.2.3. Contacts (Danh bạ)
Mối quan hệ giữa các user.

```sql
CREATE TABLE contacts (
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    contact_user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    status STRING DEFAULT 'pending', -- pending, accepted, blocked
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, contact_user_id)
);
```

### 7.2.4. Conversations Metadata (Thông tin cuộc hội thoại)
Chủ yếu lưu thông tin cho Group Chat. Đối với chat 1-1, `conversation_id` thường được hash từ `user_id_1` và `user_id_2`.

```sql
CREATE TABLE conversations (
    conversation_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type STRING NOT NULL, -- 'direct', 'group'
    name STRING, -- Tên nhóm (nếu là group)
    avatar_url STRING,
    created_by UUID REFERENCES users(user_id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Bảng mapping thành viên trong cuộc hội thoại
CREATE TABLE conversation_participants (
    conversation_id UUID REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    role STRING DEFAULT 'member', -- admin, member
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id)
);
```

### 7.2.5. Conversation Settings (Cài đặt Bảo mật & AI)
Đây là bảng quan trọng để quyết định luồng xử lý (Opt-out E2EE).

```sql
CREATE TABLE conversation_settings (
    conversation_id UUID PRIMARY KEY REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    is_e2ee_enabled BOOLEAN DEFAULT TRUE, -- Mặc định BẬT E2EE
    ai_enabled BOOLEAN DEFAULT FALSE,      -- Chỉ bật khi E2EE tắt (hoặc Edge AI)
    recording_enabled BOOLEAN DEFAULT FALSE,
    message_retention_days INT DEFAULT 30,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 7.2.6. Billing & Subscriptions (SaaS)
Quản lý gói dịch vụ.

```sql
CREATE TABLE subscriptions (
    subscription_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(user_id),
    plan_type STRING NOT NULL, -- free, pro, enterprise
    status STRING NOT NULL,     -- active, canceled, past_due
    start_date TIMESTAMPTZ DEFAULT NOW(),
    end_date TIMESTAMPTZ
);
```

---

## 7.3. Cassandra Schema (NoSQL - Messages)

Lưu trữ hàng tỷ tin nhắn với tốc độ ghi cực cao.

### 7.3.1. Messages (Bảng tin nhắn chính)
Sử dụng chiến lược **Bucketing** theo thời gian để tránh "Hot Partition".

```sql
CREATE KEYSPACE IF NOT EXISTS secureconnect_ks
  WITH replication = {'class': 'NetworkTopologyStrategy', 'replication_factor': 3};

USE secureconnect_ks;

CREATE TABLE messages (
    conversation_id UUID,
    bucket INT,              -- Bucket thời gian: 202310 (Năm/Tháng)
    message_id TIMEUUID,
    sender_id UUID,
    content TEXT,            -- Có thể là Ciphertext (Base64) hoặc Plaintext
    is_encrypted BOOLEAN,
    message_type STRING,     -- text, image, video, file
    metadata MAP<TEXT, TEXT>, -- Lưu kết quả AI (sentiment, summary) hoặc file info
    created_at TIMESTAMP,
    PRIMARY KEY ((conversation_id, bucket), created_at, message_id)
) WITH CLUSTERING ORDER BY (created_at DESC)
AND default_time_to_live = 2592000; -- 30 ngày tự xóa
```

### 7.3.2. User Messages By Sender (Index truy xuất)
Bảng này dùng để truy vấn nhanh "Tất cả tin nhắn tôi đã gửi" hoặc tìm kiếm tin nhắn theo người gửi.

```sql
CREATE TABLE user_messages_by_sender (
    sender_id UUID,
    bucket INT,
    message_id TIMEUUID,
    conversation_id UUID,
    content TEXT,
    is_encrypted BOOLEAN,
    created_at TIMESTAMP,
    PRIMARY KEY ((sender_id, bucket), created_at, message_id)
) WITH CLUSTERING ORDER BY (created_at DESC);
```

### 7.3.3. Conversation List View (Danh sách chat)
Bảng này giúp load danh sách cuộc hội thoại nhanh chóng mà không cần query toàn bộ bảng messages.

```sql
-- Mỗi khi có tin nhắn mới, ta cập nhật bảng này
CREATE TABLE conversation_list_view (
    user_id UUID,
    conversation_id UUID,
    updated_at TIMESTAMP,
    last_message_snippet TEXT,
    last_message_sender_id UUID,
    unread_count INT,
    PRIMARY KEY (user_id, updated_at)
) WITH CLUSTERING ORDER BY (updated_at DESC);
```

---

## 7.4. Redis Schema (In-Memory & Directory)

Sử dụng Key-Value patterns để quản lý trạng thái tạm thời và cache.

### 7.4.1. Global User Directory (Tra cứu nhanh)
Giải quyết bài toán shard CockroachDB.

```bash
# Key: user:email:{email_address}
# Type: String
# Value: user_uuid (UUID)
TTL: 86400 (24h) - Có thể persist nếu muốn
```

### 7.4.2. User Presence & Online Status
Quản lý trạng thái online/offline cho WebSocket.

```bash
# Key: presence:{user_id}
# Type: Hash
# Fields: 
#   status: "online" | "away" | "busy"
#   last_seen: timestamp (int)
#   device_ids: set(["dev_1", "dev_2"])

TTL: 300 (5 phút) - Nếu hết hạn nghĩa là offline
```

### 7.4.3. Signaling Store (Quản lý Video Call Room)
Lưu trạng thái người đang tham gia cuộc gọi để Signaling Server biết forward tín hiệu cho ai.

```bash
# Key: signal:room:{call_id}
# Type: Hash Map
# Fields:
#   "user_{user_id}": "{socket_connection_id}"
#   "created_at": timestamp

TTL: 3600 (1 giờ)
```

### 7.4.4. Pub/Sub Channels
Kênh thông báo real-time.

*   `chat:{user_id}`: Đẩy tin nhắn riêng tư.
*   `presence_updates`: Đẩy cập nhật trạng thái cho bạn bè.
*   `typing:{conversation_id}`: Đẩy sự kiện gõ phím.

### 7.4.5. Rate Limiting (Token Bucket)
Dùng để chống Spam/DDoS.

```bash
# Key: rate_limit:{user_id}:{endpoint_name}
# Type: String
# Value: remaining_tokens
```

---

## 7.5. Quan hệ dữ liệu (Data Relationships)

1.  **Luồng Nhắn tin:**
    *   Client (Flutter) gửi tin nhắn -> **Cassandra (messages)**.
    *   **Cassandra (user_messages_by_sender)** cũng được ghi song song để hỗ trợ tìm kiếm.
    *   Nếu `is_encrypted = false`, Server AI xử lý xong -> Update `metadata` trong Cassandra.
    *   Cập nhật **Cassandra (conversation_list_view)** để cập nhật "đã xem" hoặc snippet mới nhất.
    *   **Redis Pub/Sub** đẩy thông báo cho người nhận.

2.  **Luồng Video Call:**
    *   API tạo cuộc gọi -> Ghi Log vào **CockroachDB (call_logs)**.
    *   Client kết nối WS Signaling -> Ghi trạng thái vào **Redis (signal:room)**.
    *   Khi cuộc gọi kết thúc -> Xóa key trong Redis.

3.  **Luồng Auth:**
    *   Đăng ký -> Ghi vào **CockroachDB (users)**.
    *   Ghi mapping vào **Redis (user:email:...)** để tra cứu nhanh lần sau.

---

## 7.6. Migration & Rolling Updates

*   **CockroachDB:** Hỗ trợ schema migration online (migrating schema without downtime). Sử dụng tool `cockroachdb sql` hoặc integrate với migration tool của Go.
*   **Cassandra:** Cẩn thận khi thay đổi `PRIMARY KEY` hoặc `CLUSTERING ORDER`. Nếu cần sửa schema, thường phải tạo bảng mới (`new_messages`) và migrate data (ETL) rồi đổi tên bảng.
*   **Redis:** Là dữ liệu tạm thời, nếu đổi schema chỉ cần thay đổi code Go/Flutter, key cũ sẽ tự hết hạn TTL.

---

*Liên kết đến tài liệu tiếp theo:* `08-data-models-go-vs-dart.md`