# Deployment & Monitoring Setup Guide

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 26.1. Tổng quan

Hệ thống giám sát (Monitoring) là "đôi mắt" của kỹ sư vận hành. Nếu hệ thống gặp lỗi (Database down, Memory leak, Video lag), bạn cần biết ngay lập tức thông qua Dashboard hoặc cảnh báo (Alerts), thay vì đợi người dùng báo.

### Công cụ sử dụng
*   **Prometheus:** Thu thập dữ liệu metrics (Số liệu hiệu năng) và lưu trữ dạng Time-Series Database.
*   **Grafana:** Hiển thị dữ liệu dưới dạng Biểu đồ (Charts/Dashboards) trực quan, đẹp mắt.
*   **AlertManager:** Quản lý và gửi cảnh báo (Alerts) khi chỉ số vượt ngưỡng (Threshold) sang Slack/Email.
*   **Node Exporter:** Thu thập thông tin phần cứng của Server (CPU, RAM, Disk I/O, Network).
*   **cAdvisor:** Thu thập thông tin của Docker Containers.

---

## 26.2. Kiến trúc Giám sát (Monitoring Architecture)

Sơ đồ luồng dữ liệu từ các Server ra Dashboard.

```mermaid
graph LR
    subgraph "Servers (Targets)"
        LB[LB Server]
        APP1[App Server 01]
        APP2[App Server 02]
        DB[DB Server]
        STORAGE[Storage Server]
    end

    subgraph "Exporters (Agents)"
        EXP_Node[Node Exporter<br/>(Hardware Metrics)]
        EXP_Cad[cAdvisor<br/>(Container Metrics)]
    end

    subgraph "Monitoring Server (Optional)"
        PROM[Prometheus Server]
        AMA[Alert Manager]
    end

    subgraph "Visualization"
        GRAF[Grafana Dashboards]
        SLACK[Slack Channel]
    end

    %% Data Flow
    LB --> EXP_Cad
    APP1 --> EXP_Node
    APP1 --> EXP_Cad
    DB --> EXP_Node
    STORAGE --> EXP_Node

    EXP_Node -->|Scrape /metrics| PROM
    EXP_Cad -->|Scrape /metrics| PROM

    PROM -->|Push Alerts| AMA
    AMA -->|Send Webhook| SLACK

    PROM -->|Query Data| GRAF
```

---

## 26.3. Chiến lược Triển khai (Deployment Strategy)

Để đơn giản hóa PoC và triển khai nhanh, chúng ta sẽ chạy Prometheus, Grafana và AlertManager dưới dạng **Docker Containers** trên một Server riêng biệt hoặc chung với `storage-01`.

**Chọn giải pháp:** Deploy Monitoring Stack trên **Docker Compose** trên Server `storage-01` (vì Server này có tài nguyên nhàn rỗi hơn DB/Apps).

---

## 26.4. Cấu hình Exporters trên các Server Agents

Để Prometheus thu thập được dữ liệu, mỗi Server (LB, Apps, DB, Storage) cần chạy 2 agent nhỏ:

### 4.1. Node Exporter (Theo dõi phần cứng)
Cài đặt trên TẤT CẢ 5 Servers (`lb-01`, `app-01`, `app-02`, `db-01`, `storage-01`).

**Cách thực hiện (Ansible):**
```yaml
# Trong file roles/setup_monitoring/tasks/main.yml
- name: Download Node Exporter binary
  get_url:
    url: https://github.com/prometheus/node_exporter/releases/download/v1.6.0/node_exporter-1.6.0.linux-amd64.tar.gz
    dest: /tmp/node_exporter.tar.gz

- name: Extract Node Exporter
  unarchive:
    src: /tmp/node_exporter.tar.gz
    dest: /opt/node_exporter
    remote_src: yes

- name: Create Systemd Service for Node Exporter
  copy:
    dest: /etc/systemd/system/node_exporter.service
    content: |
      [Unit]
      Description=Node Exporter
      After=network.target

      [Service]
      User=prometheus
      ExecStart=/opt/node_exporter/node_exporter --collector.filesystem.mount-points-exclude=^/(sys|proc|dev|host|etc)({, |})($|.+)'
      Restart=always
      Type=simple

- name: Enable and Start Node Exporter
  systemd:
    name: node_exporter
    state: started
    enabled: yes
```
*   *Sau khi cài đặt:* Agent này sẽ mở port `9100` để Prometheus scrape.

### 4.2. Docker & cAdvisor (Theo dõi Container)
Nếu các App Services (Go) chạy trong Docker, cAdvisor thường được tích hợp sẵn trong Docker. Tuy nhiên, để Prometheus scrape được, cần bật Docker Daemon Metric Address.

