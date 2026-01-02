# DevOps CI/CD Pipeline Guide

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 20.1. Tổng quan

CI/CD Pipeline giúp đội ngũ phát triển đẩy code nhanh chóng, an toàn và giảm thiểu lỗi người. Quy trình của chúng ta sẽ được triển khai trên **GitHub Actions** do tính tích hợp sâu với GitHub và hỗ trợ tốt cho Docker/Kubernetes.

### Quy trình tổng quát
1.  **Developer** push code lên branch `develop` hoặc `main`.
2.  **GitHub Actions** tự kích hoạt (Trigger).
3.  **CI Stage (Tích hợp):**
    *   Chạy Unit Tests (Go & Flutter).
    *   Chạy Static Analysis (Linting).
    *   Quét bảo mật (Security Scanning).
    *   Build Docker Image.
4.  **CD Stage (Triển khai):**
    *   Đẩy Image lên Docker Registry (DockerHub/AWS ECR).
    *   Deploy lên Kubernetes Cluster (Staging hoặc Production).

---

## 20.2. Chiến lược Branching (Git Flow)

Để CI/CD hoạt động hiệu quả, cần quy định quy tắc làm việc (Branching Strategy):

*   **`main`:** Môi trường **Production**. Chỉ có code đã test kỹ mới được merge vào đây. Deploy `main` lên K8s cần **phê duyệt thủ công (Manual Approval)**.
*   **`develop`:** Môi trường **Staging**. Các feature branch được merge về đây sau khi xong. Deploy `develop` lên K8s là **tự động**.
*   **`feature/xxx`:** Các nhánh phát triển tính năng mới. Không trigger deployment, chỉ chạy tests.

---

## 20.3. Cấu trúc Thư mục CI/CD

```bash
.github/
└── workflows/
    ├── backend-ci-cd.yml       # Pipeline cho Go Services
    └── frontend-ci-cd.yml      # Pipeline cho Flutter Web
```

---

## 20.4. Backend CI/CD Pipeline (Go)

File này sẽ thực hiện: Test -> Build Image -> Push Registry -> Deploy K8s.

**File: `.github/workflows/backend-ci-cd.yml`**

```yaml
name: Backend CI/CD Pipeline

on:
  push:
    branches:
      - develop
      - main
  pull_request:
    branches:
      - develop
      - main

# Biến môi trường
env:
  REGISTRY: docker.io  # Hoặc your-registry.com
  IMAGE_NAME: secureconnect/backend-service

jobs:
  # --- JOB 1: TEST & VALIDATE ---
  test:
    name: Test & Lint
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-

      - name: Download dependencies
        run: go mod download

      - name: Run Go Vet (Static Analysis)
        run: go vet ./...

      - name: Run Unit Tests
        run: go test -v -race -coverprofile=coverage.txt ./...

      - name: Upload coverage to Codecov (Tùy chọn)
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.txt

  # --- JOB 2: BUILD & PUSH IMAGE ---
  build-and-push:
    name: Build & Push Docker Image
    needs: test # Phải pass job test mới được chạy
    runs-on: ubuntu-latest
    if: github.event_name == 'push' # Chỉ build khi push trực tiếp, không build PR
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Set up Docker Buildx (Để hỗ trợ multi-platform nếu cần)
        uses: docker/setup-buildx-action@v2

      - name: Login to Docker Hub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}

      - name: Extract metadata (Tags)
        id: meta
        uses: docker/metadata-action@v4
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}

      - name: Build and push Docker image
        uses: docker/build-push-action@v4
        with:
          context: .
          file: ./Dockerfile.backend
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          build-args: |
            SERVICE_NAME=api-gateway # Hoặc lặp lại cho các service khác

  # --- JOB 3: DEPLOY TO STAGING (AUTO) ---
  deploy-staging:
    name: Deploy to Staging (K8s)
    needs: build-and-push
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Configure kubectl
        uses: azure/k8s-set-context@v3
        with:
          method: kubeconfig
          kubeconfig: ${{ secrets.KUBE_CONFIG_STAGING }} # Secret chứa file kubeconfig

      - name: Deploy using Kustomize
        run: |
          kubectl apply -k k8s/overlays/staging
          kubectl rollout restart deployment/api-gateway -n secureconnect-staging

  # --- JOB 4: DEPLOY TO PRODUCTION (MANUAL) ---
  deploy-production:
    name: Deploy to Production (K8s)
    needs: build-and-push
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main' && github.event_name == 'push'
    environment:
      name: production
      url: https://api.secureconnect.com
    # Yêu cầu phê duyệt thủ công trên GitHub UI
    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Configure kubectl
        uses: azure/k8s-set-context@v3
        with:
          method: kubeconfig
          kubeconfig: ${{ secrets.KUBE_CONFIG_PROD }}

      - name: Deploy using Kustomize
        run: |
          kubectl apply -k k8s/overlays/prod
          
      - name: Wait for rollout to finish
        run: kubectl rollout status deployment/api-gateway -n secureconnect-prod
```

---

## 20.5. Frontend CI/CD Pipeline (Flutter Web)

File này build Flutter Web thành Docker image (Nginx) và deploy.

**File: `.github/workflows/frontend-ci-cd.yml`**

