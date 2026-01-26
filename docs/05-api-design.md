# 5. API Design & Standards

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 5.1. Tổng quan

API của SecureConnect tuân theo kiến trúc **RESTful** sử dụng giao thức **HTTPS**. Tất cả dữ liệu trao đổi đều dưới định dạng **JSON**.

Do hệ thống có tính chất đặc biệt (Hybrid E2EE), thiết kế API phải đảm bảo tính linh hoạt để nhận và xử lý cả dữ liệu đã mã hóa (Ciphertext) và dữ liệu văn bản thuần (Plaintext).

### Base URL Structure
*   **Production:** `https://api.secureconnect.com/v1`
*   **Staging:** `https://api-staging.secureconnect.com/v1`
*   **WebSocket:** `wss://signal.secureconnect.com/v1/ws`

---

## 5.2. Các chuẩn mực chung (General Standards)

### 5.2.1. Định dạng Tên (Naming Conventions)
*   **Endpoint URLs:** Sử dụng `kebab-case` (ví dụ: `/user-profiles`, `/call-records`).
*   **JSON Keys:** Sử dụng `camelCase` (ví dụ: `userId`, `accessToken`, `createdAt`). Điều này tương thích với mặc định của Go (JSON tags) và Flutter (json_serializable).
*   **Database Fields:** Sử dụng `snake_case` trong DB, convert sang `camelCase` khi trả về API.

### 5.2.2. Định danh (IDs)
*   Tất cả các ID trong hệ thống sử dụng **UUID v4** (dạng String).
*   Ví dụ: `"user_id": "550e8400-e29b-41d4-a716-446655440000"`
*   Lý do: Không thể dự đoán (security), dễ dàng tạo ra distributed mà không xung đột.

### 5.2.3. Định dạng Thời gian (Timestamps)
*   Sử dụng chuẩn **ISO 8601** ở múi giờ **UTC**.
*   Ví dụ: `"2023-10-27T10:00:00Z"`
*   Client (Flutter) chịu trách nhiệm chuyển đổi sang múi giờ local của người dùng khi hiển thị.

---

## 5.3. Thiết kế Đ yêu cầu & Trả lời (Request & Response Design)

### 5.3.1. Cấu trúc Request
Mọi request (trừ Upload file) đều phải có Header:
```http
Content-Type: application/json
Authorization: Bearer <JWT_ACCESS_TOKEN>
X-Request-ID: <uuid-v4> (Khuyến khích để trace log)
```

### 5.3.2. Cấu trúc Response Envelope
Tất cả các response API đều được bọc trong một đối tượng chuẩn (Envelope) để dễ dàng xử lý lỗi chung trên client.

**Thành công (Success):**
```json
{
  "success": true,
  "data": {
    // Dữ liệu thực tế (User, Message, v.v.)
  },
  "meta": {
    "request_id": "req_12345",
    "timestamp": "2023-10-27T10:00:00Z"
  }
}
```

