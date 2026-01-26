# 2. Tech Stack Decision Records (ADR)

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Accepted

## Giới thiệu
Tài liệu này ghi lại các quyết định kỹ thuật quan trọng (Technology Choices) cho hệ thống. Mỗi quyết định được phân tích dựa trên các yếu tố: Hiệu năng (Performance), Khả năng mở rộng (Scalability), Bảo mật (Security), và Chi phí phát triển (Development Cost).

---

## ADR-001: Chọn ngôn ngữ Backend là Go (Golang)

### Ngữ cảnh (Context)
Hệ thống cần xử lý lượng kết nối đồng thời cực lớn (hàng trăm nghìn WebSocket kết nối) và luồng dữ liệu video thời gian thực. Chúng ta cần một ngôn ngữ có hiệu năng cao nhưng không quá phức tạp để maintain.

### Các lựa chọn được xem xét
1.  **Node.js (TypeScript/JavaScript)**
2.  **Java / Kotlin**
3.  **Go (Golang)**

### Quyết định (Decision)
Chọn **Go (Golang)** làm ngôn ngữ chính cho toàn bộ các Microservices.

### Lý do (Rationale)
*   **Concurrency (Đa luồng):** Go sử dụng **Goroutines** và **Channels**, giúp xử lý hàng nghìn kết nối đồng thời với bộ nhớ cực thấp (khoảng 2KB stack mỗi goroutine so với 1MB thread của Java). Điều này cực kỳ quan trọng cho Chat Server và Signaling Server.
*   **Performance (Hiệu năng):** Go là ngôn ngữ biên dịch (Compiled), hiệu năng gần sát với C/C++, vượt trội hơn Node.js (Single-threaded) trong các tác vụ xử lý CPU nặng (như mã hóa AES/WebRTC packetizing).
*   **WebRTC Ecosystem:** Thư viện **Pion WebRTC** là thư viện WebRTC chuẩn mực viết bằng Go, cho phép chúng ta xây dựng SFU (Selective Forwarding Unit) mà không cần phụ thuộc vào Node.js hay C++.
*   **Deployment:** Biên dịch thành một file binary duy nhất, không cần cài đặt runtime (trái ngược với Java hay Node), rất dễ dàng deploy trên Docker container.

### Hậu quả (Consequences)
*   **Tích cực:** Hệ thống backend thống nhất ngôn ngữ, giảm độ trễ, chi phí hạ tầng thấp hơn (do tối ưu CPU/RAM).
*   **Tiêu cực:** Thị trường lập trình viên Go có thể ít hơn Java hoặc Node.js tại một số khu vực, cần đào tạo hoặc tuyển dụng kỹ sư có kinh nghiệm System Programming.

---

## ADR-002: Chọn Framework Frontend là Flutter

### Ngữ cảnh (Context)
Cần phát triển ứng dụng trên 4 nền tảng: Web, iOS, Android, và Desktop (Windows/Mac). Với ngân sách và đội ngũ giới hạn, việc phát triển native riêng cho từng nền tảng là bất khả thi.

### Các lựa chọn được xem xét
1.  **React Native**
2.  **Native Development** (Swift + Kotlin + React + Electron)
3.  **Flutter**

### Quyết định (Decision)
Chọn **Flutter** làm nền tảng phát triển giao diện đa nền tảng.

### Lý do (Rationale)
*   **Single Codebase:** Viết code một lần, chạy được trên Mobile, Web và Desktop. Điều này giúp giảm khoảng 60-70% mã nguồn và thời gian debug so với React Native hay Native.
*   **Performance:** Flutter không dùng WebView hay native bridge (như React Native). Nó có **Engine** riêng (Skia/Impeller) render widget trực tiếp lên màn hình. Điều này đảm bảo hiệu suất mượt mà cho các tác vụ nặng như **Video Call** hay **Animation 60fps**.
*   **Consistency:** Giao diện hiển thị "pixel-perfect" giống hệt nhau trên mọi thiết bị, tránh bug do khác biệt hệ điều hành.
*   **Desktop Support:** Flutter hỗ trợ Desktop rất tốt (Windows/Mac/Linux), giúp triển khai ứng dụng doanh nghiệp (SaaS) dễ dàng.

### Hậu quả (Consequences)
*   **Tích cực:** Tốc độ phát triển (TTM - Time to Market) cực nhanh. Dễ bảo trì codebase chung.
*   **Tiêu cực:** Kích thước ứng dụng ban đầu (APK/IPA) có thể lớn hơn native một chút do phải nhúng Engine. Đôi khi gặp khó khăn với các thư viện native chuyên biệt chưa có plugin Flutter (nhưng cộng đồng đang rất mạnh).

