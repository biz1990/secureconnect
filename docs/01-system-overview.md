# 1. System Architecture Overview

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 1.1. Executive Summary

SecureConnect là một hệ thống liên lạc toàn diện (Chat, Video Call, Cloud Drive) được xây dựng theo mô hình **SaaS (Software as a Service)**. Hệ thống được thiết kế để xử lý lượng người dùng lớn với độ trễ thấp, đặc biệt chú trọng vào tính linh hoạt giữa **Bảo mật tuyệt đối (E2EE)** và **Trí tuệ nhân tạo (AI)**.

Hệ thống sử dụng kiến trúc **Microservices**, nơi **Go (Golang)** đóng vai trò xương sống xử lý backend hiệu năng cao, và **Flutter** đóng vai trò giao diện người dùng đa nền tảng (Web, Mobile, Desktop).

### Điểm khác biệt cốt lõi:
*   **Hybrid Security:** Mặc định kích hoạt mã hóa đầu cuối (E2EE) sử dụng Signal Protocol.
*   **Opt-out Encryption:** Cho phép người dùng tắt mã hóa cho các cuộc hội thoại cụ thể để kích hoạt tính năng AI nâng cao (Transcription, Sentiment Analysis) và Recording trên Server.

---

## 1.2. High-Level Architecture Diagram

Sơ đồ dưới đây mô tả sự tương tác giữa các thành phần chính trong hệ thống.

```mermaid
graph TD
    subgraph "Client Layer (Flutter)"
        WEB[Web App]
        MOBILE[iOS / Android]
        DESK[Desktop App]
    end

    subgraph "Entry Layer"
        LB[Load Balancer / Nginx]
        GW[API Gateway (Go)]
    end

    subgraph "Core Services (Microservices - Go)"
        AUTH[Auth Service<br/>JWT, OAuth2]
        CHAT[Chat Service<br/>Message Processing]
        VIDEO[Video Service<br/>Signaling & SFU Logic]
        AI[AI Service Wrapper<br/>Processing Logic]
        STOR[Storage Service<br/>File Management]
        BILL[Billing Service<br/>SaaS Subscriptions]
    end

    subgraph "Data Layer"
        REDIS[(Redis Cache<br/>Sessions/PubSub)]
        RDB[(CockroachDB<br/>Users, Billing)]
        NOSQL[(Cassandra/ScyllaDB<br/>Messages, Logs)]
        OBJ[Object Storage<br/>MinIO / S3]
    end

    %% Flows
    WEB --> LB
    MOBILE --> LB
    DESK --> LB
    LB --> GW

    %% API Routes
    GW --> AUTH
    GW --> CHAT
    GW --> VIDEO
    GW --> STOR
    GW --> BILL

    %% Internal Logic
    CHAT -- "is_encrypted=false" --> AI
    CHAT -- "is_encrypted=true" --> NOSQL
    
    VIDEO -- "SFU Stream" --> STOR
    VIDEO -- "Signaling WS" --> REDIS

    %% Data Access
    AUTH --> RDB
    AUTH --> REDIS
    CHAT --> NOSQL
    CHAT --> REDIS
    STOR --> OBJ

    %% External
    AI -.-> |Optional Ext. API| OPENAI[OpenAI / Llama]
```

---

## 1.3. Component Descriptions

### 1.3.1. Client Layer (Flutter)
*   **Mô tả:** Ứng dụng đa nền tảng được xây dựng bằng **Flutter Framework**.
*   **Trách nhiệm:**
    *   Quản lý giao diện người dùng (UI).
    *   Xử lý logic mã hóa/giải mã E2EE tại thiết bị (Client-side crypto).
    *   Kết nối WebRTC cho Video Call.
    *   Chạy Edge AI (Smart Reply, Sentiment offline) khi cần thiết.
*   **Các nền tảng:** Web (Flutter Web), iOS, Android, Windows/Mac/Linux (Flutter Desktop).

### 1.3.2. API Gateway (Go)
*   **Mô tả:** Cổng vào duy nhất cho tất cả các request từ Client.
*   **Tech:** Go + Gin Framework.
*   **Trách nhiệm:**
    *   Xác thực và ủy quyền (AuthN/AuthZ) qua JWT.
    *   Rate Limiting (Giới hạn tốc độ truy cập).
    *   Routing request đến các Microservice phù hợp.
    *   TLS Termination.