**Thất bại (Error):**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_PAYLOAD",
    "message": "Email format is invalid",
    "details": {
      "field": "email",
      "reason": "must contain @"
    }
  },
  "meta": {
    "request_id": "req_12345",
    "timestamp": "2023-10-27T10:00:00Z"
  }
}
```

---

## 5.4. Chi tiết các nhóm API chính

### 5.4.1. Authentication API (`/auth`)
Xử lý đăng ký, đăng nhập và quản lý token.

*   **POST `/v1/auth/register`**
    *   **Input:** `email`, `password`, `full_name`.
    *   **Logic:** Kiểm tra email qua Redis Directory -> Hash password -> Tạo user -> Return JWT tokens.
*   **POST `/v1/auth/login`**
    *   **Input:** `email`, `password`, `device_id`.
    *   **Logic:** Verify hash -> Tạo Access Token (15p) & Refresh Token (30d).
*   **POST `/v1/auth/refresh`**
    *   **Input:** `refresh_token`.
    *   **Logic:** Validate Refresh token -> Cấp Access token mới.

### 5.4.2. Messaging API (`/messages`)
Đây là API phức tạp nhất do cơ chế Hybrid E2EE.

*   **POST `/v1/messages`** (Gửi tin nhắn)
    *   **Input:**
        ```json
        {
          "conversation_id": "uuid",
          "content": "Base64String or PlainText",
          "is_encrypted": true,  <-- CỜ QUAN TRỌNG
          "content_type": "text", // image, video, file
          "metadata": {
            "reply_to_message_id": "uuid",
            "client_nonce": "string" // Dùng cho E2EE logic
          }
        }
        ```
    *   **Logic Server:**
        *   Nếu `is_encrypted = true`: Lưu thẳng vào Cassandra (Cột `encrypted_content`).
        *   Nếu `is_encrypted = false`: Gửi nội dung đến AI Service -> Lấy kết quả (Sentiment/Summary) -> Lưu vào Cassandra (Cột `content` và `ai_metadata`).

*   **GET `/v1/messages`** (Lấy lịch sử tin nhắn)
    *   **Input:** `conversation_id`, `limit=50`, `cursor="..."`.
    *   **Pagination:** Sử dụng **Cursor-based pagination** (dựa trên `created_at` và `message_id`) thay vì `page/offset`.
    *   **Lý do:** Tin nhắn dạng time-series, cursor giúp lấy tin nhắn mới nhất rất nhanh mà không bị lỗi khi có tin nhắn mới chèn vào giữa.

*   **Output Example:**
        ```json
        {
          "success": true,
          "data": {
            "messages": [
              {
                "message_id": "uuid",
                "sender_id": "uuid",
                "content": "...", // Flutter tự giải mã nếu cần
                "is_encrypted": true,
                "created_at": "2023-10-27T10:00:00Z",
                "ai_metadata": null // Nếu E2EE thì null
              }
            ],
            "next_cursor": "MjAyMy0xMC0yN1QxMDowMDowMFpfbXNnXzEyMw=="
          }
        }
        ```

### 5.4.3. Call Management API (`/calls`)
Quản lý logic cuộc gọi (Signaling dùng WebSocket riêng, nhưng dùng REST để bắt đầu/kết thúc log).

*   **POST `/v1/calls/initiate`**
    *   **Input:** `participant_ids` (array), `call_type` (audio/video), `is_encrypted` (true/false).
    *   **Output:** `call_id`, `turn_servers` (credentials), `signaling_url`.

*   **POST `/v1/calls/{id}/end`**
    *   **Logic:** Ghi tổng thời gian, update trạng thái cuộc gọi trong DB.

### 5.4.4. Key Management API (`/keys`)

*   **POST `/v1/keys/upload`**
    *   **Input:**
        ```json
        {
          "identity_key": "...",
          "signed_pre_key": "...",
          "one_time_pre_keys": [...]
        }
        ```
    *   **Logic:** Lưu Public Keys vào CockroachDB/Redis. Server không bao giờ nhận Private Keys.

*   **GET `/v1/keys/{user_id}`**
    *   **Logic:** Trả về Public Keys của user đó để Client thiết lập E2EE session.

### 5.4.5. Storage API (`/storage`)

*   **POST `/v1/storage/upload-url`**
    *   Để giảm tải cho Go server, ta sử dụng **Pre-signed URL**.
    *   **Logic:** Client gọi API này -> Go server tạo pre-signed URL của MinIO/S3 -> Trả về URL cho Client -> Client upload trực tiếp lên MinIO (không đi qua Go).
    *   **Input:** `file_name`, `file_type`, `file_size`.

---

## 5.5. Xử lý Lỗi & Mã trạng thái (Error Handling)

Sử dụng chuẩn HTTP Status Codes kết hợp với mã lỗi nội bộ.

| HTTP Code | Ý nghĩa | Ví dụ Internal Code |
| :--- | :--- | :--- |
| **200 OK** | Thành công | - |
| **201 Created** | Tạo resource thành công | - |
| **400 Bad Request** | Dữ liệu đầu vào sai cấu trúc | `INVALID_PAYLOAD`, `MISSING_FIELD` |
| **401 Unauthorized** | Chưa đăng nhập hoặc Token hết hạn | `TOKEN_EXPIRED`, `INVALID_TOKEN` |
| **403 Forbidden** | Đã đăng nhập nhưng không đủ quyền | `PERMISSION_DENIED`, `E2EE_BLOCKED` |
| **404 Not Found** | Resource không tồn tại | `USER_NOT_FOUND`, `MESSAGE_NOT_FOUND` |
| **409 Conflict** | Xung đột dữ liệu | `EMAIL_ALREADY_EXISTS`, `DUPLICATE_MESSAGE` |
| **429 Too Many Requests** | Gọi quá nhanh (Rate limit) | `RATE_LIMIT_EXCEEDED` |
| **500 Internal Server Error** | Lỗi server nghiêm trọng | `INTERNAL_ERROR`, `DB_CONNECTION_FAILED` |

### Chiến lược Retry cho Client (Flutter)
*   **429:** Client phải đợi theo header `Retry-After` rồi mới thử lại.
*   **500:** Client nên thử lại tối đa 3 lần với exponential backoff (1s, 2s, 4s).
*   **401:** Client phải dùng Refresh Token để lấy Access Token mới, không retry ngay lập tức.

---

## 5.6. Pagination Strategy (Chiến lược phân trang)

Do tính chất đặc thù của hệ thống Chat, chúng ta sử dụng 2 chiến lược phân trang khác nhau:

### 5.6.1. Cursor-based Pagination (Dành cho Messages)
*   **Dùng cho:** `/messages`, `/call-logs`.
*   **Cách hoạt động:** Client gửi `cursor` (được server trả về ở request trước). Server query database "nhỏ hơn thời gian của cursor này".
*   **Ưu điểm:** Không bị trùng lặp hoặc bỏ sót tin nhắn khi có tin nhắn mới đến giữa các lần load trang.

### 5.6.2. Offset-based Pagination (Dành cho Admin/List)
*   **Dùng cho:** `/users/search`, `/admin/billing`.
*   **Cách hoạt động:** `page=1&limit=20`.
*   **Ưu điểm:** Dễ nhảy đến trang bất kỳ (ví dụ: vào thẳng trang 5).

---

## 5.7. Rate Limiting (Giới hạn tốc độ)

Để bảo vệ hệ thống khỏi DDoS và Spam, áp dụng giới hạn sau:

| Endpoint | Limit | Window | Algorithm |
| :--- | :--- | :--- | :--- |
| `/auth/login`, `/auth/register` | 5 requests | 1 phút | Token Bucket |
| `/messages` (POST) | 100 requests | 1 phút | Token Bucket |
| `/storage/upload` | 10 requests | 1 phút | Fixed Window |
| Các API khác | 1000 requests | 1 giờ | Leaky Bucket |

**Response Headers:**
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1698400000
```

---

## 5.8. API Versioning (Phiên bản hóa)

Sử dụng **URL Path Versioning**.
*   Phiên bản hiện tại: `/v1/...`
*   Khi có breaking change (làm vỡ cấu trúc cũ): Tạo `/v2/...`
*   **Policy:** Giữ phiên bản cũ (v1) chạy song song ít nhất 6 tháng sau khi ra mắt v2 để cho các Client cũ kịp cập nhật.

---

## 5.9. Security Headers

Mọi response từ API Gateway phải đi kèm các Header bảo mật:

```http
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'
```

---

## 5.10. Tài liệu OpenAPI (Swagger)

Các định nghĩa chi tiết hơn về mỗi field (data type, required/optional, example) sẽ được viết trong file `api-openapi-spec.yaml`. File này sẽ được import vào công cụ **Swagger UI** hoặc **Postman** để team Frontend dễ dàng test thử.

**Link file:** [Link đến `api-openapi-spec.yaml`]

---

*Liên kết đến tài liệu tiếp theo:* `06-websocket-signaling-protocol.md`