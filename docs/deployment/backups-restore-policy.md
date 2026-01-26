# Deployment Backups & Restore Policy

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 27.1. Tổng quan & Mục tiêu (RPO & RTO)

Dữ liệu là tài sản quý giá nhất. Mất dữ liệu (Data Loss) hoặc mất quá nhiều thời gian để khôi phục (Downtime) đều ảnh hưởng nghiêm trọng đến uy tín doanh nghiệp.

### Chỉ số mục tiêu (Recovery Objectives)
*   **RPO (Recovery Point Objective - Mức độ chấp nhận mất dữ liệu):**
    *   **Database (CockroachDB/Cassandra):** Tối đa **5 phút**. Nếu hệ thống sập, chúng ta chấp nhận mất tin nhắn gửi trong 5 phút gần nhất.
    *   **Redis (Cache):** Tối đa **15 phút**. Cache có thể xây dựng lại.
    *   **Object Storage (MinIO):** **0 phút** (Không mất file nhờ Replication).

*   **RTO (Recovery Time Objective - Thời gian khôi phục):**
    *   **Cấp bách (P1):** Khôi phục trong vòng **1 giờ**.
    *   **Cấp cao (P2):** Khôi phục trong vòng **4 giờ**.
    *   **Cấp trung (P3):** Khôi phục trong vòng **24 giờ**.

---

## 27.2. Các thành phần cần Sao lưu (Backup Targets)

Hệ thống cần sao lưu các dữ liệu sau:

| Thành phần | Dữ liệu sao lưu | Phương pháp ưu tiên |
| :--- | :--- | :--- |
| **CockroachDB** | User profiles, Contacts, Billing, Keys (Public) | Full Schedule Backup |
| **Cassandra** | Messages (Encrypted), Call Logs | Nodetool Snapshot (Per-node) |
| **Redis** | Session Keys, Online Status, Cache Pub/Sub | RDB Snapshot / AOF |
| **MinIO** | Files (Images, Videos), Backup của App | Object Versioning / Replication |
| **Git Repository** | Source Code (Go/Flutter), K8s Manifests | Git Backup (Gitea/GitHub) |
| **K8s Secrets** | Database Passwords, API Keys | HashiCorp Vault / Bitwarden (Backup Master Key) |

---

## 27.3. Chiến lược Lưu trữ Backup (Storage Strategy)

Đừng bao giờ lưu backup trên cùng máy chủ (Node) đang chạy dịch vụ. Nếu server bị cháy, bạn mất cả App lẫn Backup.

### 3.1. Cold Storage (Lưu trữ lạnh)
*   **Vị trí:** Sử dụng Object Storage riêng biệt hoặc dịch vụ Backup Cloud (AWS S3 Glacier, Backblaze B2).
*   **Chu kỳ:** Lưu trữ dữ liệu đã cũ (Older than 30 days) hoặc hàng ngày (Daily) bản sao.
*   **Chi phí:** Rẻ hơn lưu trữ trên SSD/NVMe server.

### 3.2. Off-site Backup (Ngoài hiện trường)
*   Copy các bản backup từ Primary Cluster (Ví dụ: Hetzner Germany) sang Secondary Region (Ví dụ: AWS Singapore).
*   Mục đích: Khôi phục khi thiên tai/hỏa hoạn xảy ra tại Datacenter chính.

---

## 27.4. Chi tiết Quy trình Backup từng Component

### 4.1. CockroachDB (SQL Database)

CockroachDB hỗ trợ sẵn lệnh backup rất mạnh `BACKUP TO`.

**Kịch bản:**
*   **Full Backup:** Chạy lúc 2:00 sáng hàng ngày.
*   **Incremental:** CockroachDB tự xử lý khi chạy lệnh backup liên tục (Incremental backup).

**Lệnh Backup (Crontab hoặc K8s CronJob):**
```bash
# Backup toàn bộ dữ liệu lên MinIO (S3 Compatible)
cockroach sql --insecure \
  "BACKUP TO 's3://backup-secureconnect@minio/cockroachdb?AWS_ACCESS_KEY_ID=minio&AWS_SECRET_ACCESS_KEY=minio123' \
   FROM DATABASE secureconnect_poc \
   WITH revision_history;"

# Kiểm tra tính toàn vẹn
cockroach sql --insecure \
  "SELECT * FROM system.namespace_validated_backup_details WHERE valid_start <= NOW() AND valid_end >= NOW();"
```

**Lưu ý:**
*   Sử dụng `revision_history` để có thể khôi phục (Point-in-Time Recovery - PITR) về đúng một thời điểm cụ thể (ví dụ: trước khi Developer chạy lệnh DROP TABLE).

