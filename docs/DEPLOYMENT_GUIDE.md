# SecureConnect Deployment Guide

## 🚀 Quick Start (Development)

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- Git

### 1. Clone Repository
```bash
git clone https://github.com/yourusername/secureconnect-backend.git
cd secureconnect-backend
```

### 2. Start Databases
```bash
docker-compose up -d
```

This starts:
- CockroachDB (port 26257, admin UI 8080)
- Cassandra (port 9042)
- Redis (port 6379)
- MinIO (port 9000, console 9001)

### 3. Initialize Databases
```bash
# Wait 30 seconds for Cassandra to be ready

# CockroachDB
docker exec -it secureconnect_crdb cockroach sql --insecure < scripts/cockroach-init.sql

# Cassandra
docker exec -it secureconnect_cassandra cqlsh -f /scripts/cassandra-init.cql
```

### 4. Install Dependencies
```bash
go mod download
```

### 5. Run Services

**In separate terminals:**

```bash
# Terminal 1: API Gateway
cd cmd/api-gateway
JWT_SECRET="dev-secret-key" go run main.go

# Terminal 2: Auth Service
cd cmd/auth-service
JWT_SECRET="dev-secret-key" go run main.go

# Terminal 3: Chat Service
cd cmd/chat-service
JWT_SECRET="dev-secret-key" go run main.go

# Terminal 4: Storage Service
cd cmd/storage-service
go run main.go
```

### 6. Test
```bash
curl http://localhost:8080/health
```

---

## 📦 Production Deployment

### Environment Variables

Create `.env` file:
```bash
# JWT
JWT_SECRET=your-super-secret-key-min-32-chars

# CockroachDB
DB_HOST=cockroachdb.example.com
DB_PORT=26257
DB_USER=secureconnect
DB_PASSWORD=strong-password
DB_NAME=secureconnect_prod
DB_SSL_MODE=require

# Cassandra
CASSANDRA_HOST=cassandra.example.com
CASSANDRA_KEYSPACE=secureconnect_ks
CASSANDRA_USER=secureconnect
CASSANDRA_PASSWORD=strong-password

# Redis
REDIS_HOST=redis.example.com
REDIS_PORT=6379
REDIS_PASSWORD=strong-password

# MinIO
MINIO_ENDPOINT=minio.example.com:9000
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
MINIO_BUCKET=secureconnect-files

# Services
API_GATEWAY_PORT=8080
AUTH_SERVICE_PORT=8081
CHAT_SERVICE_PORT=8082
VIDEO_SERVICE_PORT=8083
STORAGE_SERVICE_PORT=8084

# Environment
ENV=production
```

### Build Binaries
```bash
# API Gateway
CGO_ENABLED=0 GOOS=linux go build -o bin/api-gateway ./cmd/api-gateway

# Auth Service
CGO_ENABLED=0 GOOS=linux go build -o bin/auth-service ./cmd/auth-service

# Chat Service
CGO_ENABLED=0 GOOS=linux go build -o bin/chat-service ./cmd/chat-service

# Storage Service
CGO_ENABLED=0 GOOS=linux go build -o bin/storage-service ./cmd/storage-service
```

### Docker Images
```bash
# Build all images
docker build -f cmd/api-gateway/Dockerfile -t secureconnect/api-gateway:latest .
docker build -f cmd/auth-service/Dockerfile -t secureconnect/auth-service:latest .
docker build -f cmd/chat-service/Dockerfile -t secureconnect/chat-service:latest .
docker build -f cmd/storage-service/Dockerfile -t secureconnect/storage-service:latest .

# Push to registry
docker push secureconnect/api-gateway:latest
docker push secureconnect/auth-service:latest
docker push secureconnect/chat-service:latest
docker push secureconnect/storage-service:latest
```

---

## ☸️ Kubernetes Deployment

### 1. Create Namespace
```bash
kubectl create namespace secureconnect
```

### 2. Create Secrets
```bash
kubectl create secret generic secureconnect-secrets \
  --from-literal=jwt-secret=$JWT_SECRET \
  --from-literal=db-password=$DB_PASSWORD \
  --from-literal=redis-password=$REDIS_PASSWORD \
  --from-literal=minio-access-key=$MINIO_ACCESS_KEY \
  --from-literal=minio-secret-key=$MINIO_SECRET_KEY \
  -n secureconnect
```

### 3. Deploy Services
```bash
kubectl apply -f deployments/k8s/ -n secureconnect
```

### 4. Expose via Ingress
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: secureconnect-ingress
  namespace: secureconnect
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - api.secureconnect.io
    secretName: secureconnect-tls
  rules:
  - host: api.secureconnect.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: api-gateway
            port:
              number: 8080