---

## ADR-003: Chọn WebRTC Engine là Pion (Go Native)

### Ngữ cảnh (Context)
Backend đã chọn là Go. Chúng ta cần triển khai tính năng Video Call với SFU (Selective Forwarding Unit) để hỗ trợ group call. Có nhiều giải pháp sẵn có (mã nguồn mở) nhưng hầu hết viết bằng Node.js.

### Các lựa chọn được xem xét
1.  **Mediasoup** (Node.js wrapper)
2.  **Jitsi / Janus** (C/C++)
3.  **Pion WebRTC** (Pure Go)
4.  **Agora / Twilio** (SaaS Third-party)

### Quyết định (Decision)
Chọn **Pion WebRTC** để tự xây dựng Media Server/Signaling.

### Lý do (Rationale)
*   **Consistent Stack:** Mediasoup rất mạnh nhưng yêu cầu Node.js. Việc lồng ghép Node.js vào hệ thống Go làm tăng độ phức tạp vận hành (Polyglot microservices). Pion giúp toàn bộ hệ thống video chạy trên Go.
*   **Flexibility:** Pion là thư viện low-level, cho phép tùy biến sâu logic signaling, routing packet, và tích hợp các thuật toán mã hóa riêng biệt mà không bị giới hạn bởi API của bên thứ 3.
*   **Cost:** Tránh phụ thuộc vào Agora/Twilio giúp giảm chi phí vận hành dài hạn (Pay-as-you-go rất đắt cho SaaS nếu user tăng mạnh).
*   **Performance:** Pion được tối ưu hóa cực tốt cho Go, tận dụng tối đa Goroutines để xử lý stream.

### Hậu quả (Consequences)
*   **Tích cực:** Kiểm soát hoàn toàn hạ tầng video, không bị khóa vendor, hiệu năng cao.
*   **Tiêu cực:** Phải tự xây dựng và bảo trì module SFU, cần đội ngũ có kiến thức sâu về WebRTC internals (STUN/TURN/ICE, DTLS, SRTP).

---

## ADR-004: Chiến lược Database Polyglot (Cassandra + CockroachDB + Redis)

### Ngữ cảnh (Context)
Hệ thống có hai loại dữ liệu chính: **Dữ liệu giao dịch** (User, Billing, Contacts) cần độ chính xác tuyệt đối; và **Dữ liệu luồng** (Messages, Call Logs) cần ghi chép cực nhanh và phân tán rộng. Một loại Database không thể tối ưu cho cả hai.

### Các lựa chọn được xem xét
1.  **One Database to rule them all** (PostgreSQL với JSONB)
2.  **MongoDB** (Document NoSQL)
3.  **Cassandra + CockroachDB + Redis** (Polyglot)

### Quyết định (Decision)
Áp dụng kiến trúc **Polyglot Persistence**:
1.  **CockroachDB:** Cho User, Billing, Contacts.
2.  **Cassandra:** Cho Messages, Call Logs.
3.  **Redis:** Cho Cache, Sessions, Pub/Sub.

### Lý do (Rationale)
*   **CockroachDB (NewSQL):** Là phiên bản phân tán của PostgreSQL. Nó hỗ trợ đầy đủ ACID (tính toàn vẹn dữ liệu) - điều bắt buộc cho tiền bạc (Billing) và quan hệ user-contact. Khả năng mở rộng ngang (Horizontal scaling) tốt hơn PostgreSQL truyền thống.
*   **Cassandra (NoSQL):** Được thiết kế cho *write-heavy workloads*. Với hệ thống nhắn tin, ghi tin nhắn diễn ra liên tục. Cassandra hỗ trợ sharding tự nhiên, không có single point of failure, latency thấp, hoàn hảo cho time-series data như lịch sử chat.
*   **Redis:** Chỉ có Redis mới đáp ứng được độ trễ sub-millisecond cho tính năng **Typing Indicator**, **User Presence**, và **Pub/Sub** để đẩy tin nhắn real-time.

### Hậu quả (Consequences)
*   **Tích cực:** Hiệu năng tối ưu cho từng loại dữ liệu. Hệ thống chịu tải tốt hơn gấp nhiều lần so với dùng chỉ PostgreSQL hay MongoDB.
*   **Tiêu cực:** Độ phức tạp vận hành tăng lên (cần maintain 3 loại DB khác nhau). Đội ngũ DevOps cần thành thạo cả SQL và NoSQL.

---

## ADR-005: Chiến lược AI "Hybrid" (Edge AI + Server AI)

### Ngữ cảnh (Context)
Yêu cầu hệ thống vừa phải bảo mật (E2EE) vừa phải thông minh (AI). Nếu mã hóa đầu cuối, Server không đọc được -> AI chết. Nếu không mã hóa, AI chạy được nhưng mất tính bảo mật.

