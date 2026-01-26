# Maintenance & On-Call Runbook

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 23.1. Tổng quan

Mục tiêu của cuốn Runbook này là hướng dẫn kỹ sư trực (On-Call Engineer) khôi phục lại trạng thái hoạt động bình thường cho hệ thống **SecureConnect** nhanh nhất có thể (MTTR - Mean Time To Restore thấp nhất).

**Nguyên tắc cốt lõi:**
1.  **Tính ưu tiên hàng đầu là Dữ liệu:** Không thực hiện lệnh có thể làm mất dữ liệu (DROP DATABASE, DELETE...) trong lúc hoảng loạn.
2.  **Truyền thông:** Hãy thông báo cho team ngay khi nhận alert, đừng cố gắng sửa chữa một mình quá 15 phút mà không cập nhật tiến độ.
3.  **Blameless Post-Mortem:** Mục tiêu là sửa lỗi, không phải tìm ra ai gây ra lỗi.

---

## 23.2. Phân loại mức độ nghiêm trọng (Severity Levels)

Trước khi hành động, hãy xác định mức độ (Severity/Priority) của sự cố để biết cách phản ứng phù hợp.

| Mức độ | Tên gọi | Định nghĩa | Ví dụ | Thời gian phản hồi mục tiêu |
| :--- | :--- | :--- | :--- | :--- |
| **P1** | **Critical** | Hệ thống hoàn toàn sập hoặc mất dữ liệu nghiêm trọng. | Không thể đăng nhập, Video call hoàn toàn mất. | < 15 phút |
| **P2** | **High** | Chức năng chính bị lỗi ảnh hưởng đến phần lớn người dùng. | Chat cực chậm, Video giật liên tục, Không gửi được tin nhắn. | < 30 phút |
| **P3** | **Medium** | Chức năng phụ hoặc một nhóm nhỏ người dùng bị ảnh hưởng. | AI gợi ý trả lời không hoạt động, Lỗi hiển thị avatar. | < 4 giờ |

---

## 23.3. Chuẩn bị trước khi trực (Pre-Shift Checklist)

Trước khi bắt đầu ca trực hàng đêm hoặc cuối tuần, đảm bảo bạn đã có đầy đủ quyền truy cập:

*   [ ] **Máy tính xách tay:** Đã sẵn sàng và pin đầy (phòng trường mất điện/nhà).
*   [ ] **Kubectl Access:** Đã cấu hình file `~/.kube/config` để truy cập Production Cluster.
*   [ ] **VPN/Token:** Đã có key VPN hoặc MFA Token để truy cập mạng nội bộ.
*   [ ] **Dashboards mở sẵn:** Grafana (Monitoring), Kibana (Logs), Sentry (App Errors).
*   [ ] **Liên lạc:** Đã tham gia kênh Slack/WhatsApp "On-Call" và lưu số điện thoại của CTO/Team Lead.

---

## 23.4. Quy trình xử lý Incident Tổng quát (General Incident Workflow)

Khi nhận được Alert (qua PagerDuty, Slack, hoặc Email):

1.  **Acknowledge (Xác nhận):** Trả lời ngay trên kênh báo động "I'm on it" hoặc bấm "Acknowledge" trên PagerDuty.
2.  **Triage (Đánh giá):**
    *   Đó là P1, P2 hay P3?
    *   Có bao nhiêu user bị ảnh hưởng? (Xem Grafana "Active Users" drop bao nhiêu).
    *   Có phải do mới deploy code không? (Kiểm tra GitHub Actions timeline).
3.  **Mitigation (Hạn chế/Tạm thời khắc phục):** Tìm cách nhanh nhất để user có thể dùng lại (thường là Scale up, Rollback, hoặc Restart).
4.  **Resolve (Giải quyết triệt để):** Sửa lỗi gốc (Root cause).
5.  **Verify (Kiểm tra):** Hệ thống ổn định trở lại 10 phút mới đóng case.

---

## 23.5. Kịch bản 1: Hệ thống không phản hồi / Lỗi 5xx (Service Down)

**Triệu chứng:** Người dùng báo "Không tải được trang", API trả về lỗi `502 Bad Gateway` hoặc `504 Gateway Timeout`.

### Quy trình xử lý

1.  **Kiểm tra Trạng thái Pods:**
    ```bash
    # Xem các pod có bị crash không?
    kubectl get pods -n secureconnect-prod | grep -E "(CrashLoopBackOff|Error|ImagePullBackOff)"
    ```
    *   **Nếu có nhiều pod CrashLoopBackOff:**
        1.  Xem logs của pod gần nhất:
            ```bash
            kubectl logs <pod-name> -n secureconnect-prod --tail=100
            ```
        2.  Nếu logs thấy `OutOfMemory (OOMKilled)`: Tăng RAM limit trong Deployment và restart lại (Bước 2).
        3.  Nếu logs thấy `Segmentation Fault` (Lỗi code): **ROLLBACK NGAY** (Xem Kịch bản 5).