**Cấu hình Docker Daemon (`/etc/docker/daemon.json` trên mỗi Server):**
```json
{
  "metrics-addr": "127.0.0.1:9323",
  "experimental": true
}
```
*   *Lưu ý:* Cấu hình xong cần restart Docker: `systemctl restart docker`.

---

## 26.5. Triển khai Monitoring Stack (Docker Compose)

Tạo file `docker-compose.monitoring.yml` trên Server `storage-01`.

```yaml
version: '3.8'

services:
  # 1. Prometheus Server
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./configs/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h' # Lưu giữ data trong 200h
    restart: unless-stopped

  # 2. Grafana Dashboard
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin_secure_pass
      - GF_INSTALL_PLUGINS= # (Tùy chọn) grafana-clock-panel,grafana-worldmap-panel
    volumes:
      - grafana_data:/var/lib/grafana
      - ./configs/grafana/provisioning:/etc/grafana/provisioning
    depends_on:
      - prometheus
    restart: unless-stopped

  # 3. AlertManager
  alertmanager:
    image: prom/alertmanager:latest
    container_name: alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./configs/alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml
    restart: unless-stopped

volumes:
  prometheus_data:
  grafana_data:
```

---

## 26.6. Cấu hình Prometheus (`prometheus.yml`)

Đây là "bộ não" thu thập dữ liệu. File này được mount vào container.

**File: `configs/prometheus.yml`**

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'secureconnect-prod'
    environment: 'production'

# Alert Rules (Định nghĩa khi nào cảnh báo)
rule_files:
  - "/etc/prometheus/rules/alerts.yml"

# Đích thu thập (Scrape Configs)
scrape_configs:
  # 1. Scrape Node Exporter (Hardware Metrics) trên các Server
  - job_name: 'nodes'
    static_configs:
      - targets: 
          - 'lb-01:9100'
          - 'app-01:9100'
          - 'app-02:9100'
          - 'db-01:9100'
          - 'storage-01:9100'

  # 2. Scrape cAdvisor (Container Metrics) trên các Server có Docker
  - job_name: 'docker'
    static_configs:
      - targets: 
          - 'lb-01:9323'
          - 'app-01:9323'
          - 'app-02:9323'
          - 'db-01:9323'
          - 'storage-01:9323'

  # 3. Scrape Go App Metrics (Business Metrics)
  # Các service Go phải chạy HTTP Metrics ở port :8081 (Ví dụ)
  - job_name: 'secureconnect-services'
    static_configs:
      - targets:
          - 'lb-01:8081'
          - 'app-01:8081'
          - 'app-02:8081'
          - 'db-01:8081' # Ví dụ: Chat Service trả về metrics DB connections
          - 'storage-01:8081'
```

---

## 26.7. Cấu hình Alert Rules (`alerts.yml`)

Định nghĩa các quy tắc cảnh báo.

**File: `configs/prometheus/rules/alerts.yml`**

```yaml
groups:
  - name: secureconnect-alerts
    interval: 30s
    rules:
      # --- Service Down ---
      - alert: ServiceDown
        expr: up{job="nodes"} == 0
        for: 5m
        labels:
          severity: critical
          type: server_down
        annotations:
          summary: "Node Exporter unavailable"
          description: "Server {{ $labels.instance }} đã down hơn 5 phút."

      # --- High CPU Usage ---
      - alert: HighCPUUsage
        expr: 100 - (avg by (instance) (irate(node_cpu_seconds_total{job="nodes"}[5m])) * 100) > 80
        for: 5m
        labels:
          severity: warning
          type: performance
        annotations:
          summary: "High CPU Usage on {{ $labels.instance }}"
          description: "CPU vượt 80% trong 5 phút qua."

      # --- High Memory Usage ---
      - alert: HighMemoryUsage
        expr: (1 - (node_memory_MemAvailable_bytes{job="nodes"} / node_memory_MemTotal_bytes{job="nodes"})) * 100 > 90
        for: 5m
        labels:
          severity: warning
          type: performance
        annotations:
          summary: "High Memory Usage on {{ $labels.instance }}"
          description: "RAM dùng > 90%."

      # --- Database Connection Too High ---
      # Giả lập metric 'db_connections_total' được export bởi Go Service
      - alert: DatabaseOverload
        expr: db_connections_total > 1000
        for: 5m
        labels:
          severity: critical
          type: database
        annotations:
          summary: "Database Connection Pool Exhausted"
          description: "Số lượng kết nối DB quá cao."
```

---

## 26.8. Cấu hình AlertManager (`alertmanager.yml`)

Quy định nơi và cách gửi cảnh báo.

**File: `configs/alertmanager/alertmanager.yml`**

```yaml
global:
  resolve_timeout: 5m

