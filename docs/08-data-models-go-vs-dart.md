# 8. Data Models Mapping: Go vs Dart

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 8.1. Tổng quan

Để đảm bảo tính toàn vẹn dữ liệu khi giao tiếp giữa **Go (REST API/WebSocket)** và **Flutter (Client)**, chúng ta cần quy định chặt chẽ cách mapping các kiểu dữ liệu (Types), cấu trúc JSON (Serialization), và quy tắc đặt tên (Naming Conventions).

Tài liệu này cung cấp các cặp code mẫu (Go Struct <-> Dart Class) cho các entity quan trọng nhất.

---

## 8.2. Bảng映射 Kiểu dữ liệu (Primitive Type Mapping)

| Go Type | Dart/Flutter Type | JSON String Representation | Ghi chú |
| :--- | :--- | :--- | :--- |
| `string` | `String` | `"text"` | Chuỗi văn bản. |
| `int`, `int32` | `int` | `123` | Số nguyên. |
| `int64` | `int` | `123456789` | Dart số nguyên có độ lớn 64 bit (với web hỗ trợ JS number). |
| `float32`, `float64` | `double` | `12.34` | Số thực. |
| `bool` | `bool` | `true` / `false` | Boolean. |
| `time.Time` | `DateTime` | `"2023-10-27T10:00:00Z"` | Chuẩn RFC3339 (UTC). |
| `uuid.UUID` | `String` | `"550e8400-e29b..."` | Trong JSON, UUID luôn được serialize thành String. |
| `[]byte` | `String` | `"SGVsbG8gV29ybGQ..."` | Sử dụng Base64 encoding. |
| `[]T` (Slice) | `List<T>` | `[...]` | Mảng/Danh sách. |
| `map[string]T` | `Map<String, T>` | `{...}` | Từ điển/Object. |

---

## 8.3. Thư viện hỗ trợ (Dependencies)

### 8.3.1. Go (Backend)
*   **JSON Serialization:** Sử dụng thư viện chuẩn `encoding/json`.
*   **UUID:** Sử dụng `github.com/google/uuid`.
*   **Time:** Sử dụng `time` package.

### 8.3.2. Dart (Frontend - Flutter)
*   **JSON Serialization:** Khuyến khích dùng `json_serializable` (code generation) để tránh lỗi runtime và an toàn kiểu.
*   **UUID:** Sử dụng package `uuid` (hoặc tự parse string).
*   **Utils:** Cần có hàm util để chuyển String -> DateTime.

---

## 8.4. Chi tiết Mapping các Entity quan trọng

### 8.4.1. Entity: User (Người dùng)

**Mô tả:** Thông tin hồ sơ người dùng.

#### Code Go (Backend)
```go
package models

import (
    "time"
    "github.com/google/uuid"
)

type User struct {
    UserID      uuid.UUID `json:"user_id"`
    Email       string    `json:"email"`
    Username    string    `json:"username"`
    DisplayName string    `json:"display_name"`
    AvatarURL   string    `json:"avatar_url,omitempty"`
    Status      string    `json:"status"` // online, offline
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

#### Code Dart (Frontend - Flutter)
```dart
import 'package:json_annotation/json_annotation.dart';

part 'user.g.dart';

@JsonSerializable()
class User {
  final String userId;      // uuid.UUID -> String
  final String email;
  final String username;
  final String displayName;
  final String? avatarUrl;  // Optional field
  final String status;
  final DateTime createdAt;

  factory User.fromJson(Map<String, dynamic> json) => _$UserFromJson(json);
  Map<String, dynamic> toJson() => _$UserToJson(this);
}
```

---

### 8.4.2. Entity: Message (Tin nhắn) - Hybrid E2EE

**Mô tả:** Đối tượng phức tạp nhất. `content` có thể là văn bản thuần hoặc Ciphertext tùy thuộc vào cờ `is_encrypted`.

#### Code Go (Backend)
```go
package models

import (
    "time"
    "github.com/google/uuid"
)

type Message struct {
    MessageID      uuid.UUID                 `json:"message_id"`
    ConversationID uuid.UUID                 `json:"conversation_id"`
    SenderID       uuid.UUID                 `json:"sender_id"`
    Content        string                     `json:"content"` // Có thể là Plain hoặc Base64 Ciphertext
    IsEncrypted    bool                       `json:"is_encrypted"` // Cờ quan trọng
    ContentType    string                     `json:"content_type"` // text, image, video
    Metadata       map[string]interface{}     `json:"metadata,omitempty"` // Lưu AI data hoặc File info
    CreatedAt      time.Time                  `json:"created_at"`
}
```

#### Code Dart (Frontend - Flutter)
```dart
import 'package:json_annotation/json_annotation.dart';

part 'message.g.dart';

@JsonSerializable(explicitToJson: true)
class Message {
  final String messageId;
  final String conversationId;
  final String senderId;
  
  @JsonKey(name: 'content')
  final String content;
  
  @JsonKey(name: 'is_encrypted')
  final bool isEncrypted;
  
  @JsonKey(name: 'content_type')
  final String contentType;
  
  @JsonKey(name: 'metadata')
  final Map<String, dynamic>? metadata; // AI results or file info
  
  final DateTime createdAt;

