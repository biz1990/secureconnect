# DOCKER CONFLICT RESOLUTION
**Date**: 2026-01-25
**Issue**: Docker Compose Container Name Conflict

---

## 🔴 ISSUE

When running `docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d`, there's a conflict:

```
Error response from daemon: Conflict. The container name "/secureconnect_alertmanager" is already in use by container "cae1c3e2f067eb89dc8c33930960252fa28d3add91f86c8bfca95b98f48bf896"
```

---

## 🔍 ROOT CAUSE

The alertmanager container is already running from a previous docker-compose session. When docker-compose.production.yml tries to recreate it, there's a conflict with the existing container.

---

## 🔧 SOLUTIONS

### Option 1: Stop and Remove Old Containers (RECOMMENDED)

Stop all containers and start fresh:

```bash
# Stop all secureconnect containers
docker stop $(docker ps -a --filter "name=secureconnect_" -q)

# Remove all secureconnect containers
docker rm $(docker ps -a --filter "name=secureconnect_" -q)

# Start fresh with both compose files
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d
```

### Option 2: Remove Only Conflicting Container

Remove only the conflicting alertmanager container:

```bash
# Stop and remove alertmanager
docker stop secureconnect_alertmanager
docker rm secureconnect_alertmanager

# Continue with compose
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d
```

### Option 3: Use --force-recreate

Force recreate all containers:

```bash
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d --force-recreate
```

---

## ✅ VERIFICATION

After resolving the conflict, verify all services are running:

```bash
# Check all containers
docker ps --filter "name=secureconnect_"

# Expected output: All services should be "Up"
```

---

## 📝 NOTES

- The logging stack (Loki, Promtail, Grafana) is already running from a previous compose session
- Option 1 is the cleanest approach - stops everything and starts fresh
- Option 2 is the fastest approach - only removes the conflicting container
- Option 3 forces recreation of all containers

---

**Document Version**: 1.0
**Last Updated**: 2026-01-25
