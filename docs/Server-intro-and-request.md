## 1. Hệ điều hành & Môi trường (OS & Environment)

*   **Hệ điều hành (OS):** **Ubuntu 22.04 LTS** hoặc **Debian 12**. Đây là tiêu chuẩn vàng cho doanh nghiệp (Tất cả tài liệu hướng dẫn sẽ dựa trên dòng lệnh Linux).
*   **Môi trường giả lập (Simulation):**
    *   Có thể chạy trên các nền tảng ảo (Hypervisor): VMware, VirtualBox, Hyper-V.
    *   Hoặc thuê **VPS Cloud** (Đây thực chất cũng là máy ảo trên Datacenter): DigitalOcean, Vultr, Linode, AWS EC2, Azure VM.
*   **Runtime:** Docker & Docker Compose (cần cài đặt trên tất cả các Server).

---

## 2. Chiến lược Phân chia Server (Server Segmentation)

Để đảm bảo hiệu năng và an toàn, hệ thống được chia thành **2 Tầng chính**: Tầng Ứng dụng (App) và Tầng Dữ liệu (DB). Ngoài ra cần thêm Server Load Balancer.

Tối thiểu cho hệ thống này hoạt động (tối thiểu), bạn cần **5 Server riêng biệt**.

---

## 3. Chi tiết từng Server (The 5 Servers)

Dưới đây là mô hình **Optimized Production**.

### SERVER 1: Load Balancer & Reverse Proxy (Cổng vào)
*   **Mã định danh:** `lb-01`
*   **Nhiệm vụ:**
    *   Nhận toàn bộ request từ người dùng (HTTP/HTTPS, WSS).
    *   Phân chia tải đến các App Servers (Server 2 & 3).
    *   Xử lý SSL/TLS (HTTPS termination).
    *   Bật tắt bảo mật (WAF).
*   **Cấu hình tối thiểu:**
    *   **CPU:** 2 vCPU (Xử lý mạng IO cần tốc độ xung nhột cao).
    *   **RAM:** 2 GB (Nginx rất nhẹ).
    *   **Disk:** 40 GB SSD (Chỉ để lưu Logs, không quan trọng).
    *   **Mạng:** Băng thông lớn (Gigabit hoặc 1Gbps lên).
    *   **OS:** Ubuntu Server.

### SERVER 2 & 3: Application Servers (Backend Services)
*   **Mã định danh:** `app-01`, `app-02`
*   **Nhiệm vụ:** Chạy các container Go (API Gateway, Auth Service, Chat Service, Video Service).
    *   Mỗi server sẽ chạy một loạt Docker Containers của các dịch vụ này.
*   **Cấu hình tối thiểu:**
    *   **CPU:** 8 vCPU (Quan trọng cho Video/WebRTC và Mã hóa E2EE).
    *   **RAM:** 16 GB (Go ăn RAM rất ít, nhưng cần đủ để cache dữ liệu và xử lý đồng thời hàng ngàn kết nối).
    *   **Disk:** 100 GB SSD (Lưu logs, images tạm, swap).
    *   **OS:** Ubuntu Server.

### SERVER 4: Database Cluster (SQL & NoSQL)
*   **Mã định danh:** `db-01`
*   **Nhiệm vụ:**
    *   Chạy **CockroachDB** (SQL) - Cho Users, Contacts, Billing.
    *   Chạy **Cassandra** (NoSQL) - Cho Messages, Call Logs.
    *   Chạy **Redis** - Cho Cache, Sessions, Pub/Sub.
*   **Cấu hình tối thiểu:**
    *   **CPU:** 16 vCPU (Cassandra và CockroachDB rất "nghiện" CPU để xử lý query).
    *   **RAM:** 64 GB (Đây là phần quan trọng nhất. Database cần RAM để cache dữ liệu trên đĩa).
    *   **Disk:** 2 TB NVMe SSD (IOPS cực nhanh là sống còn của Database).
    *   **OS:** Ubuntu Server.

### SERVER 5: Storage & Media Infrastructure
*   **Mã định danh:** `storage-01`
*   **Nhiệm vụ:**
    *   Chạy **MinIO** (S3 Compatible) - Ổ đĩa cá nhân (Lưu ảnh/video).
    *   Chạy **TURN Server (Coturn)** - Quan trọng nhất cho Video Call (Xuyên NAT).
*   **Cấu hình tối thiểu:**
    *   **CPU:** 4 vCPU.
    *   **RAM:** 8 GB.
    *   **Disk:** 10 TB HDD (hoặc 2TB SSD nếu giá cả cho phép) - Để lưu file.
    *   **Mạng:** Băng thông rất lớn + **IP Công khai (Public IP)** là bắt buộc cho TURN Server.

---

## 4. Bảng Tóm tắt Cấu hình (Simulation Specs)

| # | Tên Server | Số lượng | CPU | RAM | Ổ cứng (Disk) | Mục đích chính |
|:---:|:---:|:---:|:---:|:---:|:---:|
| **1** | `lb-01` (Nginx) | 1 | 2 Cores | 2 GB | 40 GB SSD | Load Balancing, Proxy HTTP/WSS. |
| **2** | `app-01` (Go) | 1 | 8 Cores | 16 GB | 100 GB SSD | Chạy Container App (Chat, Auth, API). |
| **3** | `app-02` (Go) | 1 | 8 Cores | 16 GB | 100 GB SSD | Chạy Container App (Dự phòng/Scale). |
| **4** | `db-01` (DB) | 1 | 16 Cores | 64 GB | 2 TB NVMe | Chạy CockroachDB, Cassandra, Redis. |
| **5** | `storage-01` | 1 | 4 Cores | 8 GB | 10 TB HDD/SSD | MinIO, TURN Server. |

