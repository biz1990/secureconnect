# Kubernetes Deployment Manifests Guide

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 19.1. Tổng quan

Kubernetes là hệ điều hành cụm (Container Orchestration) cho hệ thống của chúng tôi. Nó chịu trách nhiệm quản lý vòng đời của các container Go Backend, tự động mở rộng (Scaling), và tự phục hồi (Self-healing) nếu container bị sập.

### Mục tiêu triển khai
*   **High Availability:** Chạy ít nhất 2 replicas cho mỗi service để không có downtime (POD - Point of Deployment).
*   **Horizontal Pod Autoscaler (HPA):** Tự động tăng số lượng Pod khi CPU/RAM vượt ngưỡng.
*   **Secret Management:** Tách biệt thông tin nhạy cảm (Password, API Key) khỏi source code.
*   **Zero Downtime Deployment:** Hỗ trợ Rolling Update khi cập nhật phiên bản mới.

---

## 19.2. Cấu trúc Thư mục K8s

```bash
k8s/
├── overlays/
│   ├── dev/                 # Environment configs cho Dev
│   ├── staging/              # Configs cho Staging
│   └── prod/                # Configs cho Production
├── base/
│   ├── namespace.yaml        # Namespace separation
│   ├── configmap.yaml        # Cấu hình không nhạy cảm
│   ├── secrets.yaml          # Template cho Secrets (không commit thật vào git)
│   ├── gateway-deployment.yaml
│   ├── chat-deployment.yaml
│   ├── video-deployment.yaml
│   └── ingress.yaml         # Nginx Ingress Rules
└── helm/
    ├── cockroachdb/          # Helm chart cho DB (không tự viết Yaml dài dòng)
    ├── cassandra/
    └── redis/
```

---

## 19.3. Namespace Isolation

Tách biệt môi trường để tránh xung đột resource.

**File: `k8s/base/namespace.yaml`**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: secureconnect-prod
  labels:
    name: production
    project: secureconnect
```

---

## 19.4. Secret Management (Quản lý bí mật)

**QUAN TRỌNG:** Không bao giờ đẩy file secrets thật chứa mật khẩu lên Git. Dùng Kustomize hoặc tạo thủ công.

### Cách tạo Secret (CLI)
```bash
# Tạo secret generic
kubectl create secret generic app-secrets \
  --from-literal=db-password='super-secure-pass' \
  --from-literal=jwt-secret='jwt-signing-key' \
  -n secureconnect-prod
```

### File Template (dùng cho reference/kustomize)
**File: `k8s/base/secrets.yaml`**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
type: Opaque
# stringData sẽ được hash thành data khi apply
stringData:
  # Đừng để mật khẩu thật ở đây
  db-password: "PLACEHOLDER"
  jwt-secret: "PLACEHOLDER"
  minio-access-key: "PLACEHOLDER"
  ai-openai-key: "PLACEHOLDER"
```

---

## 19.5. ConfigMaps (Cấu hình ứng dụng)

Dùng cho các cấu hình không nhạy cảm: URLs, Ports, Service Names.

**File: `k8s/base/configmap.yaml`**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  ENV: "production"
  
  # Database Configs
  DB_HOST: "cockroachdb-public"
  REDIS_HOST: "redis-master"
  
  # Cassandra Config
  CASSANDRA_HOSTS: "cassandra-0,cassandra-1,cassandra-2"
  
  # Object Storage
  MINIO_ENDPOINT: "http://minio-hl-svc:9000"
  
  # Logging
  LOG_LEVEL: "info"
```

---

## 19.6. Deployment cho Core Services (Backend)

Ví dụ triển khai cho **Chat Service**. Các service khác (Auth, Video) tuân theo mẫu tương tự, chỉ thay đổi `image`, `port`, và `resources`.

**File: `k8s/base/chat-deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: chat-service
  namespace: secureconnect-prod
spec:
  replicas: 3                 # Chạy 3 pods để High Availability
  strategy:
    type: RollingUpdate        # Zero downtime update
    rollingUpdate:
      maxSurge: 1              # Tối đa thêm 1 pod khi update
      maxUnavailable: 1       # Tối đa 1 pod bị down trong khi update

  selector:
    matchLabels:
      app: chat-service

  template:
    metadata:
      labels:
        app: chat-service
        version: v1
    spec:
      # Anti-affinity: Tách các pod ra các Node khác nhau (Nếu cluster có nhiều node)
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                  - key: app
                    operator: In
                    values:
                    - chat-service
                topologyKey: "kubernetes.io/hostname"

      containers:
      - name: chat-service
        image: secureconnect/chat-service:v1.0.0  # Tag từ Docker build
        imagePullPolicy: IfNotPresent
        
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        
        env:
        # Tải Config từ ConfigMap
        - name: ENV
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: ENV
        - name: REDIS_HOST
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: REDIS_HOST
        
        # Tải Secret từ Secret Object
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: db-password
              
        # Resource Limits (Quan trọng để tránh một pod ăn hết tài nguyên cluster)
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        
        # Health Checks (Bắt buộc)
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 1
---
# Service để load balance traffic giữa các Pods
apiVersion: v1
kind: Service
metadata:
  name: chat-service
  namespace: secureconnect-prod
