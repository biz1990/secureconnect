# MANUAL SETUP GUIDE
**For SMTP and Firebase Configuration**

---

## 📍 FILE LOCATIONS

All manual setup files are located in: `d:/secureconnect/secureconnect-backend/secrets/`

### Files to Update:

1. **SMTP Username**: [`d:/secureconnect/secureconnect-backend/secrets/smtp_username.txt`](secureconnect-backend/secrets/smtp_username.txt:1)
2. **SMTP Password**: [`d:/secureconnect/secureconnect-backend/secrets/smtp_password.txt`](secureconnect-backend/secrets/smtp_password.txt:1)
3. **Firebase Project ID**: [`d:/secureconnect/secureconnect-backend/secrets/firebase_project_id.txt`](secureconnect-backend/secrets/firebase_project_id.txt:1)

---

## 📧 SMTP SETUP

### Current Values (Placeholders)

**File**: [`smtp_username.txt`](secureconnect-backend/secrets/smtp_username.txt:1)
```
noreply@example.com
```

**File**: [`smtp_password.txt`](secureconnect-backend/secrets/smtp_password.txt:1)
```
vy2gfyz0v8JyQmR68zb9aqpmgRsveSV6
```

### How to Update

#### Option 1: Using VSCode

1. Open VSCode
2. Navigate to: `d:/secureconnect/secureconnect-backend/secrets/`
3. Open `smtp_username.txt`
4. Replace `noreply@example.com` with your SMTP username
5. Save the file
6. Open `smtp_password.txt`
7. Replace the password with your SMTP password
8. Save the file

#### Option 2: Using PowerShell

```powershell
# Update SMTP username
Set-Content -Path "d:\secureconnect\secureconnect-backend\secrets\smtp_username.txt" -Value "your-smtp-username" -NoNewline

# Update SMTP password
Set-Content -Path "d:\secureconnect\secureconnect-backend\secrets\smtp_password.txt" -Value "your-smtp-password" -NoNewline
```

#### Option 3: Using Notepad

1. Open Notepad
2. File > Open
3. Navigate to: `d:\secureconnect\secureconnect-backend\secrets\`
4. Open `smtp_username.txt`
5. Replace content with your SMTP username
6. Save
7. Repeat for `smtp_password.txt`

### Where to Get SMTP Credentials

#### SendGrid
1. Go to https://sendgrid.com/
2. Sign in or create account
3. Navigate to Settings > API Keys
4. Create API Key with "Mail Send" permissions
5. Use API Key as username, "apikey" as password

#### Mailgun
1. Go to https://www.mailgun.com/
2. Sign in or create account
3. Navigate to Settings > API Keys
4. Use API Key as password, your domain as username

#### AWS SES
1. Go to AWS Console > SES
2. Create SMTP credentials
3. Use provided username and password

#### Gmail (Not recommended for production)
1. Go to Google Account > Security
2. Enable 2-Step Verification
3. Generate App Password
4. Use app-specific password

---

## 🔥 FIREBASE SETUP

### Current Value

**File**: [`firebase_project_id.txt`](secureconnect-backend/secrets/firebase_project_id.txt:1)
```
secureconnect-dev
```

### How to Update

#### Option 1: Using VSCode

1. Open VSCode
2. Navigate to: `d:/secureconnect/secureconnect-backend/secrets/`
3. Open `firebase_project_id.txt`
4. Replace `secureconnect-dev` with your Firebase project ID
5. Save the file

#### Option 2: Using PowerShell

```powershell
# Update Firebase project ID
Set-Content -Path "d:\secureconnect\secureconnect-backend\secrets\firebase_project_id.txt" -Value "your-firebase-project-id" -NoNewline
```

#### Option 3: Using Notepad

1. Open Notepad
2. File > Open
3. Navigate to: `d:\secureconnect\secureconnect-backend\secrets\`
4. Open `firebase_project_id.txt`
5. Replace content with your Firebase project ID
6. Save

### Where to Find Firebase Project ID

#### From Firebase Console

1. Go to https://console.firebase.google.com/
2. Select your project
3. Click on the gear icon (Project Settings)
4. Project ID is shown at the top of the General tab

#### From Firebase Credentials File

The project ID is also in your Firebase credentials file:
1. Open [`secureconnect-backend/secrets/firebase_credentials.json`](secureconnect-backend/secrets/firebase_credentials.json:1)
2. Look for `"project_id"` field
3. Copy that value to [`firebase_project_id.txt`](secureconnect-backend/secrets/firebase_project_id.txt:1)

Example:
```json
{
  "type": "service_account",
  "project_id": "your-project-id-here",
  ...
}
```

---

## ✅ VERIFICATION

After updating the files, verify they are correct:

### Check SMTP Files

```powershell
Get-Content "d:\secureconnect\secureconnect-backend\secrets\smtp_username.txt"
Get-Content "d:\secureconnect\secureconnect-backend\secrets\smtp_password.txt"
```

### Check Firebase Project ID

```powershell
Get-Content "d:\secureconnect\secureconnect-backend\secrets\firebase_project_id.txt"
```

---

## 🔄 DOCKER RESTART

After updating all files, restart Docker services:

```bash
cd d:/secureconnect/secureconnect-backend
docker-compose -f docker-compose.production.yml down
docker-compose -f docker-compose.production.yml up -d
```

Or using PowerShell:

```powershell
cd d:\secureconnect\secureconnect-backend
docker-compose -f docker-compose.production.yml down
docker-compose -f docker-compose.production.yml up -d
```

---

## 🧪 TESTING

### Test Email Sending

1. Trigger a password reset email
2. Check your email inbox
3. Verify email was received

### Test Firebase Integration

1. Trigger a push notification
2. Check your device
3. Verify notification was received

---

## 📋 CHECKLIST

- [ ] SMTP username updated in [`smtp_username.txt`](secureconnect-backend/secrets/smtp_username.txt:1)
- [ ] SMTP password updated in [`smtp_password.txt`](secureconnect-backend/secrets/smtp_password.txt:1)
- [ ] Firebase project ID updated in [`firebase_project_id.txt`](secureconnect-backend/secrets/firebase_project_id.txt:1)
- [ ] Docker services restarted
- [ ] Email sending tested
- [ ] Firebase integration tested

---

**Document Version**: 1.0
**Last Updated**: 2026-01-25