**Tổng cộng:** 5 Server.

---

## 5. Giả lập trên Máy ảo (VM) - Cách thực hiện

Nếu bạn muốn thử nghiệm trên máy cá nhân (Laptop/PC mạnh) bằng **VMware** hoặc **VirtualBox**:

**Cách 1: All-in-One (Dễ nhất)**
*   Tạo 1 VM duy nhất.
*   Cấu hình: 16 Cores (tùy máy chủ), 32GB RAM.
*   Chạy `docker-compose` lên trên đó. Tất cả các service (LB, App, DB, Storage) chạy trong cùng 1 VM này.
*   **Hạn chế:** Chỉ test được tính năng logic, không thể test hiệu năng mạng hay phân tán dữ liệu.

**Cách 2: Cluster trên Localhost (Gần thực tế nhất)**
*   Nếu bạn có PC mạnh (ví dụ: 64GB RAM, Threadripper CPU), có thể dùng **Multipass** hoặc **KVM** để tạo 5 VM ảo chạy trên PC của bạn.
*   Mỗi VM cấu hình như bảng ở mục 4.
*   Kết nối chúng vào một Virtual Switch ( mạng nội bộ của VM).
*   Kết quả: Bạn có một hệ thống Datacenter thu nhỏ ngay trên bàn làm việc.

**Cách 3: Thuê VPS (Chi phí thấp)**
*   Thuê 5 VPS từ nhà cung cấp giá rẻ (Hetzner, OVH, Vultr, DigitalOcean).
*   Tổng chi phí ước tính: $100 - $200/tháng tùy vào nhà cung cấp.
*   Bạn có quyền truy cập Root vào từng server như Remote Desktop (SSH).

---

## 6. Các cổng mạng cần mở (Networking & Ports)

Để hệ thống hoạt động, bạn cần cấu hình Firewall (Cloud Firewal hoặc iptables trên Server) cho phép các port sau:

| Port | Giao thức | Server | Mô tả |
|:---:|:---:|:---:|:---|
| **80**, **443** | TCP | `lb-01` | Public HTTP/HTTPS vào hệ thống. |
| **8080** | TCP | `app-01`, `app-02` | Nginx (LB) forward về đây. |
| **9000** | TCP | `storage-01` | Console quản lý MinIO. |
| **26257** | TCP | `db-01` | CockroachDB UI quản lý (nếu cần debug). |
| **9042** | TCP | `db-01` | Cassandra Client Connection. |
| **6379** | TCP | `db-01` | Redis Connection. |
| **3478** | TCP + UDP | `storage-01` | **QUAN TRỌNG:** Cổng TURN Server (Video Call). Phải mở UDP. |

---

## 7. Quy trình Triển khai trên Server (Deployment Guide)

Giả sử bạn đã có 5 Server (hoặc 5 VPS) và có IP của chúng (ví dụ: `192.168.1.10` -> `.14`).

### Bước 1: Chuẩn bị trên tất cả Server
*   Cài đặt **Docker** và **Docker Compose**.
*   Cài đặt **Git**.
*   Tạo folder `/opt/secureconnect` trên mọi server.

### Bước 2: Setup Database (Server 4 - `db-01`)
*   Clone code git về server này.
*   Chạy `docker-compose up -d cockroachdb cassandra redis` (Sử dụng file `docker-compose.yml` đã viết).
*   Kiểm tra logs xem DB có lên không.

### Bước 3: Setup Storage & TURN (Server 5 - `storage-01`)
*   Chạy `docker-compose up -d minio coturn`.
*   **Quan trọng:** Đảm bảo `coturn` (TURN) được cấu hình đúng IP công khai của `storage-01` (`external-ip`).

### Bước 4: Setup Backend Apps (Server 2 & 3 - `app-01`, `app-02`)
*   Clone code git.
*   Tạo file `.env` để chỉ ra IP của Server Database và Storage.
    ```env
    DB_HOST=192.168.1.13 # IP của db-01
    REDIS_HOST=192.168.1.13
    MINIO_ENDPOINT=192.168.1.14
    ```
*   Chạy `docker-compose up -d`.

### Bước 5: Setup Load Balancer (Server 1 - `lb-01`)
*   Cài đặt **Nginx** (nếu không dùng Dockerfile NGINX).
*   Tạo cấu hình `nginx.conf` để `proxy_pass` truy vấn từ port 80/443 sang `app-01:8080` và `app-02:8080`.
*   Restart Nginx.

### Bước 6: Test từ Localhost (Máy bạn)
*   Trên máy của bạn, trỏ file `hosts` (Windows: `C:\Windows\System32\drivers\etc\hosts`, Mac/Linux: `/etc/hosts`):
    ```text
    192.168.1.10  api.secureconnect.com
    192.168.1.10  chat.secureconnect.com
    ```
*   Mở trình duyệt web hoặc App Flutter, trỏ URL `http://api.secureconnect.com/v1/auth/login`.
*   Nếu đăng nhập thành công, chúc mừng bạn đã dựng thành công hệ thống phân tán hoàn chỉnh!

---

Bạn có muốn tôi giúp bạn viết file **`deployment/ansible-playbook.yml`** 