# Route các cảnh báo
routes:
  - match:
      severity: critical
    receiver: 'slack-critical' # Gửi kênh Slack nghiêm trọng
  - match:
      severity: warning
    receiver: 'slack-warnings'

# Người nhận (Receivers)
receivers:
  - name: 'slack-critical'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR_SLACK_WEBHOOK_CRITICAL'
        channel: '#on-call'
        send_resolved: true # Gửi tin nhắn khi vấn đề được giải quyết (Recovery)

  - name: 'slack-warnings'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR_SLACK_WEBHOOK_WARNINGS'
        channel: '#devops-alerts'
        send_resolved: true
```

---

## 26.9. Tích hợp Metrics vào Go Services

Đây là phần quan trọng nhất: Làm thế nào để Prometheus "thấy" được số liệu hiệu năng của app Go (số request, latency, goroutines)?

Sử dụng thư viện `prometheus/client_golang`.

### Ví dụ Code trong `cmd/api-gateway/main.go`:

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Metric: Số request HTTP theo method và path
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests.",
        },
        []string{"method", "path"},
    )

    // Metric: Thời gian xử lý request (Histogram)
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request latency distribution.",
            Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
        },
        []string{"method", "path"},
    )
)

func prometheusMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Bắt đầu thời gian
        start := time.Now()
        
        // 2. Xử lý request tiếp theo
        c.Next()
        
        // 3. Ghi nhận dữ liệu metrics
        duration := time.Since(start).Seconds()
        status := c.Writer.Status()
        path := c.FullPath()
        method := c.Request.Method

        httpRequestsTotal.WithLabelValues(method, path, status).Inc()
        httpRequestDuration.WithLabelValues(method, path).Observe(duration)
    }
}

func main() {
    // ... Code khởi tạo Gin Server ...
    
    r := gin.Default()
    r.Use(prometheusMiddleware())

    // Expose metrics endpoint cho Prometheus scrape
    // Prometheus sẽ gọi: GET http://lb-01:8081/metrics
    r.GET("/metrics", gin.WrapH(promhttp.Handler(), ""))

    r.Run(":8081")
}
```

---

## 26.10. Thiết lập Dashboard Grafana

Sau khi chạy Docker Compose, truy cập `http://<storage-01-ip>:3000`. Đăng nhập bằng `admin` / `admin_secure_pass`.

### 10.1. Thêm Data Source Prometheus
1.  Configuration -> Data Sources -> Add data source.
2.  Name: `Prometheus`.
3.  Type: `Prometheus`.
4.  URL: `http://prometheus:9090` (Tại sao dùng hostname "prometheus"? Vì cùng trong Docker network của Compose).
5.  Click Save & Test.

### 10.2. Dashboard Cần thiết
Bạn có thể import các dashboard JSON có sẵn, hoặc tạo mới. Các panel quan trọng cần thiết lập:

*   **Node Exporter Full:** Theo dõi CPU, RAM, Disk của 5 Servers.
*   **Docker Containers Overview:** Theo dõi CPU/RAM của từng container riêng biệt (Chat, Video, DB).
*   **Go Application Metrics:**
    *   *Panel 1:* Request Rate (QPS) - Biết bao nhiêu người đang dùng hệ thống.
    *   *Panel 2:* Latency Histogram (P95, P99) - Biết API có bị chậm không.
    *   *Panel 3:* Error Rate (HTTP 5xx) - Biết app có bị crash không.

---

## 26.11. Lưu ý Quan trọng cho Video Call

WebRTC metrics rất phức tạp. Để theo dõi chất lượng cuộc gọi:

1.  **Nên dùng WebRTC Agent:** Thư viện `Pion WebRTC` có cung cấp metrics, nhưng bạn cần expose chúng qua HTTP để Prometheus scrape được.
2.  **Các chỉ số quan trọng:**
    *   `ice_candidates_count`: Số lượng ICE candidate (quá nhiều = kết nối mạng xấu).
    *   `packets_lost_percent`: Tỷ lệ gói tin nhắn bị mất (Video giật).
    *   `bitrate`: Bitrate upload/download.

---

## 26.12. Bảo mật Monitoring

*   **Tường lửa (Firewall):** Chỉ cho phép IP quản trị (IP của DevOps hoặc Laptop bạn) truy cập vào port `3000` (Grafana) và `9090` (Prometheus). Đừng mở Public.
*   **Nginx Proxy:** Nếu muốn truy cập từ bên ngoài, hãy đặt Prometheus/Grafana sau một Nginx với Basic Auth (User/Pass).
*   **Dữ liệu nhạy cảm:** Prometheus logs có thể chứa thông tin `path` của request (ví dụ: `/users/login`), có thể dò ra user. Cẩn thận khi hiển thị logs trong Grafana.

---

*Liên kết đến tài liệu tiếp theo:* `deployment/backups-restore-policy.md`