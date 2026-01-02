## Tóm tắt các điểm quan trọng về E2EE:

### **1. Cryptographic Libraries:**
- **libsodium-wrappers**: Modern, easy-to-use crypto library
- Hỗ trợ: X25519 (ECDH), Ed25519 (signing), XSalsa20-Poly1305 (encryption)

### **2. Key Management Flow:**

```
User Registration → Generate Keys → Upload Public Keys → Server Storage
                                                              ↓
User A wants to message User B ← Fetch B's Public Keys ← Server
                ↓
        Establish Session (X3DH)
                ↓
        Double Ratchet Encryption
                ↓
        Send Encrypted Message → Server → User B
```

### **3. Message Lifecycle:**

```
Plaintext Message
    ↓
Derive Message Key from Chain Key
    ↓
Encrypt with AES-256 (or XSalsa20)
    ↓
Ratchet Chain Key Forward
    ↓
Send Ciphertext + Header + Nonce
    ↓
Server stores (cannot decrypt)
    ↓
Recipient receives
    ↓
Derive same Message Key
    ↓
Decrypt → Plaintext
```

### **4. Security Features:**

- **Perfect Forward Secrecy**: Compromise of long-term keys doesn't compromise past messages
- **Future Secrecy**: Compromise of session keys doesn't compromise future messages  
- **Deniability**: Cannot prove who sent a message
- **Integrity**: AEAD encryption ensures messages aren't tampered with

### **5. Group Chat Specifics:**

Sender Keys cho group chat hiệu quả hơn multiple 1-1 encryption:
- Mỗi người gửi có một sender key
- Sender key được mã hóa và gửi cho từng thành viên
- Tin nhắn group chỉ cần mã hóa 1 lần thay vì N lần

### **6. Media Files:**

- Mỗi file có encryption key riêng
- Key được gửi trong message metadata (đã mã hóa)
- Server chỉ lưu file encrypted, không có key

### **7. Storage Security:**

```typescript
// IndexedDB databases
- crypto_storage: Identity keys
- sessions: Session states
- signed_prekeys: Signed pre-keys
- onetime_prekeys: One-time pre-keys
- sender_keys: Group sender keys
- message_keys: Stored for out-of-order messages
```

### **8. Key Rotation:**

```typescript
// Automated key rotation
setInterval(async () => {
  await rotateSignedPreKey();
}, 7 * 24 * 60 * 60 * 1000); // 7 days

// Check and replenish one-time pre-keys
setInterval(async () => {
  const count = await getUnusedPreKeyCount();
  if (count < 20) {
    await replenishOneTimePreKeys();
  }
}, 24 * 60 * 60 * 1000); // Daily
```

### **9. Verification:**

Người dùng nên verify safety numbers để chống MITM:
```
Alice's Safety Number: 12345 67890 12345 67890 12345 67890
Bob's Safety Number:   12345 67890 12345 67890 12345 67890

If numbers match → Secure communication
If numbers differ → Potential MITM attack
```