# Maintenance & Troubleshooting Guide

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 22.1. Tổng quan

Khi hệ thống gặp sự cố (Incident), quy trình xử lý cần tuân theo quy tắc: **Nhận diện -> Kiểm tra Logs/Metrics -> Chẩn đoán -> Cách khắc phục**.

Hệ thống giám sát (Monitoring) chủ yếu dựa trên:
*   **Logs:** ELK Stack (Elasticsearch, Logstash, Kibana) hoặc Loki.
*   **Metrics:** Prometheus + Grafana.
*   **Traces:** Jaeger (để trace request qua nhiều services).
*   **Error Tracking:** Sentry (đuổi theo lỗi app Flutter).

---

## 22.2. Công cụ Chẩn đoán Cơ bản (Diagnostic Tools)

### 22.2.1. Lấy Logs từ Kubernetes
```bash
# Xem logs real-time của một service
kubectl logs -f deployment/chat-service -n secureconnect-prod

# Xem logs của pod cụ thể (nếu có nhiều replicas)
kubectl logs -f chat-service-xxxxx -n secureconnect-prod

# Xem logs của container trước đó (nếu pod đã restart)
kubectl logs chat-service-xxxxx -n secureconnect-prod --previous
```

### 22.2.2. Tìm lỗi theo Request ID
Khi một API trả về lỗi, Client luôn gửi `X-Request-ID` hoặc logs ghi `request_id`.
*   **ELK/Kibana:** Gõ `request_id: "req_xyz123"` vào thanh tìm kiếm để xem lịch sửữ request đi qua các services nào (API Gateway -> Chat -> DB).

### 22.2.3. Kiểm tra Trạng thái Pods
```bash
# Xem toàn bộ pods có vấn đề
kubectl get pods -n secureconnect-prod | grep -E "(ImagePullBackOff|CrashLoopBackOff|Error)"

# Mô tả chi tiết pod
kubectl describe pod chat-service-xxxxx -n secureconnect-prod
```

---

## 22.3. Backend Issues (Go Services)

### 22.3.1. Sự cố: Service không phản hồi (Timeout)

**Triệu chứng:** Client nhận `504 Gateway Timeout`.
**Nguyên nhân:**
1.  Database (Cassandra/Cockroach) quá tải (Slow query).
2.  Service bị treo (Deadlock hoặc Infinite loop).
3.  Pod CPU giới hạn quá thấp (CPU Throttling).

**Cách khắc phục:**
1.  **Check Logs:** Xem có log `"context deadline exceeded"` không.
2.  **Check Metrics (Prometheus):**
    *   `go_goroutines`: Nếu quá cao (>10,000) -> Có thể bị Goroutine leak.
    *   `rate(go_gc_duration_seconds)`: Nếu GC chạy quá thường -> Heap memory quá đầy.
3.  **Check DB:** Xem DB connection pool đã hết chưa (`database/sql: connection limit`).
4.  **Tạm thời:** Scale thêm Pods (`kubectl scale deployment/chat-service --replicas=6`).

### 22.3.2. Sự cố: Out of Memory (OOMKilled)

**Triệu chứng:** Pod thoát ngay lập tức, logs có `OOMKilled`.
**Nguyên nhân:** Service tiêu tốn RAM vượt quá giới hạn `limits.memory` trong K8s.

**Cách khắc phục:**
1.  Tăng `limits.memory` trong YAML Deployment (ví dụ: từ 512Mi lên 1Gi).
2.  Tìm bug Memory Leak trong code Go (ví dụ: Slice append vô hạn, chưa clear Map).
3.  **Optimize:** Giảm số lượng Worker Pool nếu dùng.

### 22.3.3. Sự cố: Lỗi E2EE Decryption Failed

**Triệu chứng:** User báo tin nhắn không mở được hoặc báo lỗi "Decryption failed".
**Nguyên nhân:**
1.  User A và User B không có chung Session Key (Key sync issue).
2.  Đã phát hiện Man-in-the-Middle (MITM) attack.
3.  Client (Flutter) đã xóa Private Keys của bạn cũ nhưng tin nhắn cũ vẫn dùng key đó.

**Cách khắc phục:**
1.  **Xem Logs Backend:** Lỗi này thường xuất hiện ở Client, nhưng Backend log API `/messages` sẽ thấy request.
2.  **Client Side:** Kiểm tra xem `Safety Number` (Mã an toàn) của hai bên có trùng nhau không.
3.  **Action:** Yêu cầu người dùng reset session (nhắn tin mới sẽ tạo session mới) hoặc "Reset Secure Session" trong Settings.