### Các lựa chọn được xem xét
1.  **Server-side AI only** (Tắt E2EE toàn bộ).
2.  **Edge AI only** (Chạy AI trên điện thoại, tắt AI server).
3.  **Hybrid AI** (Tùy chọn Opt-out E2EE).

### Quyết định (Decision)
Chọn **Hybrid AI (Hướng B đã thảo luận)**: Cho phép người dùng tắt E2EE để kích hoạt Server AI, nhưng mặc định vẫn dùng Edge AI trên thiết bị.

### Lý do (Rationale)
*   **Tính linh hoạt:** SaaS phục vụ nhiều đối tượng khách hàng. Doanh nghiệp cần Recording (Ghi âm cuộc họp) -> Tắt E2EE -> Server AI xử lý. Cá nhân cần riêng tư -> Bật E2EE -> Edge AI (Google ML Kit) xử lý gợi ý trả lời trên máy.
*   **Bảo mật theo mặc định (Secure by Default):** Mặc định cài đặt của ứng dụng là E2EE bật. Người dùng phải chủ động tắt nếu muốn dùng tính năng "thông minh" hơn.
*   **Giảm tải Server:** Các tác vụ AI nhẹ (Smart Reply, Sentiment local) được đẩy xuống Flutter chạy bằng TFLite/ML Kit, giúp giảm chi phí API Server (OpenAI) và giảm độ trễ.

### Hậu quả (Consequences)
*   **Tích cực:** Đáp ứng được cả hai nhu cầu cực đoan: Bảo mật tuyệt đối và Tính năng thông minh mạnh mẽ.
*   **Tiêu cực:** Phức tạp hóa logic frontend (Flutter phải xử lý 2 luồng code: mã hóa/giải mã và gọi AI local). Backend phải xử lý điều kiện logic `if is_encrypted`.

---

## ADR-006: Chọn Messaging Protocol là WebSocket (Gorilla)

### Ngữ cảnh (Context)
Cần giao tiếp thời gian thực (Real-time) giữa Client và Server cho Chat và Signaling Video.

### Các lựa chọn được xem xét
1.  **Server-Sent Events (SSE):** Chỉ Server -> Client (một chiều).
2.  **HTTP Long Polling:** Cũ kỹ, tốn tài nguyên.
3.  **gRPC:** Hiệu năng cao nhưng không tốt qua trình duyệt/Internet (cần proxy).
4.  **WebSocket:** Full-duplex.

### Quyết định (Decision)
Sử dụng **WebSocket** với thư viện `Gorilla WebSocket` cho Go và `web_socket_channel` cho Flutter.

### Lý do (Rationale)
*   **Full-duplex:** Hỗ trợ gửi nhận tin nhắn song song (Chat 2 chiều) và Signaling (SDP Offer/Answer trao đổi liên tục).
*   **Low Latency:** Giữ kết nối mở, không cần thiết lập lại header như HTTP.
*   **Compatibility:** Được hỗ trợ mặc định trên mọi trình duyệt hiện đại và Flutter.
*   **Gorilla:** Là thư viện WebSocket chuẩn mực, ổn định nhất của hệ sinh thái Go.

### Hậu quả (Consequences)
*   **Tích cực:** Độ trễ thấp nhất cho Chat và Video.
*   **Tiêu cực:** Quản lý kết nối stateful phức tạp hơn HTTP. Cần xử lý logic Reconnection (tự động kết nối lại khi mất mạng) kỹ lưỡng trên Flutter.

---

## Tóm tắt Stack cuối cùng

| Lớp | Công nghệ chọn | Phân loại |
| :--- | :--- | :--- |
| **Frontend** | Flutter | Mobile/Web/Desktop Framework |
| **Backend** | Go (Golang) | Microservices Language |
| **API/Realtime** | Gin + Gorilla WebSocket | HTTP/WebSocket Server |
| **Video Engine** | Pion WebRTC | SFU & Media Server |
| **Database (OLTP)** | CockroachDB | Distributed SQL (Users/Billing) |
| **Database (NoSQL)**| Cassandra | Wide-Column (Messages/Logs) |
| **Cache** | Redis | In-memory Store |
| **Object Storage** | MinIO | S3 Compatible Storage |
| **AI (Edge)** | Google ML Kit / TFLite | On-device AI (Flutter) |
| **AI (Server)** | OpenAI API / Custom LLM | Cloud AI (Go wrapper) |
| **DevOps** | Docker + Kubernetes | Container Orchestration |

---

*Liên kết đến tài liệu tiếp theo:* `03-security-architecture.md`