spec:
  selector:
    app: chat-service
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
  type: ClusterIP # Chỉ truy cập nội bộ cluster
```

---

## 19.7. Deployment cho Video Service (WebRTC)

Video Service yêu cầu cao hơn về CPU và Network, đặc biệt là UDP.

**File: `k8s/base/video-deployment.yaml`** (Chỉ khác biệt)

```yaml
# ... (metadata giống trên)

  template:
    spec:
      containers:
      - name: video-service
        image: secureconnect/video-service:v1.0.0
        
        # WebRTC cần UDP traffic
        ports:
        - containerPort: 8080  # Signaling (TCP)
          name: http
        - containerPort: 5004-5200 # Dynamic UDP Ports (Nếu dùng SFU với exposed ports)
          protocol: UDP
          
        resources:
          requests:
            memory: "512Mi"   # RAM cao hơn chat
            cpu: "1000m"      # 1 Core
          limits:
            memory: "1Gi"
            cpu: "2000m"      # Max 2 Cores
```

---

## 19.8. Ingress Controller (Nginx)

Để tiếp nhận traffic từ Internet (HTTPS) và route đến các Services.

**Giả định bạn đã cài Nginx Ingress Controller:**

**File: `k8s/base/ingress.yaml`**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: secureconnect-ingress
  namespace: secureconnect-prod
  annotations:
    # Sử dụng Cert-Manager để tự động tạo SSL Let's Encrypt (Hoặc tự upload cert)
    cert-manager.io/cluster-issuer: "letsencrypt-prod" 
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/use-regex: "true"
    # Config cho WebSocket (Quan trọng cho Chat/Video Signaling)
    nginx.ingress.kubernetes.io/proxy-connect-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/websocket-services: "chat-service"
    
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - api.secureconnect.com
    - signal.secureconnect.com
    secretName: secureconnect-tls-secret
  
  rules:
  # 1. API Gateway & Chat Service
  - host: api.secureconnect.com
    http:
      paths:
      - path: /v1/healthz
        pathType: Prefix
        backend:
          service:
            name: api-gateway
            port:
              number: 8080
      - path: /v1
        pathType: Prefix
        backend:
          service:
            name: api-gateway
            port:
              number: 8080
              
  # 2. Signaling Service (WebSocket - riêng domain hoặc path)
  - host: signal.secureconnect.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: video-service
            port:
              number: 8080
```

---

## 19.9. Horizontal Pod Autoscaler (HPA)

Tự động scale khi tải tăng (ví dụ: lượng người dùng truy cập tăng gấp đôi).

**File: `k8s/base/hpa.yaml`**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: chat-service-hpa
  namespace: secureconnect-prod
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: chat-service
  
  minReplicas: 3
  maxReplicas: 10             # Tối đa scale lên 10 pods
  
  metrics:
  # Scale theo CPU
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70  # Khi CPU > 70% -> Scale up
  
  # Scale theo RAM
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## 19.10. Quản lý Database với Helm

Việc viết manifests thủ công cho Cassandra/CockroachDB rất rủi ro (dễ mất data). Nên sử dụng **Helm Charts** có sẵn.

**Deploy CockroachDB:**
```bash
# Add repo
helm repo add cockroachdb https://charts.cockroachdb.com/

# Install
helm install my-cockroachdb cockroachdb/cockroachdb \
  --namespace secureconnect-prod \
  --set statefulset.replicas=3 \
  --set tls.enabled=true
```

**Deploy Redis:**
```bash
helm install my-redis bitnami/redis \
  --namespace secureconnect-prod \
  --set architecture=standalone \
  --set auth.password=your-redis-pass
```

---

## 19.11. Quy trình Deploy (Workflow)

1.  **Docker Build:** (Xem file `devops/docker-setup.md`).
2.  **Push Image:** Đẩy image lên Container Registry (Docker Hub, AWS ECR, GCR).
3.  **Kustomize:** Sử dụng Kustomize để merge các patches (thay đổi image tag, số replicas) giữa môi trường.
    ```bash
    # Áp dụng môi trường Prod
    kubectl apply -k k8s/overlays/prod
    ```
4.  **Verify:**
    ```bash
    kubectl get pods -n secureconnect-prod
    kubectl get hpa -n secureconnect-prod
    ```

---

## 19.12. Best Practices cho K8s

1.  **Requests & Limits:** Luôn set cả 2 giá trị này. Nếu không set `requests`, K8s không biết bạn cần bao nhiêu tài nguyên để schedule pod. Nếu không set `limits`, 1 pod bị lỗi RAM có thể làm sập cả node (OOM Killer).
2.  **Liveness vs Readiness:**
    *   `livenessProbe`: Nếu fail -> K8s **kill** và restart lại pod.
    *   `readinessProbe`: Nếu fail -> Traffic **không gửi** vào pod này nữa (dễ bị lỗi mạng tạm thời). Đừng set liveness và readiness giống nhau.
3.  **Termination Grace Period:** Mặc định là 30s. Với Chat/WebSocket, hãy tăng lên 60-90s để K8s chờ xử lý xong kết nối trước khi ép tắt pod.

---

*Liên kết đến tài liệu tiếp theo:* `devops/ci-cd-pipeline.md`