2.  **Kiểm tra Load Balancer / Ingress:**
    ```bash
    # Kiểm tra xem Nginx Ingress có chạy không
    kubectl get pods -n ingress-nginx
    ```
    *   Nếu Ingress down: Kiểm tra CPU của node, có thể node đang quá tải khiến K8s không schedule được pod.

3.  **Kiểm tra Database Connections:**
    *   Đ Grafana: Chart `go_sql_connections_max_open`. Nếu đạt giới hạn (VD: 100/100) -> DB hết connection pool.
    *   **Hành động:**
        1.  Tăng số lượng Pod của Service (để chia sẻ connection).
        2.  Hoặc tăng `max_open_conns` trong code (cần deploy lại).
        3.  Restart lại Services (pod mới sẽ kết nối lại).

4.  **Tạm thời khắc phục:**
    Nếu không tìm ra nguyên nhân ngay lập tức:
    *   Scale up tất cả core services lên gấp đôi.
    ```bash
    kubectl scale deployment/api-gateway --replicas=6 -n secureconnect-prod
    kubectl scale deployment/chat-service --replicas=6 -n secureconnect-prod
    ```

---

## 23.6. Kịch bản 2: Video Call Bị Giật / Mất Kết nối (WebRTC Issues)

**Triệu chứng:** Người dùng phàn nàn video bị giật hình, mất tiếng, hoặc không kết nối được. Đây là P1 nếu ảnh hưởng đến nhiều cuộc họp.

### Quy trình xử lý

1.  **Kiểm tra Video Service Pods:**
    ```bash
    kubectl top pods -n secureconnect-prod | grep video-service
    ```
    *   Nếu **CPU > 90%**: SFU đang quá tải xử lý stream. -> Scale up Video Service.
    ```bash
    kubectl autoscaler hpa video-service-hpa -n secureconnect-prod
    # Hoặc scale thủ công:
    kubectl scale deployment/video-service --replicas=10
    ```

2.  **Kiểm tra TURN Server:**
    *   WebRTC cần TURN/STUN để xuyên qua Firewall.
    *   Nếu bạn dùng **Coturn** container, kiểm tra pod Coturn có đang chạy không.
    *   Nếu dùng dịch vụ bên ngoài (Twilio), kiểm tra dashboard nhà cung cấp xem có outage không.

3.  **Kiểm tra Network Policy:**
    *   Đôi khi K8s Network Policy chặn cổng UDP giữa các node.
    *   Ping thử từ một node khác: `nc -uz <pod-ip> 5004` (port UDP video).

4.  **Hành động khắc phục:**
    *   Nếu lỗi chung cho tất cả: **Restart lại Deployment Video Service**.
    ```bash
    kubectl rollout restart deployment/video-service -n secureconnect-prod
    ```

---

## 23.7. Kịch bản 3: Database (Cassandra) Chậm / Timeout

**Triệu chứng:** Chat load rất chậm, logs thấy `Cassandra timeout`, API trả về lỗi lưu tin nhắn.

### Quy trình xử lý

1.  **Kiểm tra Cluster Health:**
    *   Nếu dùng Helm Chart CockroachDB/Cassandra, chạy lệnh check health:
        ```bash
        kubectl exec -it cockroachdb-0 -n secureconnect-prod -- ./cockroach node status --insecure
        ```
    *   Nếu thấy node status là `decommissioning` hoặc `down`: Node DB đang chết.

2.  **Giải tỏa (Unthrottling) - *Chỉ dùng trong tình trạng cấp bách*:**
    *   Mặc định Cassandra dùng `LOCAL_QUORUM` (2/3 nodes).
    *   Tạm thời đổi Consistency Level trong code Go thành `LOCAL_ONE` để ghi nhanh hơn, chấp nhận rủi ro replication chậm (cần deploy nhanh code fix).

3.  **Thực hiện Cleanup Repair:**
    *   Nếu một node DB chết, dữ liệu sẽ không được replicate đủ 3 bản.
    *   Bật chế độ Repair (thường tự động, nhưng có thể kích hoạt thủ công nếu cần).

4.  **Giải pháp dài hạn (Nếu tình trạng ổn định):**
    *   Thêm node DB mới vào cluster.
    *   Chạy `nodetool repair` để khôi phục dữ liệu bị thiếu.

---

## 23.8. Kịch bản 4: Lỗi Bảo mật / Xâm nhập (Security Breach)