### 4.2. Cassandra (NoSQL Database)

Cassandra sử dụng cơ chế dữ liệu phân tán, việc backup toàn cluster rất phức tạp.

**Chiến lược:**
*   **Nodetool Snapshot:** Dùng công cụ tích hợp `nodetool` để chụp nhanh dữ liệu trên mỗi Node. Rất nhanh, nhưng chỉ sao lưu data của node đó.
*   **Replication:** Chỉ cần backup 1 node trong mỗi Replication Set (RF=3) để giảm tải.

**Lệnh Backup (Chạy trên mỗi Pod Cassandra):**
```bash
# Snapshot data vào thư mục /backup (volume mount từ PVC)
nodetool snapshot /backup -t $(date +%Y-%m-%d)
```

**Tự động hóa (K8s CronJob):**
*   Sử dụng `k8ssandra` hoặc `cass-operator` có tính năng tạo job snapshot tự động vào S3.
*   Hoặc dùng `medusa` (Cassandra Manager chuyên nghiệp) để backup toàn cluster.

### 4.3. Redis (Cache)

Redis là In-memory, dữ liệu thay đổi liên tục.

**Chiến lược:**
*   **RDB Snapshot:** Lệnh `SAVE` hoặc `BGSAVE` để dump toàn bộ dữ liệu vào file `.rdb`.
*   **AOF (Append Only File):** Ghi từng lệnh ghi log vào file. Mất điện thì replay lại file này.

**Lệnh Backup (Manual hoặc CronJob):**
```bash
# Bắt đầu quá trình lưu vào file dump.rdb
redis-cli BGSAVE
```

### 4.4. MinIO / S3 (Object Storage)

MinIO hỗ trợ Versioning và Replication.

**Chiến lược:**
*   **Replication:** Cấu hình MinIO Cluster (ví dụ: 4 node, quorum 2). Nếu 1 node hỏng, dữ liệu vẫn còn.
*   **Versioning:** Bật **Object Versioning**. Nếu người dùng upload đè file `avatar.jpg` cũ, file cũ vẫn được giữ lại một bản (version `avatar.jpg.2`) -> Có thể khôi phục khi lỡ tay xóa.
*   **Gateway Tiering:** Mới file upload về SSD (Hot data), sau 30 ngày tự động đẩy sang HDD (Cold data).

---

## 27.5. Backup Configuration & Secrets (Quan trọng)

**QUY TẮC KHÔNG ĐƯỢC:** KHÔNG BAO GIỜ CHECK-in Database Passwords, API Keys (OpenAI, Stripe), hay Private Keys (E2EE Master Keys) vào Git Repository (nhất là Public Git).

**Giải pháp:**
1.  **K8s Secrets:** Lưu trong K8s Secret Object.
2.  **Encryption:** Sử dụng công cụ **SealedSecrets** (Bitnami) để mã hóa các Secret này. Chỉ file `sealed-secrets.json` được check vào Git (File này không thể đọc nếu không có Master Key).
3.  **Backup Master Key:** Master Key của SealedSecrets phải được lưu an toàn:
    *   Thay vì lưu trên máy, lưu trên **Password Manager** của Admin (1Password, Bitwarden) hoặc **HashiCorp Vault**.
    *   In ra giấy và cất vào két sắt của công ty.

---

## 27.6. Quy trình Khôi phục (Restore Procedures)

### 6.1. Sự cố: Database Corrupted (CockroachDB)

**Triệu chứng:** `CorruptedRange` error, Node không thể start.

**Quy trình khôi phục:**
1.  **Xác định thời điểm lỗi:** Tìm thời gian gần nhất hệ thống hoạt động bình thường.
2.  **Drop dữ liệu cũ (Cẩn thận):**
    ```bash
    cockroach sql --insecure "DROP DATABASE secureconnect_poc;"
    ```
3.  **Restore từ Backup:**
    ```bash
    cockroach sql --insecure \
      "RESTORE DATABASE secureconnect_poc FROM 's3://backup-secureconnect@minio/cockroachdb?...' \
       AS OF SYSTEM TIME '2023-11-01 02:00:00+00:00';"
    ```
4.  **Kiểm tra:** Query vài bảng user để xem dữ liệu có toàn vẹn không.

### 6.2. Sự cố: Cassandra Node chết

**Triệu chứng:** Một Pod Cassandra bị crash và không lên lại.

**Quy trình khôi phục:**
1.  **Sửa ổ cứng:** Thay volume dữ liệu.
2.  **Chạy lại Pod:** K8s sẽ tạo lại pod mới.
3.  **Xử lý dữ liệu cũ (Nếu node chết vĩnh viễn):**
    *   Lệnh `nodetool repair` chạy ngầm trên các node còn sống sẽ tự động điền data (repopulate) vào node mới này dựa trên các bản sao (replicas) còn lại.

