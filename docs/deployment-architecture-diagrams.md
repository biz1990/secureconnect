# Deployment Architecture Diagrams

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 25.1. Tổng quan Chiến lược Triển khai (Deployment Strategy)

Hệ thống SecureConnect sử dụng kiến trúc **3-Tier Deployment** (Triển khai 3 lớp):

1.  **Public/DMZ Tier (Lớp công khai):** Load Balancer, Firewall, Nginx Ingress.
2.  **Application Tier (Lớp ứng dụng):** Các Microservices (Auth, Chat, Video) chạy trên Docker/Kubernetes.
3.  **Data Tier (Lớp dữ liệu):** Databases phân tán (Cassandra, CockroachDB) và Cache (Redis).

---

## 25.2. Sơ đồ Chi tách Server Logic (Logical Server Architecture)

Sơ đồ này mô tả cách chúng ta chia nhỏ hạ tầng vật lý hoặc VPS thành các nhóm chức năng riêng biệt để đảm bảo tính bảo mật và hiệu năng.

```mermaid
graph TD
    subgraph "Tier 1: Public / DMZ"
        LB[Load Balancer / Cloudflare]
        NGINX[Nginx Ingress Controller]
    end

    subgraph "Tier 2: Application Services (Go Microservices)"
        GW[API Gateway]
        AUTH[Auth Service]
        CHAT[Chat Service]
        VIDEO[Video Service / SFU]
        AI[AI Service Wrapper]
    end

    subgraph "Tier 3: Data Storage & State"
        REDIS[Redis Cluster]
        CRDB[CockroachDB Cluster]
        CASS[Cassandra Cluster]
        MINIO[MinIO Object Storage]
    end

    subgraph "Infrastructure & Monitoring"
        PROM[Prometheus Metrics]
        GRAF[Grafana Dashboard]
        ELK[ELK Stack Log]
    end

    %% Traffic Flow
    User[End Users] -->|HTTPS/WSS| LB
    LB --> NGINX
    NGINX --> GW
    
    GW --> AUTH
    GW --> CHAT
    GW --> VIDEO
    
    AUTH --> CRDB
    AUTH --> REDIS
    
    CHAT --> CASS
    CHAT --> REDIS
    
    VIDEO --> REDIS
    VIDEO -.->|P2P Media Stream| User
    
    STOR[Storage Service] -.-> MINIO
```

### Giải thích các nhóm Server:
1.  **Public Servers:** Chạy Nginx Ingress, chỉ mở port 80 (HTTP) và 443 (HTTPS).
2.  **App Servers:** Chạy các Docker container Go. Nằm trong mạng nội bộ (Private Network), chỉ có thể truy cập qua Ingress.
3.  **DB Servers:** Máy chủ chuyên dụng chạy Database, ưu tiên cấu hình IO cao (SSD/NVMe), chỉ cho phép App Servers kết nối thông qua Private IP.

---

## 25.3. Sơ đồ Kiến trúc Docker (Docker Deployment Architecture)

Sơ đồ này mô tả cách các Container Docker giao tiếp với nhau thông qua Docker Networks (Network Isolation).

```mermaid
graph LR
    subgraph "External Network (Internet)"
        User[User Browser / Flutter App]
    end

    subgraph "Docker Host: Public Zone"
        NGINX[Nginx Container]
    end
    
    subgraph "Docker Host: Application Zone"
        subgraph "Network: secureconnect-backend"
            GW[Gateway Container<br/>:8080]
            AUTH[Auth Container<br/>:8080]
            CHAT[Chat Container<br/>:8080]
            VIDEO[Video Container<br/>:8080]
        end
    end

    subgraph "Docker Host: Data Zone"
        subgraph "Network: secureconnect-db"
            R1[Redis Container 1<br/>:6379]
            R2[Redis Container 2<br/>:6379]
            CRDB[CockroachDB Container<br/>:26257, 8080]
            CASS[Cassandra Container 1<br/>:9042]
        end
    end

    %% Volumes (Storage)
    V_CRDB[(Persisted Volume<br/>CRDB Data)]
    V_CASS[(Persisted Volume<br/>Cassandra Data)]

    %% Connections
    User -- "Port 80/443" --> NGINX
    NGINX -- "Reverse Proxy" --> GW
    
    GW -.->|REST / WS| AUTH
    GW -.->|REST / WS| CHAT
    GW -.->|REST / WS| VIDEO
    
    AUTH -- "TCP" --> CRDB
    AUTH -- "TCP" --> R1
    CHAT -- "TCP" --> CASS
    VIDEO -- "TCP" --> R2
    
    CRDB --> V_CRDB
    CASS --> V_CASS
```

### Docker Networks Isolation (Chi tiết cấu hình)
*   **secureconnect-frontend:** Network cho UI (không dùng trong PoC backend, nhưng có thể dùng cho Web Nginx).
*   **secureconnect-backend:** Chứa toàn bộ services Go. Các service này có thể giao tiếp với nhau qua tên container (ví dụ: `gw` gọi `auth`).
*   **secureconnect-db:** Chứa Database. Chỉ các container ở `backend` mới có thể kết nối vào network này.