**Triệu chứng:** Hệ thống gửi báo cáo "Unusual traffic", log thấy nhiều failed login attempts, hoặc user báo "Account bị hack".

### Quy trình xử lý (P1)

1.  **Chặn (Block):**
    *   Chặn IP tấn công ở Level Nginx Ingress:
        ```bash
        # (Cần Ingress annotation allow-listing hoặc IP restriction)
        # Nếu không thể chặn IP nhanh qua config, có thể chặn tại firewall (AWS Security Group)
        ```

2.  **Kiểm tra Logs (ELK):**
    *   Gõ "Status: 401 AND URI: /v1/auth/login" + "Failure" vào Kibana.
    *   Xem có tấn công Brute Force (đoán password) không?

3.  **Hành động:**
    *   Nếu là tấn công Distributed (DDoS): Bật **Rate Limiting** cực khắt (1 req/min) trên API Gateway.
    *   Nếu là lộ mật khẩu Database: Đổi mật khẩu user DB (tại secrets K8s) và Restart Services (cần cẩn thận).
    *   Nếu lộ Private Keys (E2EE): Đây là thảm họa. **Phải thông báo cho tất cả user thay đổi mật khẩu và tạo lại cặp Key.**

4.  **Báo cáo:**
    *   Gửi email cho toàn bộ user: "Chúng tôi phát hiện hoạt động bất thường, vui lòng đổi mật khẩu".

---

## 23.9. Kịch bản 5: Sau khi Deploy Mới (Rollback)

**Triệu chứng:** Alert bay sầm ngay sau khi team bạn deploy version mới (VD: 5 phút sau deploy).

### Quy trình xử lý (Sai lầm thường gặp)

1.  **Dừng ngay lập tức:** Đừng cố gắng sửa lỗi trong lúc áp lực. Đưa hệ thống về trạng thái cũ là ưu tiên số 1.
2.  **Rollback Code:**
    ```bash
    # Quay lại version deployment trước đó
    kubectl rollout undo deployment/api-gateway -n secureconnect-prod
    kubectl rollout undo deployment/chat-service -n secureconnect-prod
    
    # Hoặc deploy lại image version cũ
    kubectl set image deployment/api-gateway \
      api-gateway=secureconnect/api-gateway:v1.0.0 \
      -n secureconnect-prod
    ```
3.  **Giám sát:** Chờ hệ thống ổn định khoảng 10 phút.
4.  **Điều tra:** Sau khi người dùng ổn định, mở ticket bug mới và xử lý ở nhánh `develop` thay vì `main`.

---

## 23.10. Giao tiếp & Thông báo (Communication)

Khi sự cố kéo dài quá 30 phút, bạn phải thông báo cho khách hàng/Stakeholder.

*   **Nội dung Status Page (Status Page):** Cập nhật trang status (ví dụ: status.secureconnect.com).
    *   *Màu Đỏ:* "Investigating connectivity issues with Video Calls."
    *   *Màu Vàng:* "Some users are experiencing slow message delivery. We are working on it."
    *   *Màu Xanh:* "Systems are operational."
*   **Slack Channel:** Cập nhật mỗi 15 phút một lần, kể cả khi "không có gì mới" (để mọi người biết bạn vẫn đang xử lý).
    *   *Mẫu tin:* `14:00 - Still investigating the root cause. CPU usage has dropped after scaling. Will update in 15m.`

---

## 23.11. Post-Incident Review (Sau sự cố)

Sau khi sự cố được giải quyết (trước khi ca trực kết thúc), bạn phải viết báo cáo **Post-Mortem (Báo cáo sự cố)**.

### Cấu trúc Báo cáo
1.  **Tóm tắt:** Đã xảy ra gì? (Đơn giản, khách quan).
2.  **Mức độ ảnh hưởng:** Bao nhiêu user bị ảnh hưởng? Bao lâu?
3.  **Thời gian dòng sự kiện (Timeline):** 10:00 (Alert) -> 10:05 (Ack) -> 10:15 (Mitigation) -> 11:00 (Resolved).
4.  **Nguyên nhân gốc (Root Cause):** Tại sao xảy ra? (Code bug, Hardware fail, Human error).
5.  **Hành động khắc phục (Immediate Actions):** Đã làm gì để sửa?
6.  **Bài học & Hành động dài hạn (Lessons Learned):**
    *   Thêm Monitor check cái gì?
    *   Cần sửa code như thế nào để không lặp lại?
    *   Cần đổi quy trình Deploy?

---

*Liên kết đến tài liệu tiếp theo:* `maintenance/decommissioning-policy.md` (Nếu cần viết chính sách ngừng vận hành version cũ, hoặc kết thúc tài liệu).