### 22.3.4. Sự cố: WebRTC Signaling Fail

**Triệu chứng:** Bấm gọi, máy đổ chuông nhưng không kết nối video (hoặc báo `Failed`).
**Nguyên nhân:**
1.  **ICE Candidate Failed:** Không tìm được đường đi mạng (Firewall chặn UDP).
2.  **TURN Server Down:** Nếu cả P2P và STUN fail, sẽ dùng TURN. Nếu Turn die -> Fail.
3.  **PeerConnection crash:** Backend (Pion) gặp lỗi logic.

**Cách khắc phục:**
1.  **Client Debug:** Trên trình duyệt Console (F12), xem `webkitRTCPeerConnection` logs.
2.  **Server Debug:** Xem logs `video-service`:
    ```bash
    kubectl logs -f deployment/video-service | grep ICE
    ```
3.  **Kiểm tra TURN:** Đảm bảo TURN (Coturn) đang chạy và có đủ UDP port mở.
4.  **Fallback:** Tắt video, thử chỉ Audio (Audio thường dễ穿过 Firewall hơn Video).

---

## 22.4. Frontend Issues (Flutter)

### 22.4.1. Sự cố: App Crash (Lúc mở lúc ẩn)

**Cách khắc phục:**
1.  **Sentry:** Vào dashboard Sentry -> Issues. Stack trace sẽ cho biết chính xác dòng code nào gây crash.
2.  **Common Causes:**
    *   `StateError`: Truy cập Provider khi chưa có `ProviderScope`.
    *   `MissingPluginException`: Plugin (Camera, Mic) chưa được cài đúng (Native link lỗi).
    *   `NetworkImageLoadException`: Load ảnh avatar lỗi (URL 404).

### 22.4.2. Sự cố: WebSocket (Chat) ngắt liên tục (Flapping)

**Triệu chứng:** Trạng thái Online/Offline nhấp nháy liên tục.
**Nguyên nhân:**
1.  Network người dùng không ổn định.
2.  Server (`chat-service` đang khởi động lại (Rolling Update)).
3.  **Keep-Alive timeout:** Client không gửi ping thường xuyên.

**Cách khắc phục:**
1.  **Client Code:** Kiểm tra logic `WebSocketChannel` có `reconnect` đúng không (Exponential Backoff: đợi 1s, 2s, 4s...).
2.  **Server Config:** Tăng `read_timeout` và `write_timeout` của Nginx/Load Balancer lên 3600s cho WebSocket.

### 22.4.3. Sự cố: UI không update (Riverpod State)

**Triệu chứng:** Nhắn tin xong nhưng danh sách chat không hiện tin mới.
**Nguyên nhân:**
1.  Lỗi trong logic `Riverpod`.
2.  Gọi hàm `ref.read(...)` trong `build()` thay vì `ref.watch(...)`.
3.  Lỗi Mutability: Update state trực tiếp (`state.count++`) thay vì dùng `copyWith()`.

**Cách khắc phục:**
1.  Run Widget Tests (`flutter test`) để bắt lỗi build logic.
2.  Thêm `debugPrint()` vào `notifier` để xem state có thay đổi thật không.

---

## 22.5. Database Issues (NoSQL & SQL)

### 22.5.1. Cassandra: Write Timeout

**Triệu chứng:** API gửi tin nhắn báo lỗi `write timeout`.
**Nguyên nhân:**
1.  **Overloaded:** Node Cassandra đang bị quá tải (CPU 100%).
2.  **Compaction:** Cassandra đang chạy process `compaction` dọn dẹp data (ăn CPU/Disk IO).
3.  **Slow Replica:** Một node trong cụm (Replication) quá chậm, khiến `LOCAL_QUORUM` không thể hoàn thành kịp.

**Cách khắc phục:**
1.  **Tạm thời:** Thay đổi Consistency Level trong Go code (tạm thời) xuống `LOCAL_ONE` cho lần đó (giải tỏa).
2.  **Về dài hạn:**
    *   Thêm node Cassandra mới.
    *   Tăng `write_request_timeout_in_ms` trong cấu hình driver `gocql`.
    *   Kiểm tra hardware (Disk IO, Network Latency).

### 22.5.2. CockroachDB: Schema Change Failed