---

## 25.4. Sơ đồ Kiến trúc Kubernetes (K8s Deployment Architecture)

Sơ đồ này phức tạp hơn, mô tả chi tiết cách các Pods, Services, và Ingress hoạt động trong một Cluster Kubernetes (ví dụ trên Google Cloud GKE hoặc AWS EKS).

```mermaid
graph TD
    subgraph "Cluster: secureconnect-prod"
        subgraph "Namespace: ingress-nginx"
            ING[Nginx Ingress Controller<br/>Pod]
        end

        subgraph "Namespace: secureconnect-prod"
            %% INGRESS LAYER
            ISVC[Ingress Service<br/>Type: LoadBalancer]
            
            %% SERVICES & DEPLOYMENTS
            subgraph "API Gateway"
                DP_GW[Deployment: api-gateway]
                P_GW[Pod: api-gateway-pod-1]
                P_GW_2[Pod: api-gateway-pod-2]
                SVC_GW[Service: api-gateway<br/>ClusterIP]
            end

            subgraph "Auth Service"
                DP_AUTH[Deployment: auth-service]
                P_AUTH_1[Pod: auth-service-x]
                P_AUTH_2[Pod: auth-service-y]
                SVC_AUTH[Service: auth-service<br/>ClusterIP]
            end

            subgraph "Chat Service"
                DP_CHAT[Deployment: chat-service]
                P_CHAT[Pod: chat-service-replica-1..N]
                SVC_CHAT[Service: chat-service<br/>ClusterIP]
            end

            subgraph "Video Service"
                DP_VID[Deployment: video-service]
                P_VID[Pod: video-service-1..N]
                SVC_VID[Service: video-service<br/>ClusterIP]
            end
        end

        subgraph "Namespace: secureconnect-db"
            %% STATEFUL SETS (Databases)
            ST_CRDB[StatefulSet: cockroachdb<br/>Pod: cockroachdb-0,1,2]
            ST_CASS[StatefulSet: cassandra<br/>Pod: cassandra-0..N]
            ST_REDIS[StatefulSet: redis<br/>Pod: redis-master]
            
            %% HEADLESS SERVICES (For internal comm)
            H_SVC_CRDB[Headless Service: cockroachdb]
            H_SVC_CASS[Headless Service: cassandra]
            H_SVC_REDIS[Service: redis<br/>ClusterIP]
        end
    end

    subgraph "Cloud Provider (AWS / GCP)"
        EXT_LB[External Load Balancer<br/>AWS ELB / GCP LB]
        PVC[Persistent Volume Claims<br/>AWS EBS / GCP PD]
    end

    %% TRAFFIC & CONNECTIONS
    User[External User] -->|HTTPS| EXT_LB
    EXT_LB -->|NodePort| ISVC
    ISVC --> ING
    ING -->|Routing| SVC_GW
    ING -->|Routing| SVC_CHAT
    ING -->|Routing| SVC_VID
    
    %% INTERNAL CONNECTIONS (Inter-Service Comm)
    SVC_GW -.->|HTTP/RPC| SVC_AUTH
    SVC_CHAT -.->|gRPC/Protobuf| ST_CASS
    SVC_AUTH -.->|PostgreSQL| ST_CRDB
    SVC_CHAT -.->|TCP| ST_REDIS
    
    %% STORAGE MAPPING
    ST_CRDB -.-> PVC
    ST_CASS -.-> PVC
```

### Giải thích các thành phần K8s:

1.  **Ingress Controller (`ingress-nginx`):**
    *   Đây là "Cửa ngõ" duy nhất của Cluster.
    *   Nó nhận HTTPs từ Internet và route đến các `Service` bên trong Cluster dựa trên `IngressRules` (Host: `api.secureconnect.com` -> Service: `api-gateway`).

2.  **Deployments vs Services:**
    *   **Deployment:** Quản lý các Pods (ví dụ: đảm bảo luôn có 2 replicas của `auth-service` đang chạy). Nếu Pod bị chết, Deployment sẽ tạo lại Pod mới.
    *   **Service:** Cung cấp một địa chỉ IP ổn định (ClusterIP) hoặc DNS cho các Pods. Ví dụ: `api-gateway` Service sẽ truy cập đến bất kỳ pod nào đang chạy của Deployment đó.

3.  **Namespace Separation:**
    *   `secureconnect-prod`: Chứa các services ứng dụng.
    *   `secureconnect-db`: Chứa các databases (Cassandra, CRDB). Tách biệt namespace giúp quản lý quyền truy cập (Quota tài nguyên) tốt hơn.

4.  **Persistent Volumes (PVC):**
    *   Dữ liệu Database (CRDB, Cassandra) được lưu trong PVC (được map tới các Block Storage trên Cloud Provider).
    *   Khi Pod Database bị khởi động lại (update version), dữ liệu vẫn còn ở PVC, không bị mất.