### 6.3. Sự cố: Redis Flush hoặc Corrupted

**Triệu chứng:** Bạn lỡ tay chạy `FLUSHALL` hoặc AOF bị lỗi.

**Quy trình khôi phục:**
1.  **Dừng Redis:** `docker-compose stop redis`
2.  **Copy file backup rdb:** Copy file `.rdb` mới nhất từ Backup Storage vào `/var/lib/redis`.
3.  **Chạy lại Redis:** `docker-compose up redis`
    *   Redis sẽ tự động load file `.rdb`.
    *   Hệ thống sẽ "khởi động lại" từ trạng thái gần nhất (Session keys sẽ bị hết hạn nếu lâu quá, nhưng user cần login lại - acceptable).

### 6.4. Sự cố: Xóa nhầm file trên MinIO

**Triệu chứng:** User báo mất video quan trọng.

**Quy trình khôi phục:**
1.  Kiểm tra Object Versioning của file đó.
2.  Dùng `mc` (MinIO Client) hoặc Web Console để `rollback` về phiên bản cũ.
3.  Hoặc nếu xóa vĩnh viễn (Expire/Delete):
    *   Từ Backup Storage (MinIO/S3 Glacier) tải lại file.
    *   Upload lại vào bucket Active của MinIO.
    *   Cập nhật lại URL trong Database.

---

## 27.7. Chính sách lưu giữ dữ liệu (Retention Policy)

Để tiết kiệm chi phí lưu trữ (Tiền thuê VPS/Cloud):

| Dữ liệu | Thời gian giữ (Retention) | Hành động khi hết hạn |
| :--- | :--- | :--- |
| **Database Backup** | 90 Ngày | Tự động xóa bản backup cũ (>90 days). |
| **User Data (Messages)** | Vĩnh viễn (Chỉ khi User xóa) | Tuân thủ quy định "Right to be Forgotten" của GDPR. |
| **Video/Audio Files** | 365 Ngày (1 năm) | Nếu User không mở lại file sau 1 năm, xóa file và Backup để tiết kiệm. |
| **Logs (Nginx/App Logs)** | 30 Ngày | Xóa logs cũ để giải phóng dung lượng đĩa. |
| **Audit Logs** | 5 Năm | Để phục vụ điều tra pháp lý. |

---

## 27.8. Kiểm thử Backup (Testing Backups)

Một bản backup không bao giờ được kiểm thử thì là **null backup** (Backup vô nghĩa).

**Quy trình kiểm thử hàng tháng:**
1.  **Validation:** Chạy lệnh Validate của CockroachDB/Cassandra để xem bản backup có lỗi gì không.
2.  **Dry-Run Restore:** Lấy bản backup mới nhất, Restore lên một Database Test Staging riêng biệt.
3.  **Verify:** Query thử vài bảng dữ liệu, so sánh số lượng rows.
4.  **Automate:** Viết script (Go/Bash) chạy hàng đêm thực hiện bước này và gửi báo cáo (Success/Fail) qua Slack/Email cho DevOps.

---

## 27.9. Disaster Recovery (DR) Plan (Kịch bản xấu nhất)

Nếu toàn bộ Datacenter (Ví dụ: Hetzner Germany) bị cháy hoặc mất mạng dài ngày:

1.  **Switch DNS:** Thay đổi DNS (`api.secureconnect.com`) trỏ về IP của Secondary Cluster (AWS Singapore).
2.  **Failover App:** Deploy lại các Services Docker lên Cluster mới (đã có sẵn Docker Images).
3.  **Restore Database:** Restore dữ liệu CockroachDB/MinIO từ Off-site Backup (S3) vào Cluster mới.
4.  **Tổng thời gian (RTO):** Mục tiêu là dịch vụ hoạt động lại trong vòng **2-4 giờ**.

---

## 27.10. Công cụ hỗ trợ (Tools)

*   **Velegrup:** Để quản lý và chạy lệnh sao lưu (Backup jobs) trên toàn bộ servers.
*   **Restic:** Một công cụ dòng lệnh (CLI) cực mạnh để sao lưu file và thư mục lên nhiều storage (S3, SFTP, MinIO) với tính năng Deduplication và Encryption (GPG).
*   **K8s Velero:** Để sao lưu toàn bộ Resources (Pods, Persistent Volumes) của Kubernetes. Nếu bạn xóa nhầm namespace `secureconnect-prod`, Velero giúp restore lại tất cả.

---