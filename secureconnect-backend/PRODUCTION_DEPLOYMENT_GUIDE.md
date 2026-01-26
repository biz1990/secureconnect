# Production Deployment Guide - SecureConnect

**Date:** 2026-01-26  
**Version:** 2.0  
**Status:** Production Ready (with prerequisites)

---

## Prerequisites

Before deploying to production, ensure you have:

1. ✅ **Docker and Docker Compose** installed
2. ✅ **Production server** with adequate resources (minimum 8GB RAM, 4 CPU cores)
3. ✅ **Public IP address** for TURN server
4. ✅ **Domain name** with DNS configured
5. ✅ **SSL/TLS certificates** (Let's Encrypt or commercial CA)
6. ✅ **SMTP provider** (SendGrid, Mailgun, AWS SES, or Gmail App Password)
7. ✅ **Firebase project** with service account credentials
8. ✅ **Secrets generated** (see Secrets Setup section)

---

## Quick Start (Local Production Simulation)

For testing production configuration locally:

```bash
cd secureconnect-backend

# 1. Generate secrets
./scripts/generate-secret-files.sh

# 2. Start production stack with logging
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d --build

# 3. Check service health
docker-compose -f docker-compose.production.yml ps

# 4. View logs
docker-compose -f docker-compose.production.yml logs -f

# 5. Access services
# - API Gateway: http://localhost:8080
# - Grafana: http://localhost:3000 (admin / check secrets/grafana_admin_password.txt)
# - Prometheus: http://localhost:9091
# - MinIO Console: http://localhost:9001
```

---

## Production Deployment Steps

### Step 1: Server Preparation

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Create application directory
sudo mkdir -p /opt/secureconnect
sudo chown $USER:$USER /opt/secureconnect
cd /opt/secureconnect
```

### Step 2: Clone Repository

```bash
# Clone repository (use your actual repository URL)
git clone https://github.com/your-org/secureconnect.git
cd secureconnect/secureconnect-backend

# Checkout production branch
git checkout main
```

### Step 3: Generate Secrets

```bash
# Generate all secrets
./scripts/generate-secret-files.sh

# Verify secrets were created
ls -la secrets/

# Set proper permissions
chmod 600 secrets/*.txt secrets/*.json
```

### Step 4: Configure Environment

```bash
# Copy production environment template
cp .env.production.example .env.production

# Edit production environment
nano .env.production
```

**Required configurations:**

```bash
# TURN Server - CRITICAL
# Get your server's public IP: curl ifconfig.me
# Edit configs/turnserver.conf and replace:
# external-ip=YOUR_PUBLIC_IP/YOUR_PRIVATE_IP
# Example: external-ip=203.0.113.1/10.0.0.5

# SMTP Configuration
SMTP_HOST=smtp.sendgrid.net  # or smtp.gmail.com
SMTP_PORT=587
# Update secrets/smtp_username.txt and secrets/smtp_password.txt

# Firebase Configuration
# Upload your firebase-adminsdk.json to secrets/firebase_credentials.json
# Update secrets/firebase_project_id.txt with your project ID

# Domain Configuration
APP_URL=https://yourdomain.com
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://api.yourdomain.com
```

### Step 5: Configure TLS/HTTPS

**Option A: Let's Encrypt (Recommended)**

```bash
# Install certbot
sudo apt install certbot python3-certbot-nginx -y

# Generate certificates
sudo certbot certonly --standalone -d yourdomain.com -d api.yourdomain.com

# Certificates will be at:
# /etc/letsencrypt/live/yourdomain.com/fullchain.pem
# /etc/letsencrypt/live/yourdomain.com/privkey.pem
```

**Option B: Commercial Certificate**

Upload your certificate files to:
- `/opt/secureconnect/certs/fullchain.pem`
- `/opt/secureconnect/certs/privkey.pem`

**Update NGINX Configuration:**

Create `configs/nginx-ssl.conf`:

```nginx
upstream api_gateway {
    server api-gateway:8080;
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name yourdomain.com api.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

# HTTPS Server
server {
    listen 443 ssl http2;
    server_name yourdomain.com api.yourdomain.com;

    # SSL Configuration
    ssl_certificate /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Security Headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Proxy to API Gateway
    location / {
        proxy_pass http://api_gateway;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts
        proxy_connect_timeout 60;
        proxy_send_timeout 300;
        proxy_read_timeout 300;
    }
}
```

### Step 6: Configure Alertmanager

Edit `configs/alertmanager.yml`:

```yaml
# Update email configuration
email_configs:
  - to: 'ops-team@yourdomain.com'
    from: 'alertmanager@yourdomain.com'
    smarthost: 'smtp.sendgrid.net:587'
    auth_username: 'apikey'
    auth_password_file: '/run/secrets/smtp_password'
```

### Step 7: Deploy Services

```bash
# Build and start all services
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d --build

# Monitor deployment
docker-compose -f docker-compose.production.yml logs -f

# Wait for all services to be healthy (may take 2-3 minutes)
watch docker-compose -f docker-compose.production.yml ps
```

### Step 8: Verify Deployment

```bash
# Check service health
curl http://localhost:8080/health

# Check Prometheus targets
curl http://localhost:9091/api/v1/targets | jq

# Check Grafana
curl http://localhost:3000/api/health

# Check Loki
curl http://localhost:3100/ready

# Test TURN server
turnutils_uclient -u $(cat secrets/turn_user.txt) -w $(cat secrets/turn_password.txt) YOUR_PUBLIC_IP
```

### Step 9: Configure Firewall

```bash
# Allow only necessary ports
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP (will redirect to HTTPS)
sudo ufw allow 443/tcp   # HTTPS
sudo ufw allow 3478/udp  # TURN/STUN
sudo ufw allow 3478/tcp  # TURN/STUN
sudo ufw allow 5349/tcp  # TURN TLS
sudo ufw allow 50100:50150/udp  # TURN relay ports

# Enable firewall
sudo ufw enable
```

### Step 10: Set Up Monitoring

1. **Access Grafana:** https://yourdomain.com:3000
   - Login: admin / (check `secrets/grafana_admin_password.txt`)
   - Import dashboards from `configs/grafana-dashboard.json`

2. **Configure Prometheus Alerts:**
   - Verify alerts are firing: http://localhost:9091/alerts
   - Test alertmanager: http://localhost:9093

3. **Set Up Log Aggregation:**
   - Access Loki: http://localhost:3100
   - Query logs in Grafana using Loki datasource

---

## Post-Deployment Checklist

- [ ] All services are healthy (`docker-compose ps` shows all as "Up (healthy)")
- [ ] HTTPS is working (test https://yourdomain.com)
- [ ] TURN server is accessible (test with `turnutils_uclient`)
- [ ] Prometheus is scraping all targets (check /targets)
- [ ] Grafana dashboards are displaying metrics
- [ ] Loki is receiving logs (query in Grafana)
- [ ] Alertmanager is sending test alerts
- [ ] Database backups are running (check `/backups` volume)
- [ ] Firewall is configured correctly
- [ ] SSL certificates are valid and auto-renewing

---

## Maintenance

### Daily Tasks
- Monitor Grafana dashboards
- Check Prometheus alerts
- Review Loki logs for errors

### Weekly Tasks
- Verify database backups
- Check disk space usage
- Review security logs

### Monthly Tasks
- Update Docker images
- Rotate secrets (see SECRETS_ROTATION_GUIDE.md)
- Review and update firewall rules
- Test disaster recovery procedures

---

## Troubleshooting

### Services Not Starting

```bash
# Check logs
docker-compose -f docker-compose.production.yml logs SERVICE_NAME

# Common issues:
# 1. Secrets not generated - run ./scripts/generate-secret-files.sh
# 2. Port conflicts - check with: sudo netstat -tulpn
# 3. Insufficient resources - check: docker stats
```

### TURN Server Not Working

```bash
# Check TURN server logs
docker-compose -f docker-compose.production.yml logs turn-server

# Verify external IP is configured
grep "external-ip" configs/turnserver.conf

# Test TURN connectivity
turnutils_uclient -v -u $(cat secrets/turn_user.txt) -w $(cat secrets/turn_password.txt) YOUR_PUBLIC_IP
```

### Database Connection Issues

```bash
# Check CockroachDB
docker exec secureconnect_crdb cockroach sql --insecure -e "SHOW DATABASES;"

# Check Cassandra
docker exec secureconnect_cassandra cqlsh -u $(cat secrets/cassandra_user.txt) -p $(cat secrets/cassandra_password.txt) -e "DESCRIBE KEYSPACES;"

# Check Redis
docker exec secureconnect_redis redis-cli -a $(cat secrets/redis_password.txt) PING
```

---

## Rollback Procedure

If deployment fails:

```bash
# Stop all services
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml down

# Restore from backup (if needed)
./scripts/restore-databases.sh /backups/BACKUP_DATE

# Checkout previous version
git checkout PREVIOUS_TAG

# Redeploy
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d --build
```

---

## Security Recommendations

1. **Never commit secrets to git** - Always use Docker secrets or environment variables
2. **Rotate secrets regularly** - At least every 90 days
3. **Enable TLS for all services** - Including databases
4. **Restrict database ports** - Only allow internal network access
5. **Use strong passwords** - Minimum 24 characters for production
6. **Enable audit logging** - Monitor all administrative actions
7. **Keep Docker images updated** - Apply security patches regularly
8. **Use firewall rules** - Restrict access to only necessary ports

---

## Support

For issues or questions:
- Check logs: `docker-compose logs -f`
- Review audit reports in `/docs`
- Contact: ops-team@secureconnect.com

---

**Last Updated:** 2026-01-26  
**Document Version:** 2.0
