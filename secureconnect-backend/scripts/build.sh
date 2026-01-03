#!/bin/bash

# Lưu ý: Script này chạy từ thư mục gốc secureconnect-backend/

# echo "Start building SecureConnect Services..."

# # Build Auth Service
# echo "Building auth-service..."
# CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/auth-service ./cmd/auth-service

# # Build Chat Service
# echo "Building chat-service..."
# CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/chat-service ./cmd/chat-service

# # Build API Gateway
# echo "Building api-gateway..."
# CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api-gateway ./cmd/api-gateway

# # Build Video Service
# echo "Building video-service..."
# CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/video-service ./cmd/video-service

# echo "Build Complete!"

#!/bin/bash

echo "Bắt đầu xây dựng (Build) các SecureConnect Services..."

# Biến môi trường chứa danh sách các service cần build
SERVICES=("api-gateway" "auth-service" "chat-service" "video-service")

# Chạy go mod tidy để đảm bảo go.mod và go.sum đồng bộ
echo "Running go mod tidy..."
go mod tidy

# Vòng lặp qua từng service để build
for SERVICE_NAME in "${SERVICES[@]}"; do
    echo "-------------------------------------------------"
    echo "Đang build: $SERVICE_NAME"
    
    # Cú pháp build:
    # CGO_ENABLED=0: Biên dịch binary không phụ thuộc C (file thực thi duy nhất, chạy mọi nơi).
    # GOOS=linux: Build cho Linux (vì Docker chạy Linux).
    # -o bin/$SERVICE_NAME: Chỉ tên file output.
    # ./cmd/$SERVICE_NAME: Vào thư mục source code của service đó.
    
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/${SERVICE_NAME} ./cmd/${SERVICE_NAME}
    
    if [ $? -eq 0 ]; then
        echo "SUCCESS: $SERVICE_NAME đã được build thành công."
    else
        echo "ERROR: Build thất bại tại $SERVICE_NAME"
        exit 1
    fi
done

echo "-------------------------------------------------"
echo "Hoàn tất!"