```

---

## 🔧 Configuration

### CockroachDB Production Setup
```sql
-- Create dedicated user
CREATE USER secureconnect WITH PASSWORD 'strong-password';

-- Grant permissions
GRANT ALL ON DATABASE secureconnect_prod TO secureconnect;

-- Enable geo-partitioning (optional)
ALTER DATABASE secureconnect_prod CONFIGURE ZONE USING 
  num_replicas = 3, 
  constraints = '[+region=us-east]';
```

### Cassandra Production Setup
```cql
-- Create keyspace with replication
CREATE KEYSPACE IF NOT EXISTS secureconnect_ks
  WITH replication = {
    'class': 'NetworkTopologyStrategy',
    'datacenter1': 3
  };

-- Enable authentication
ALTER ROLE cassandra WITH PASSWORD = 'new-superuser-password';
CREATE ROLE secureconnect WITH PASSWORD = 'app-password' AND LOGIN = true;
GRANT ALL ON KEYSPACE secureconnect_ks TO secureconnect;
```

### Redis Production Setup
```bash
# redis.conf
requirepass strong-password
maxmemory 2gb
maxmemory-policy allkeys-lru
save 900 1
save 300 10
```

### MinIO Production Setup
```bash
# Start MinIO with TLS
minio server /data \
  --address ":9000" \
  --console-address ":9001" \
  --certs-dir /etc/minio/certs
```

---

## 📊 Monitoring

### Prometheus Metrics
Each service exposes `/metrics` endpoint:
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'api-gateway'
    static_configs:
      - targets: ['api-gateway:8080']
  
  - job_name: 'auth-service'
    static_configs:
      - targets: ['auth-service:8081']
  
  - job_name: 'chat-service'
    static_configs:
      - targets: ['chat-service:8082']
```

### Grafana Dashboards
Import provided dashboards:
- `monitoring/grafana/api-gateway.json`
- `monitoring/grafana/chat-service.json`
- `monitoring/grafana/database-metrics.json`

### Health Checks
```bash
# All services have /health endpoint
curl http://api-gateway:8080/health
curl http://auth-service:8081/health
curl http://chat-service:8082/health
curl http://storage-service:8084/health
```

---

## 🔐 Security Checklist

- [ ] Change all default passwords
- [ ] Use strong JWT secret (min 32 chars)
- [ ] Enable TLS/SSL for all services
- [ ] Configure firewall rules
- [ ] Enable database authentication
- [ ] Set up VPC/private network
- [ ] Configure rate limiting
- [ ] Enable audit logging
- [ ] Set up backup strategy
- [ ] Configure CORS properly
- [ ] Use secrets management (Vault, AWS Secrets Manager)

---

## 📈 Scaling

### Horizontal Scaling
```bash
# Scale services
kubectl scale deployment api-gateway --replicas=5 -n secureconnect
kubectl scale deployment chat-service --replicas=10 -n secureconnect
```

### Database Scaling

**CockroachDB:**
- Add nodes to cluster
- Rebalance automatically

**Cassandra:**
- Add nodes to ring
- Run `nodetool repair`

**Redis:**
- Use Redis Cluster or Sentinel
- Enable replication

---

## 🔄 CI/CD Pipeline

### GitHub Actions Example
```yaml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Build Docker images
        run: |
          docker build -f cmd/api-gateway/Dockerfile -t secureconnect/api-gateway:$GITHUB_SHA .
          
      - name: Push to registry
        run: |
          docker push secureconnect/api-gateway:$GITHUB_SHA
          
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/api-gateway api-gateway=secureconnect/api-gateway:$GITHUB_SHA
```

---

## 📝 Maintenance

### Backup Strategy
```bash
# CockroachDB
cockroach dump secureconnect_prod > backup.sql

# Cassandra
nodetool snapshot secureconnect_ks

# Redis
redis-cli BGSAVE
```

### Database Migrations
```bash
# Run migrations
go run migrations/main.go up

# Rollback
go run migrations/main.go down
```

---

## 🆘 Troubleshooting

### Service won't start
```bash
# Check logs
docker logs api-gateway
kubectl logs -f deployment/api-gateway -n secureconnect

# Check env variables
env | grep JWT_SECRET

# Test database connections
telnet cockroachdb.example.com 26257
```

### WebSocket connection fails
- Check CORS settings
- Verify JWT token
- Check firewall/load balancer timeout settings
- Ensure WebSocket upgrade is allowed

### High latency
- Check database query performance
- Review Redis cache hit rate
- Monitor network latency
- Check load balancer health

---

For more information, see:
- [API Documentation](./API_DOCUMENTATION.md)
- [Architecture Overview](./01-system-overview.md)
- [Security Architecture](./03-security-architecture.md)
