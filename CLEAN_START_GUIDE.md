# CLEAN START GUIDE
**Date**: 2026-01-25
**Purpose**: Clean Docker environment and start fresh

---

## 🔴 CURRENT ISSUE

Docker has multiple containers with conflicting names. The docker-compose command is creating new containers with random suffixes instead of reusing existing ones.

---

## 🔧 SOLUTION: Clean Start

### Step 1: Stop All SecureConnect Containers

```bash
docker stop $(docker ps -a --filter "name=secureconnect_" -q)
```

### Step 2: Remove All SecureConnect Containers

```bash
docker rm $(docker ps -a --filter "name=secureconnect_" -q)
```

### Step 3: Start Fresh with Both Compose Files

```bash
cd secureconnect-backend
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d
```

---

## 🔄 ONE-LINER COMMAND (PowerShell)

```powershell
cd d:\secureconnect\secureconnect-backend
docker stop $(docker ps -a --filter "name=secureconnect_" --format "{{.Names}}" -q)
docker rm $(docker ps -a --filter "name=secureconnect_" --format "{{.Names}}" -q)
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d
```

---

## 🔄 ONE-LINER COMMAND (Bash/WSL)

```bash
cd d:/secureconnect/secureconnect-backend
docker stop $(docker ps -a --filter "name=secureconnect_" --format "{{.Names}}" -q)
docker rm $(docker ps -a --filter "name=secureconnect_" --format "{{.Names}}" -q)
docker-compose -f docker-compose.production.yml -f docker-compose.logging.yml up -d
```

---

## ✅ VERIFICATION

After clean start, verify all services are running:

```bash
docker ps --filter "name=secureconnect_"
```

**Expected Output**: All services should show "Up" status

---

## 📝 NOTES

- This will stop ALL SecureConnect containers
- All data will be preserved in Docker volumes
- Services will start with fresh container IDs
- No application code changes required

---

**Document Version**: 1.0
**Last Updated**: 2026-01-25