---

## 25.5. Luồng mạng chi tiết (Network Traffic Flow)

Để đảm bảo an toàn và hiệu quả, luồng dữ liệu đi qua các lớp như sau:

### Luồng 1: HTTP API (Chat, Auth)
```mermaid
sequenceDiagram
    participant User as U
    participant LoadBalancer as LB
    participant Ingress as NG
    participant Gateway as GW
    participant AuthSvc as AS
    participant DB as CRDB

    U->>LB: GET https://api.secureconnect.com/v1/auth/login
    LB->>NG: TCP 443
    NG->>GW: TCP 80 (Internal)
    GW->>AS: TCP 8080 (gRPC / HTTP)
    AS->>CRDB: TCP 26257 (SQL Query)
    CRDB-->>AS: Result Rows
    AS-->>GW: JSON User Data
    GW-->>NG: JSON Response
    NG-->>LB: JSON Response
    LB-->>U: JSON Response
```

### Luồng 2: Real-time WebSocket (Chat)
```mermaid
sequenceDiagram
    participant User as U
    participant Ingress as NG
    participant ChatSvc as CS
    participant Redis as R

    U->>NG: wss://api.secureconnect.com/v1/ws/chat
    NG->>CS: Forward WebSocket Upgrade
    CS->>R: Subscribe channel (user_id)
    CS-->>U: Connected
    
    Note over U, CS: Trạng thái Online/Offline lưu trong Redis
    U->>CS: Send Message (JSON)
    CS->>R: Publish Message
    CS->>R: Set typing_status = true
    CS->>U: Broadcast Message (Qua Redis Pub/Sub)
```

### Luồng 3: WebRTC Media (Video Call)
*   **Lưu ý:** Luồng Media (Video/Audio) sử dụng giao thức **SRTP** và chạy ngang (P2P hoặc qua SFU), **KHÔNG đi qua Ingress/HTTP**.

```mermaid
sequenceDiagram
    participant ClientA as A
    participant ClientB as B
    participant Turn as TURN
    participant Sfu as SFU

    A->>TURN: UDP Allocate Port
    TURN-->>A: Candidate:TURN

    A->>B: Signaling (Offer via HTTP/WebSocket Server)
    B->>A: Signaling (Answer via HTTP/WebSocket Server)
    
    A->>SFU: Establish Connection
    B->>SFU: Establish Connection
    
    A->>SFU: RTP Packets (Video/Audio)
    SFU->>B: Forward RTP Packets
    
    Note over A, B: Media Stream không đi qua HTTP Server
```

---

## 25.6. Quy mô Mở rộng (Scaling Diagram)

Khi lượng người dùng tăng, hệ thống mở rộng (Scale-out) theo chiều ngang (Horizontal Scaling) như sau:

```mermaid
graph LR
    subgraph "Phase 1: Initial Deployment"
        G1[Gateway: 1 Replica]
        A1[Auth: 2 Replicas]
        C1[Chat: 3 Replicas]
    end

    subgraph "Phase 2: Scale Up (Traffic x 2)"
        G2[Gateway: 2 Replicas]
        A2[Auth: 4 Replicas]
        C2[Chat: 6 Replicas]
    end

    subgraph "Phase 3: High Load (Scale Video)"
        G3[Gateway: 3 Replicas]
        A3[Auth: 6 Replicas]
        C3[Chat: 8 Replicas]
        V3[Video Service: 5 Replicas<br/>(CPU Heavy)]
    end

    %% Kubernetes HPA (Horizontal Pod Autoscaler) tự động thêm Pods dựa trên CPU/Metric
    subgraph "Kubernetes Cluster"
        K8S[Master Node<br/>Quản lý Schedule]
        W1[Worker Node 1]
        W2[Worker Node 2]
        W3[Worker Node 3]
    end

    %% Pods được phân bổ (Schedule) tự động qua các Node
    G2 --> W1
    A2 --> W1
    C2 --> W2
    V3 --> W3
```

---

## 25.7. Tóm tắt & Lưu ý triển khai

1.  **Tách biệt Network:** Luôn tách biệt mạng dữ liệu (Database) khỏi mạng ứng dụng (Backend). Không bao giờ để DB có IP công khai.
2.  **Ingress Controller:** Đừng để các Pods (Services) dùng `NodePort` để truy cập trực tiếp từ Internet. Luôn dùng `ClusterIP` kết hợp với `Ingress Controller` (Nginx) để bảo mật và quản lý SSL/SSL dễ dàng.
3.  **Persistent Storage:** Dữ liệu Database phải được gắn vào `PersistentVolumeClaim` (PVC). Đừng lưu DB vào container (OverlayFS) vì khi Pod bị chết, dữ liệu sẽ mất.
4.  **Resource Limits:** Luôn đặt `requests` và `limits` cho RAM/CPU cho Pods. Nếu không, một pod lỗi (Memory Leak) có thể ăn hết tài nguyên của cả Node, làm sập toàn bộ hệ thống.