### 1.3.3. Auth Service
*   **Mô tả:** Quản lý danh tính người dùng và quyền truy cập.
*   **Tech:** Go + PostgreSQL/CockroachDB.
*   **Chức năng:**
    *   Đăng ký/Đăng nhập (Email/Password, OAuth2).
    *   Quản lý Refresh Tokens.
    *   Quản lý cài đặt bảo mật (2FA).
    *   Lưu trữ Public Keys cho E2EE (không bao giờ lưu Private Keys).

### 1.3.4. Chat Service
*   **Mô tả:** Xử lý luồng tin nhắn văn bản và tệp đính kèm.
*   **Tech:** Go + Gorilla WebSocket + Cassandra.
*   **Chức năng:**
    *   Nhận tin nhắn từ Client (dạng mã hóa hoặc văn bản thuần tùy cờ `is_encrypted`).
    *   Nếu `is_encrypted = false`: Chuyển tiếp nội dung đến **AI Service** để xử lý.
    *   Lưu trữ tin nhắn vào Cassandra.
    *   Quản lý trạng thái (Online/Offline, Typing) qua Redis Pub/Sub.

### 1.3.5. Video Service & Signaling
*   **Mô tả:** Quản lý kết nối thoại và hình ảnh thực (Video Call).
*   **Tech:** Go + Pion WebRTC (SFU - Selective Forwarding Unit).
*   **Chức năng:**
    *   **Signaling Server:** Trao đổi SDP (Offer/Answer) và ICE Candidates qua WebSocket riêng biệt (Port riêng để tối ưu hiệu năng).
    *   **SFU Logic:** Định tuyến luồng media giữa các peer, giảm tải cho thiết bị.
    *   **Recording:** Nếu cuộc gọi không được mã hóa E2EE, SFU sẽ forward stream đến Recording Service.

### 1.3.6. AI Service Wrapper
*   **Mô tả:** Module xử lý thông minh, hoạt động dựa trên cờ bảo mật.
*   **Tech:** Go (Wrapper) hoặc Python (Core Logic) gRPC.
*   **Chức năng:**
    *   Nhận văn bản thuần từ Chat Service.
    *   Gọi API LLM (OpenAI, Anthropic) hoặc Model nội bộ.
    *   Trả về kết quả: Sentiment Analysis, Summary, Translation.
    *   *Lưu ý:* Không bao giờ được gọi khi `is_encrypted = true`.

### 1.3.7. Storage Service
*   **Mô tả:** Quản lý "Ổ đĩa cá nhân" của người dùng.
*   **Tech:** Go + MinIO (S3 Compatible).
*   **Chức năng:**
    *   Nhận upload file từ Client (file đã được mã hóa bởi Client).
    *   Quản lý metadata và quyền truy cập.
    *   Tạo signed URL cho việc download.

---

## 1.4. The Hybrid Security Model (Chiến lược bảo mật lai)

Đây là đặc điểm quan trọng nhất của hệ thống. Hệ thống không áp dụng E2EE cứng nhắc mà cho phép chuyển đổi linh hoạt.

### Luồng A: Secure Mode (Mặc định - E2EE BẬT)
*   **Yêu cầu:** Bảo mật tuyệt đối, Server không được biết nội dung.
1.  **Client (Flutter):** Mã hóa tin nhắn bằng Public Key của người nhận (Signal Protocol).
2.  **Client -> Server:** Gửi dữ liệu đã mã hóa (`is_encrypted: true`).
3.  **Chat Service:** Nhận dữ liệu, **KHÔNG** giải mã. Chuyển thẳng vào Database.
4.  **AI Service:** Không được kích hoạt.
5.  **Database:** Lưu trữ ciphertext.

### Luồng B: Intelligent Mode (Tùy chọn - E2EE TẮT)
*   **Yêu cầu:** Cần AI dịch thuật, ghi âm cuộc họp, hoặc phân tích cảm xúc.
1.  **Client (Flutter):** Gửi dữ liệu văn bản thuần (`is_encrypted: false`). Lưu ý: Kết nối vẫn được bảo vệ qua HTTPS/TLS.
2.  **Client -> Server:** Gửi dữ liệu rõ ràng.
3.  **Chat Service:** Nhận dữ liệu.
4.  **AI Service:** Được kích hoạt. Phân tích nội dung, tạo metadata (sentiment, summary).
5.  **Database:** Lưu trữ nội dung rõ ràng (hoặc mã hóa bằng Server Key để bảo mật tại rest).