**Triệu chứng:** Khi chạy migration (thêm cột bảng), bị treo hoặc timeout.
**Nguyên nhân:** CockroachDB đang chạy migration trên bảng dữ liệu lớn (ví dụ `messages`), không thể xong nhanh.

**Cách khắc phục:**
1.  **Không được hở (Kill) process:** Hãy để migration chạy.
2.  **Check Job:**
    ```sql
    SHOW JOBS;
    ```
3.  Nếu quá lâu, cân nhắc chia nhỏ migration (Split Migration: Tạo bảng mới -> Migrate data bằng script -> Drop bảng cũ).

---

## 22.6. Kubernetes Issues

### 22.6.1. ImagePullBackOff

**Triệu chứng:** Pod không thể start, status `ImagePullBackOff`.
**Nguyên nhân:**
1.  Tên Image hoặc Tag sai trong Deployment YAML.
2.  Lỗi xác thực đăng nhập Docker Registry.

**Cách khắc phục:**
1.  Kiểm tra secret `imagePullSecrets`.
2.  `kubectl describe pod` -> Xem section `Events` để xem thông báo lỗi chi tiết (401 Unauthorized, 404 Not Found).

### 22.6.2. CrashLoopBackOff

**Triệu chứng:** Pod start xong lại crash ngay lập tức, lặp đi lặp lại.
**Nguyên nhân:**
1.  Application startup fails (Lỗi config, connect DB failed).
2.  Liveness probe trả về fail liên tục.

**Cách khắc phục:**
1.  `kubectl logs <pod-name>`: Xem logs báo lỗi gì (DB password sai? Config missing?).
2.  Nếu logs thấy exit code 137 -> OOMKilled -> Tăng RAM.
3.  Tạm thời chỉnh `livenessProbe` `initialDelaySeconds` lên cao hơn (ví dụ 60s) để app có thời gian khởi động.

---

## 22.7. Quy trình xử lý Incident khẩn cấp (Major Incident)

Nếu hệ thống **Toàn bộ sập** (All services down) hoặc **Video Call không gọi được cho toàn hệ thống**:

### Bước 1: Tuyên bố Incident
*   Kênh thông báo: Slack/WhatsApp group On-call.
*   Đánh giá mức độ (Severity): P1 (Critical), P2 (High).

### Bước 2: Rollback (Hoàn tác)
*   Kiểm tra Deployment mới nhất có phải nguyên nhân không.
*   Nếu vừa deploy code mới < 10 phút -> **ROLLBACK NGAY**.
    ```bash
    kubectl rollout undo deployment/chat-service -n secureconnect-prod
    ```

### Bước 3: Scale Out (Mở rộng)
*   Tăng số lượng Replicas cho các services còn sống để chịu tải tạm.
    ```bash
    kubectl scale deployment/chat-service --replicas=10
    kubectl scale deployment/api-gateway --replicas=5
    ```

### Bước 4: Cắt tính năng nặng (Feature Toggle)
*   Nếu AI Service đang quá tải và làm chậm chat -> Tắt AI Feature tạm thời.
    *   Edit ConfigMap -> `AI_ENABLED: "false"`.
    *   Restart Pods.
*   Nếu Video Service quá tải -> Hạn chế số người tham gia cuộc gọi (Max Participants = 2).

### Bước 5: Post-Incident Review
*   Sau khi khắc phục, viết báo cáo **Post-mortem**:
    *   Nguyên nhân gốc là gì?
    *   Tại sao hệ thống không phát hiện sớm hơn?
    *   Action Item để ngăn chặn lại lần sau.

---

## 22.8. Checklist hàng ngày (Daily Health Check)

Để giảm thiểu sự cố, team DevOps nên check hàng ngày:

1.  **Grafana Dashboard:**
    *   CPU Usage < 70% (Tất cả services).
    *   Memory Usage < 80%.
    *   Error Rate (HTTP 5xx) < 0.1%.
2.  **Kubernetes:**
    *   Tất cả Pods trạng thái `Running`.
    *   Không có Pending Pods (do thiếu tài nguyên).
3.  **Database:**
    *   Disk Usage < 80% (Cassandra/Data partition).
    *   Connections pool không full.
4.  **Backup:**
    *   Check job backup CockroachDB/Redis chạy thành công đêm hôm qua chưa.

---

*Liên kết đến tài liệu tiếp theo:* `maintenance/on-call-runbook.md` (File này bổ sung chi tiết các lệnh cụ thể cho On-call engineer).