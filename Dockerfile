# --- STAGE 1: BUILD ---
# Use Go standard image for compilation
FROM golang:1.24-alpine AS builder

# Install required tools (git, cacerts)
RUN apk add --no-cache git ca-certificates tzdata curl

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first to utilize Docker Cache (very important for fast subsequent builds)
COPY secureconnect-backend/go.mod secureconnect-backend/go.sum ./

# Download library dependencies
RUN go mod download

# Copy all source code into container
# (Will copy cmd/, internal/, pkg/, directories)
COPY secureconnect-backend/. .

# --- BUILD BINARY ---
# Use ARG to know which service to build (passed by docker-compose)
ARG SERVICE_NAME=""
ARG CMD=""

# Build Go code into static binary file
# All binaries are named "service" for consistency
# Build from the cmd directory where main.go is located
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/service ./cmd/${SERVICE_NAME}

# --- STAGE 2: RUN ---
# Use Alpine lightweight image to run the application (actual runtime image)
FROM alpine:latest

# Install link libraries (missing causes Go code errors)
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user to increase security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary file from Stage Builder to Stage Runner
# All binaries are named "service"
COPY --from=builder /app/service /app/service

# Copy config files (if needed)
COPY secureconnect-backend/configs /app/configs

# Copy Swagger API documentation files
COPY secureconnect-backend/api /app/api

# Assign ownership to appuser
RUN chown -R appuser:appgroup /app

# Switch to appuser (don't run as root)
USER appuser

# Expose port 8082 for chat-service
EXPOSE 8082

# Run the binary
CMD ["./service"]