```yaml
name: Frontend CI/CD Pipeline (Web)

on:
  push:
    branches: [develop, main]

jobs:
  test:
    name: Test Flutter
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Setup Flutter
        uses: subosito/flutter-action@v2
        with:
          flutter-version: '3.16.0'
          channel: 'stable'

      - name: Get dependencies
        run: flutter pub get

      - name: Analyze code
        run: flutter analyze

      - name: Run Tests
        run: flutter test --coverage --coverage-path=coverage/lcov.info

  build-deploy-web:
    name: Build & Deploy Web
    needs: test
    runs-on: ubuntu-latest
    if: github.event_name == 'push'
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Setup Flutter
        uses: subosito/flutter-action@v2
        with:
          flutter-version: '3.16.0'

      - name: Build Web App
        run: |
          flutter build web --release
          # Output sẽ nằm ở folder build/web

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to Docker Hub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}

      - name: Build and Push Web Image
        uses: docker/build-push-action@v4
        with:
          context: .
          file: ./Dockerfile.frontend
          push: true
          tags: ${{ secrets.DOCKER_USERNAME }}/secureconnect-web:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Deploy to Staging K8s
        if: github.ref == 'refs/heads/develop'
        uses: azure/k8s-set-context@v3
        with:
          method: kubeconfig
          kubeconfig: ${{ secrets.KUBE_CONFIG_STAGING }}
        
      - name: Update Web Deployment
        if: github.ref == 'refs/heads/develop'
        run: |
          # Patch image mới cho deployment web
          kubectl set image deployment/web-app -n secureconnect-staging \
            web-app=${{ secrets.DOCKER_USERNAME }}/secureconnect-web:latest
```

---

## 20.6. Quản lý Secrets (KUBE CONFIG & DOCKER PASS)

Để CI/CD hoạt động, bạn cần cung cấp thông tin đăng nhập cho GitHub.

### 1. Docker Hub Credentials
Vào Repository Settings -> **Secrets and variables** -> Actions.
*   **Name:** `DOCKER_USERNAME`
*   **Value:** username Docker Hub của bạn.
*   **Name:** `DOCKER_PASSWORD`
*   **Value:** Access Token (hoặc Password) Docker Hub.

### 2. Kubernetes Config (kubeconfig)
File `~/.kube/config` chứa thông tin cluster để `kubectl` có thể kết nối.
*   **Name:** `KUBE_CONFIG_STAGING`
*   **Value:** Nội dung file config cho Staging Cluster (Base64 encode thường an toàn hơn, nhưng GitHub Actions hỗ trợ text trực tiếp).
*   **Name:** `KUBE_CONFIG_PROD`
*   **Value:** Nội dung file config cho Production Cluster.

*Quan trọng:* Hãy tạo riêng một Service Account trên Kubernetes có quyền `edit` hoặc `admin` trong namespace `secureconnect-*` để CI/CD dùng, không dùng quyền admin toàn cluster.

---

## 20.7. Quản lý Environment Namespaces (GitHub Environments)

GitHub hỗ trợ tính năng **Environments** để quản lý protection rules (quy tắc bảo vệ môi trường).

1.  Vào Repository -> **Settings** -> **Environments**.
2.  Tạo môi trường: **Staging** và **Production**.
3.  **Cấu hình Production:**
    *   **Required reviewers:** Yêu cầu ít nhất 1 người (Lead Dev) phê duyệt trước khi deploy.
    *   **Wait timer:** Đợi 5 phút (thời gian kiểm tra lại).

Khi đó, trong file YML (`deploy-production` job), code chạy `environment: production` sẽ tự động kích hoạt các rule này.

---

## 20.8. Rolling Back (Hoàn tác Deployment)

Nếu version mới bị lỗi, chúng ta cần quay lại version cũ.

### Cách 1: Kubectl thủ công
```bash
# Xem lịch sử hình ảnh
kubectl rollout history deployment/api-gateway -n secureconnect-prod

# Rollback về bản trước đó
kubectl rollout undo deployment/api-gateway -n secureconnect-prod
```

### Cách 2: Re-run CI/CD cũ
Trên GitHub Actions, tìm lại workflow chạy trước đó (Success), chọn **Re-run jobs**. Nó sẽ build lại image cũ (tag cũ) và deploy đè lên.

---

## 20.9. Security Scanning (Quét lỗ hổng bảo mật)

Đây là bước rất quan trọng cho hệ thống SaaS, nhất là khi xử lý dữ liệu nhạy cảm. Hãy thêm step **Trivy** vào pipeline sau khi build image.

```yaml
# Thêm vào job build-and-push
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:latest
    format: 'sarif'
    output: 'trivy-results.sarif'

- name: Upload Trivy results to GitHub Security
  uses: github/codeql-action/upload-sarif@v2
  with:
    sarif_file: 'trivy-results.sarif'
```
Nếu phát hiện lỗ hổng critical/high, pipeline sẽ fail (failed) và chặn không cho deploy.

---

## 20.10. Chiến lược tối ưu (Optimization Tips)

1.  **Caching:** Sử dụng cache cho Go modules (`actions/cache`) và Docker layers (`cache-from: type=gha`). Giúp giảm thời gian build từ 5 phút xuống còn 30 giây.
2.  **Parallel Jobs:** Trong workflow YAML, job `test` và job `security-scan` có thể chạy song song nếu không phụ thuộc nhau (`needs: ...`) để tiết kiệm thời gian.
3.  **Notification:** Thêm step gửi tin nhắn về Slack/Discord khi deploy thành công hoặc thất bại để team nắm bắt tình trạng.
    ```yaml
    - name: Slack Notification
      if: always()
      uses: 8398a7/action-slack@v3
      with:
        status: ${{ job.status }}
        fields: repo,message,commit,author
      env:
        SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK }}
    ```

---

*Liên kết đến tài liệu tiếp theo:* `qa/testing-strategy.md`