  factory Message.fromJson(Map<String, dynamic> json) => _$MessageFromJson(json);
  Map<String, dynamic> toJson() => _$MessageToJson(this);
}
```

---

### 8.4.3. Entity: AI Metadata (Dữ liệu AI)

**Mô tả:** Chỉ tồn tại trong `message.metadata` nếu `is_encrypted == false`.

#### JSON Example
```json
{
  "sentiment": "positive",
  "confidence": 0.95,
  "summary": "User is happy",
  "smart_replies": ["Great!", "Thanks"]
}
```

#### Code Go (Backend - Type Alias)
```go
// Dùng map[string]interface{} để linh hoạt
type AIMetadata map[string]interface{}
```

#### Code Dart (Frontend - Model)
```dart
@JsonSerializable()
class AIMetadata {
  final String sentiment;
  final double confidence;
  final String? summary;
  final List<String>? smartReplies;

  factory AIMetadata.fromJson(Map<String, dynamic> json) => _$AIMetadataFromJson(json);
  Map<String, dynamic> toJson() => _$AIMetadataToJson(this);
}
```

---

### 8.4.4. Entity: Signaling Payload (WebRTC)

**Mô tả:** Dữ liệu trao đổi qua WebSocket Signaling. Do Go WebSocket library thường nhận `[]byte`, ta cần parse từ JSON string.

#### Code Go (Backend)
```go
package ws_models

type SignalingMessage struct {
    Type    string                 `json:"type"` // offer, answer, ice_candidate
    CallID  string                 `json:"call_id"`
    Payload map[string]interface{} `json:"payload"`
}
```

#### Code Dart (Frontend - Flutter)
```dart
@JsonSerializable()
class SignalingMessage {
  final String type; // offer, answer, ice_candidate, join, leave
  final String callId;
  final Map<String, dynamic>? payload;

  factory SignalingMessage.fromJson(Map<String, dynamic> json) => _$SignalingMessageFromJson(json);
  Map<String, dynamic> toJson() => _$SignalingMessageToJson(this);
}
```

---

## 8.5. Quy tắc xử lý Timestamp & Date

### 8.5.1. Backend (Go)
*   **Gửi đi:** `time.Time` tự động serialize thành chuỗi RFC3339 (ví dụ: `2023-10-27T10:00:00Z`) khi dùng `json.Marshal`.
*   **Nhận về:** Khi unmarshal JSON từ client, Go tự nhận diện format chuẩn RFC3339.

### 8.5.2. Frontend (Dart)
*   **Nhận về:** Dùng `DateTime.parse(jsonString)`. Chuỗi JSON từ Go tuân thủ chuẩn ISO 8601 nên `DateTime.parse` hiểu được.
*   **Gửi đi:** Dùng `toIso8601String()` để chuyển `DateTime` thành chuỗi trước khi gửi request.
    ```dart
    // Ví dụ gửi request
    final payload = {
      "scheduled_at": myDateTime.toIso8601String(), // 2023-10-27T10:00:00.000Z
    };
    ```

---

## 8.6. Quy tắc xử lý UUID

*   **Backend (Go):** Trong struct, dùng `uuid.UUID`. Khi gửi JSON, field này sẽ xuất hiện dưới dạng String.
*   **Frontend (Dart):** Xử lý UUID dưới dạng `String` thuần túy.
    *   Nếu cần tính toán (ví dụ v4), dùng thư viện `uuid` package trong Dart.
    *   Khi nhận API: `String userId = json['user_id'];`

---

## 8.7. Enum Handling (Xử lý Liệt kê)

Cần thống nhất tên các giá trị Enum giữa hai bên.

### Ví dụ: Message Type

#### Go (Backend)
```go
type MessageType string

const (
    MessageTypeText   MessageType = "text"
    MessageTypeImage  MessageType = "image"
    MessageTypeVideo  MessageType = "video"
    MessageTypeAudio  MessageType = "audio"
    MessageTypeFile   MessageType = "file"
)
```

#### Dart (Frontend)
```dart
enum MessageType {
  @JsonValue("text") text,
  @JsonValue("image") image,
  @JsonValue("video") video,
  @JsonValue("audio") audio,
  @JsonValue("file") file,
}
```
*Lưu ý:* Sử dụng `@JsonValue` annotation trong Dart để đảm bảo serialize string đúng như Go mong đợi.

---

## 8.8. Error Response Object Mapping

Cả hai bên cần đồng bộ cấu trúc lỗi.

#### Go (Backend)
```go
type ErrorResponse struct {
    Success bool   `json:"success"`
    Error   ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

#### Dart (Frontend)
```dart
@JsonSerializable()
class ErrorResponse {
  final bool success;
  final ErrorDetail error;

  factory ErrorResponse.fromJson(Map<String, dynamic> json) => _$ErrorResponseFromJson(json);
}

@JsonSerializable()
class ErrorDetail {
  final String code;
  final String message;

  factory ErrorDetail.fromJson(Map<String, dynamic> json) => _$ErrorDetailFromJson(json);
}
```

---

## 8.9. Lệnh Code Generation cho Flutter

Vì chúng ta dùng `json_serializable`, team Flutter cần chạy lệnh sau mỗi khi thay đổi file Model:

```bash
# Cài đặt build_runner (lần đầu)
flutter pub add build_runner json_annotation

# Chạy code generation
flutter pub run build_runner build --delete-conflicting-outputs
```

Lệnh này sẽ tự động tạo ra file `user.g.dart`, `message.g.dart` chứa các hàm `fromJson`, `toJson`.

---

*Liên kết đến tài liệu tiếp theo:* `backend/project-structure.md`