### Người dùng kiểm soát:
*   UI Flutter có **Switch/Toggle** để chuyển đổi chế độ này theo từng Conversation hoặc từng Cuộc gọi.

---

## 1.5. Data Storage Strategy

Hệ thống sử dụng chiến lược **Polyglot Persistence** (sử dụng nhiều loại database cho các mục đích khác nhau) để tối ưu hóa hiệu năng.

| Dữ liệu (Data) | Loại Database | Lý do chọn |
| :--- | :--- | :--- |
| **User Profiles, Billing, Contacts** | **CockroachDB (SQL)** | Cần tính toàn vẹn giao dịch ACID mạnh mẽ, quan hệ phức tạp, độ nhất quán cao. |
| **Messages, Call Logs** | **Cassandra/ScyllaDB (NoSQL)** | Dữ liệu dạng time-series, ghi chép cực nhanh, khả năng đọc ghi lớn, phân tán dễ dàng. |
| **Sessions, Online Status, Typing** | **Redis (In-memory)** | Độ trễ cực thấp (sub-millisecond), cần Pub/Sub cho real-time. |
| **Files (Images, Videos)** | **MinIO / S3 (Object Storage)** | Lưu trữ file lớn, không cấu trúc, tích hợp dễ dàng với CDN. |
| **Keys Directory** | **Redis** | Tra cứu nhanh `email` -> `user_id` để xử lý đăng nhập/check tồn tại. |

### Sharding Strategy (Chiến lược phân mảnh)
*   **User Data:** Shard theo `user_id` (Hash sharding). Sử dụng một Global Directory (Redis) để tra cứu nhanh vị trí shard.
*   **Message Data:** Partition theo `conversation_id` và `bucket` (thời gian) để tránh một partition quá lớn.

---

## 1.6. Technology Stack Summary

| Layer | Technology | Purpose |
| :--- | :--- | :--- |
| **Frontend** | **Flutter** | Cross-platform UI (Web, iOS, Android, Desktop). |
| **Backend** | **Go (Golang)** | High-performance Microservices. |
| **API Framework** | **Gin / Fiber** | HTTP Router & Middleware. |
| **Real-time** | **Gorilla WebSocket** | Chat connections. |
| **Video Engine** | **Pion WebRTC** | Pure Go WebRTC implementation (SFU). |
| **Cryptography** | **libsodium** (Go) / **cryptography** (Flutter) | Signal Protocol implementation. |
| **Databases** | **CockroachDB, Cassandra, Redis** | Persistent storage & Caching. |
| **AI / ML** | **Python (FastAPI) / Go gRPC** | AI Processing server-side (when opt-out). |
| **DevOps** | **Docker, Kubernetes** | Containerization & Orchestration. |
| **Monitoring** | **Prometheus, Grafana** | Metrics & Visualization. |

---

## 1.7. Scalability & Performance Goals

1.  **Concurrent Connections:** Hệ thống WebSocket cần hỗ trợ **100,000+ kết nối đồng thời** trên một instance Chat Service (tối ưu bằng Go epoll).
2.  **Message Latency:** Thời gian nhận tin nhắn từ Người A -> Người B **< 200ms** (trong điều kiện mạng bình thường).
3.  **Video Quality:** Hỗ trợ **720p@30fps** cho group call (4-10 người) với adaptive bitrate.
4.  **High Availability:** Các dịch vụ Core phải đạt được **99.9% Uptime**. Sử dụng Kubernetes để tự động restart pod khi lỗi.

---

## 1.8. Next Steps

Sau khi nắm vững tổng quan này, đội ngũ phát triển nên tham khảo các tài liệu chi tiết sau:
*   `02-tech-stack-decision.md`: Tại sao chọn các công nghệ trên.
*   `03-security-architecture.md`: Chi tiết thuật toán mã hóa Signal Protocol.
*   `api-openapi-spec.yaml`: Định nghĩa chi tiết các API endpoint.