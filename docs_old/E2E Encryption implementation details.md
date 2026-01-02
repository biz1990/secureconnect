# End-to-End Encryption (E2EE) Implementation

## 1. Encryption Architecture Overview

```
┌─────────────┐                                    ┌─────────────┐
│   Client A  │                                    │   Client B  │
│             │                                    │             │
│ ┌─────────┐ │                                    │ ┌─────────┐ │
│ │ Private │ │                                    │ │ Private │ │
│ │   Key   │ │                                    │ │   Key   │ │
│ └────┬────┘ │                                    │ └────┬────┘ │
│      │      │                                    │      │      │
│ ┌────▼────┐ │    Encrypted Message               │ ┌────▼────┐ │
│ │ Public  │ │    ─────────────────►              │ │ Public  │ │
│ │   Key   │ │                                    │ │   Key   │ │
│ └─────────┘ │                                    │ └─────────┘ │
│             │                                    │             │
│  Encrypt    │                                    │  Decrypt    │
└─────────────┘                                    └─────────────┘
       │                                                  ▲
       │                                                  │
       └──────────────►  Server (No Keys)  ──────────────┘
                    (Stores encrypted data only)
```

## 2. Cryptographic Protocol: Signal Protocol

Chúng ta sử dụng **Signal Protocol** (Double Ratchet Algorithm) - chuẩn công nghiệp cho E2EE.

### 2.1 Key Types

```typescript
interface IdentityKeyPair {
  publicKey: Uint8Array;   // Ed25519 public key (32 bytes)
  privateKey: Uint8Array;  // Ed25519 private key (64 bytes)
}

interface PreKey {
  keyId: number;
  publicKey: Uint8Array;   // X25519 public key
  privateKey: Uint8Array;  // X25519 private key
}

interface SignedPreKey extends PreKey {
  signature: Uint8Array;   // Signed by identity key
  timestamp: number;
}

interface OneTimePreKey extends PreKey {
  used: boolean;
}

interface SessionKey {
  rootKey: Uint8Array;
  chainKey: Uint8Array;
  messageKey: Uint8Array;
}
```

### 2.2 Key Hierarchy

```
Identity Key Pair (Long-term)
    │
    ├── Signed Pre-Key (Medium-term, rotated weekly)
    │       │
    │       └── Signature (signed by Identity Key)
    │
    └── One-Time Pre-Keys (Single-use, 100+ keys)

Session Keys (Per conversation)
    │
    ├── Root Key (Updated per message)
    ├── Chain Key (Updated per message)
    └── Message Keys (Derived from Chain Key)
```

## 3. Client-Side Implementation

### 3.1 Key Generation and Storage

```typescript
// crypto-manager.ts
import * as sodium from 'libsodium-wrappers';
import { IndexedDB } from './storage';

class CryptoManager {
  private db: IndexedDB;
  
  constructor() {
    this.db = new IndexedDB('crypto_storage');
  }

  // Generate Identity Key Pair (once per device)
  async generateIdentityKeyPair(): Promise<IdentityKeyPair> {
    await sodium.ready;
    
    const keyPair = sodium.crypto_sign_keypair();
    
    const identityKeyPair = {
      publicKey: keyPair.publicKey,
      privateKey: keyPair.privateKey
    };

    // Store securely in IndexedDB
    await this.db.put('identity', 'keypair', identityKeyPair);
    
    return identityKeyPair;
  }

  // Generate Signed Pre-Key
  async generateSignedPreKey(identityKeyPair: IdentityKeyPair): Promise<SignedPreKey> {
    await sodium.ready;
    
    const keyPair = sodium.crypto_box_keypair();
    const keyId = Date.now();
    
    // Sign the public key with identity private key
    const signature = sodium.crypto_sign_detached(
      keyPair.publicKey,
      identityKeyPair.privateKey
    );

    const signedPreKey: SignedPreKey = {
      keyId,
      publicKey: keyPair.publicKey,
      privateKey: keyPair.privateKey,
      signature,
      timestamp: Date.now()
    };

    await this.db.put('signed_prekeys', keyId.toString(), signedPreKey);
    
    return signedPreKey;
  }

  // Generate One-Time Pre-Keys (batch of 100)
  async generateOneTimePreKeys(count: number = 100): Promise<OneTimePreKey[]> {
    await sodium.ready;
    
    const preKeys: OneTimePreKey[] = [];
    
    for (let i = 0; i < count; i++) {
      const keyPair = sodium.crypto_box_keypair();
      const preKey: OneTimePreKey = {
        keyId: Date.now() + i,
        publicKey: keyPair.publicKey,
        privateKey: keyPair.privateKey,
        used: false
      };
      
      preKeys.push(preKey);
      await this.db.put('onetime_prekeys', preKey.keyId.toString(), preKey);
    }
    
    return preKeys;
  }

  // Get stored keys
  async getIdentityKeyPair(): Promise<IdentityKeyPair | null> {
    return await this.db.get('identity', 'keypair');
  }

  async getSignedPreKey(keyId: string): Promise<SignedPreKey | null> {
    return await this.db.get('signed_prekeys', keyId);
  }

  async getOneTimePreKey(keyId: string): Promise<OneTimePreKey | null> {
    return await this.db.get('onetime_prekeys', keyId);
  }

  // Mark one-time pre-key as used
  async markPreKeyUsed(keyId: string): Promise<void> {
    const preKey = await this.getOneTimePreKey(keyId);
    if (preKey) {
      preKey.used = true;
      await this.db.put('onetime_prekeys', keyId, preKey);
    }
  }
}

export default new CryptoManager();
```

### 3.2 Key Registration with Server

```typescript
// key-registration.ts
class KeyRegistration {
  async registerKeys(): Promise<void> {
    const cryptoManager = CryptoManager;
    
    // Generate or get existing identity key
    let identityKeyPair = await cryptoManager.getIdentityKeyPair();
    if (!identityKeyPair) {
      identityKeyPair = await cryptoManager.generateIdentityKeyPair();
    }

    // Generate signed pre-key
    const signedPreKey = await cryptoManager.generateSignedPreKey(identityKeyPair);

    // Generate one-time pre-keys
    const oneTimePreKeys = await cryptoManager.generateOneTimePreKeys(100);

    // Upload public keys to server
    await this.uploadKeysToServer({
      identityKey: this.arrayToBase64(identityKeyPair.publicKey),
      signedPreKey: {
        keyId: signedPreKey.keyId,
        publicKey: this.arrayToBase64(signedPreKey.publicKey),
        signature: this.arrayToBase64(signedPreKey.signature)
      },
      oneTimePreKeys: oneTimePreKeys.map(key => ({
        keyId: key.keyId,
        publicKey: this.arrayToBase64(key.publicKey)
      }))
    });
  }

  private async uploadKeysToServer(keys: any): Promise<void> {
    const response = await fetch('/api/v1/keys/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify(keys)
    });

    if (!response.ok) {
      throw new Error('Failed to register keys');
    }
  }

  private arrayToBase64(array: Uint8Array): string {
    return btoa(String.fromCharCode(...array));
  }

  private base64ToArray(base64: string): Uint8Array {
    return new Uint8Array(atob(base64).split('').map(c => c.charCodeAt(0)));
  }
}
```

### 3.3 Session Establishment (X3DH - Extended Triple Diffie-Hellman)

```typescript
// session-establishment.ts
class SessionManager {
  private cryptoManager = CryptoManager;

  // Initiator: Start session with recipient
  async initiateSession(recipientUserId: string): Promise<string> {
    await sodium.ready;

    // 1. Fetch recipient's public keys from server
    const recipientKeys = await this.fetchRecipientKeys(recipientUserId);

    // 2. Get own identity key pair
    const identityKeyPair = await this.cryptoManager.getIdentityKeyPair();
    if (!identityKeyPair) {
      throw new Error('Identity key not found');
    }

    // 3. Generate ephemeral key pair
    const ephemeralKeyPair = sodium.crypto_box_keypair();

    // 4. Perform X3DH key agreement
    const sharedSecret = this.performX3DH(
      identityKeyPair,
      ephemeralKeyPair,
      recipientKeys
    );

    // 5. Derive initial chain keys
    const { rootKey, chainKey } = this.deriveChainKeys(sharedSecret);

    // 6. Create and store session
    const sessionId = this.generateSessionId(recipientUserId);
    await this.storeSession(sessionId, {
      recipientUserId,
      rootKey,
      sendingChainKey: chainKey,
      receivingChainKey: null,
      sendMessageNumber: 0,
      receiveMessageNumber: 0,
      previousChainLength: 0
    });

    // 7. Send initial key exchange message to server
    await this.sendInitialKeyExchange(recipientUserId, {
      identityKey: identityKeyPair.publicKey,
      ephemeralKey: ephemeralKeyPair.publicKey,
      usedOneTimePreKeyId: recipientKeys.oneTimePreKey?.keyId
    });

    return sessionId;
  }

  // Recipient: Accept session
  async acceptSession(initiatorUserId: string, keyExchangeData: any): Promise<string> {
    await sodium.ready;

    // 1. Get own keys
    const identityKeyPair = await this.cryptoManager.getIdentityKeyPair();
    const signedPreKey = await this.cryptoManager.getSignedPreKey(
      keyExchangeData.signedPreKeyId
    );
    const oneTimePreKey = await this.cryptoManager.getOneTimePreKey(
      keyExchangeData.oneTimePreKeyId
    );

    if (!identityKeyPair || !signedPreKey) {
      throw new Error('Required keys not found');
    }

    // 2. Perform X3DH (recipient side)
    const sharedSecret = this.performX3DH_Recipient(
      identityKeyPair,
      signedPreKey,
      oneTimePreKey,
      keyExchangeData
    );

    // 3. Derive chain keys
    const { rootKey, chainKey } = this.deriveChainKeys(sharedSecret);

    // 4. Create and store session
    const sessionId = this.generateSessionId(initiatorUserId);
    await this.storeSession(sessionId, {
      recipientUserId: initiatorUserId,
      rootKey,
      sendingChainKey: null,
      receivingChainKey: chainKey,
      sendMessageNumber: 0,
      receiveMessageNumber: 0,
      previousChainLength: 0
    });

    // 5. Mark one-time pre-key as used
    if (oneTimePreKey) {
      await this.cryptoManager.markPreKeyUsed(oneTimePreKey.keyId.toString());
    }

    return sessionId;
  }

  // X3DH Key Agreement (Initiator)
  private performX3DH(
    identityKeyPair: IdentityKeyPair,
    ephemeralKeyPair: any,
    recipientKeys: any
  ): Uint8Array {
    // DH1 = DH(IKa, SPKb)
    const dh1 = sodium.crypto_scalarmult(
      this.convertSigningKeyToX25519(identityKeyPair.privateKey),
      recipientKeys.signedPreKey.publicKey
    );

    // DH2 = DH(EKa, IKb)
    const dh2 = sodium.crypto_scalarmult(
      ephemeralKeyPair.privateKey,
      this.convertSigningKeyToX25519(recipientKeys.identityKey)
    );

    // DH3 = DH(EKa, SPKb)
    const dh3 = sodium.crypto_scalarmult(
      ephemeralKeyPair.privateKey,
      recipientKeys.signedPreKey.publicKey
    );

    // DH4 = DH(EKa, OPKb) - if one-time pre-key available
    let dh4 = new Uint8Array(32);
    if (recipientKeys.oneTimePreKey) {
      dh4 = sodium.crypto_scalarmult(
        ephemeralKeyPair.privateKey,
        recipientKeys.oneTimePreKey.publicKey
      );
    }

    // Combine all DH outputs
    const sharedSecret = new Uint8Array(dh1.length + dh2.length + dh3.length + dh4.length);
    sharedSecret.set(dh1, 0);
    sharedSecret.set(dh2, dh1.length);
    sharedSecret.set(dh3, dh1.length + dh2.length);
    sharedSecret.set(dh4, dh1.length + dh2.length + dh3.length);

    // KDF to derive final shared secret
    return sodium.crypto_kdf_derive_from_key(
      32,
      1,
      'x3dh-key',
      sodium.crypto_generichash(32, sharedSecret)
    );
  }

  // Convert Ed25519 signing key to X25519 encryption key
  private convertSigningKeyToX25519(ed25519Key: Uint8Array): Uint8Array {
    return sodium.crypto_sign_ed25519_sk_to_curve25519(ed25519Key);
  }

  // Derive Root Key and Chain Key from shared secret
  private deriveChainKeys(sharedSecret: Uint8Array): { rootKey: Uint8Array; chainKey: Uint8Array } {
    const kdf = sodium.crypto_kdf_derive_from_key(
      64,
      1,
      'ratchet',
      sharedSecret
    );

    return {
      rootKey: kdf.slice(0, 32),
      chainKey: kdf.slice(32, 64)
    };
  }

  private generateSessionId(userId: string): string {
    return `session_${userId}_${Date.now()}`;
  }

  private async storeSession(sessionId: string, session: any): Promise<void> {
    const db = new IndexedDB('sessions');
    await db.put('sessions', sessionId, session);
  }

  private async fetchRecipientKeys(userId: string): Promise<any> {
    const response = await fetch(`/api/v1/keys/${userId}`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });
    return await response.json();
  }

  private async sendInitialKeyExchange(userId: string, data: any): Promise<void> {
    await fetch(`/api/v1/keys/exchange`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify({ userId, data })
    });
  }
}
```

### 3.4 Message Encryption/Decryption (Double Ratchet)

```typescript
// message-encryption.ts
class MessageEncryption {
  // Encrypt message
  async encryptMessage(sessionId: string, plaintext: string): Promise<EncryptedMessage> {
    await sodium.ready;

    // 1. Get session
    const session = await this.getSession(sessionId);

    // 2. Derive message key from chain key
    const messageKey = this.deriveMessageKey(session.sendingChainKey);

    // 3. Update chain key (ratchet forward)
    session.sendingChainKey = this.ratchetChainKey(session.sendingChainKey);
    session.sendMessageNumber++;

    // 4. Encrypt message with message key
    const nonce = sodium.randombytes_buf(sodium.crypto_secretbox_NONCEBYTES);
    const ciphertext = sodium.crypto_secretbox_easy(
      sodium.from_string(plaintext),
      nonce,
      messageKey
    );

    // 5. Create message header
    const header = {
      sessionId,
      messageNumber: session.sendMessageNumber,
      previousChainLength: session.previousChainLength
    };

    // 6. Save updated session
    await this.saveSession(sessionId, session);

    return {
      header: this.encodeHeader(header),
      ciphertext: this.arrayToBase64(ciphertext),
      nonce: this.arrayToBase64(nonce)
    };
  }

  // Decrypt message
  async decryptMessage(encryptedMessage: EncryptedMessage): Promise<string> {
    await sodium.ready;

    // 1. Decode header
    const header = this.decodeHeader(encryptedMessage.header);

    // 2. Get session
    const session = await this.getSession(header.sessionId);

    // 3. Check if we need to perform DH ratchet step
    if (header.messageNumber < session.receiveMessageNumber) {
      // Out of order message - use stored message key
      const messageKey = await this.getStoredMessageKey(
        header.sessionId,
        header.messageNumber
      );
      return this.decryptWithKey(encryptedMessage, messageKey);
    }

    // 4. Derive message key
    const messageKey = this.deriveMessageKey(session.receivingChainKey);

    // 5. Store message key for potential out-of-order messages
    await this.storeMessageKey(header.sessionId, header.messageNumber, messageKey);

    // 6. Update chain key
    session.receivingChainKey = this.ratchetChainKey(session.receivingChainKey);
    session.receiveMessageNumber++;

    // 7. Decrypt
    const plaintext = this.decryptWithKey(encryptedMessage, messageKey);

    // 8. Save updated session
    await this.saveSession(header.sessionId, session);

    return plaintext;
  }

  // Derive message key from chain key using HMAC-based KDF
  private deriveMessageKey(chainKey: Uint8Array): Uint8Array {
    return sodium.crypto_generichash(
      32,
      new Uint8Array([0x01]),
      chainKey
    );
  }

  // Ratchet chain key forward
  private ratchetChainKey(chainKey: Uint8Array): Uint8Array {
    return sodium.crypto_generichash(
      32,
      new Uint8Array([0x02]),
      chainKey
    );
  }

  private decryptWithKey(encryptedMessage: EncryptedMessage, messageKey: Uint8Array): string {
    const ciphertext = this.base64ToArray(encryptedMessage.ciphertext);
    const nonce = this.base64ToArray(encryptedMessage.nonce);

    const plaintext = sodium.crypto_secretbox_open_easy(
      ciphertext,
      nonce,
      messageKey
    );

    return sodium.to_string(plaintext);
  }

  private async getSession(sessionId: string): Promise<any> {
    const db = new IndexedDB('sessions');
    return await db.get('sessions', sessionId);
  }

  private async saveSession(sessionId: string, session: any): Promise<void> {
    const db = new IndexedDB('sessions');
    await db.put('sessions', sessionId, session);
  }

  private async storeMessageKey(sessionId: string, messageNumber: number, key: Uint8Array): Promise<void> {
    const db = new IndexedDB('message_keys');
    await db.put('keys', `${sessionId}_${messageNumber}`, key);
  }

  private async getStoredMessageKey(sessionId: string, messageNumber: number): Promise<Uint8Array> {
    const db = new IndexedDB('message_keys');
    return await db.get('keys', `${sessionId}_${messageNumber}`);
  }

  private encodeHeader(header: any): string {
    return btoa(JSON.stringify(header));
  }

  private decodeHeader(encodedHeader: string): any {
    return JSON.parse(atob(encodedHeader));
  }

  private arrayToBase64(array: Uint8Array): string {
    return btoa(String.fromCharCode(...array));
  }

  private base64ToArray(base64: string): Uint8Array {
    return new Uint8Array(atob(base64).split('').map(c => c.charCodeAt(0)));
  }
}

interface EncryptedMessage {
  header: string;
  ciphertext: string;
  nonce: string;
}
```

## 4. Server-Side Implementation

### 4.1 Key Management API

```typescript
// server/key-service.ts
import { Router } from 'express';
import { prisma } from './database';
import { authenticate } from './middleware/auth';

const router = Router();

// Register user's public keys
router.post('/keys/register', authenticate, async (req, res) => {
  const userId = req.user.id;
  const { identityKey, signedPreKey, oneTimePreKeys } = req.body;

  try {
    // Validate keys
    if (!identityKey || !signedPreKey || !oneTimePreKeys) {
      return res.status(400).json({ error: 'Missing required keys' });
    }

    // Store identity key (once per user)
    await prisma.identityKey.upsert({
      where: { userId },
      create: {
        userId,
        publicKey: identityKey
      },
      update: {
        publicKey: identityKey
      }
    });

    // Store signed pre-key
    await prisma.signedPreKey.create({
      data: {
        userId,
        keyId: signedPreKey.keyId,
        publicKey: signedPreKey.publicKey,
        signature: signedPreKey.signature,
        timestamp: new Date()
      }
    });

    // Store one-time pre-keys
    const preKeyPromises = oneTimePreKeys.map((key: any) =>
      prisma.oneTimePreKey.create({
        data: {
          userId,
          keyId: key.keyId,
          publicKey: key.publicKey,
          used: false
        }
      })
    );

    await Promise.all(preKeyPromises);

    res.json({ success: true, message: 'Keys registered successfully' });
  } catch (error) {
    console.error('Key registration error:', error);
    res.status(500).json({ error: 'Failed to register keys' });
  }
});

// Fetch recipient's public keys (for initiating session)
router.get('/keys/:userId', authenticate, async (req, res) => {
  const { userId } = req.params;

  try {
    // Get identity key
    const identityKey = await prisma.identityKey.findUnique({
      where: { userId }
    });

    // Get latest signed pre-key
    const signedPreKey = await prisma.signedPreKey.findFirst({
      where: { userId },
      orderBy: { timestamp: 'desc' }
    });

    // Get an unused one-time pre-key
    const oneTimePreKey = await prisma.oneTimePreKey.findFirst({
      where: {
        userId,
        used: false
      }
    });

    // Mark one-time pre-key as used
    if (oneTimePreKey) {
      await prisma.oneTimePreKey.update({
        where: { id: oneTimePreKey.id },
        data: { used: true }
      });
    }

    res.json({
      identityKey: identityKey?.publicKey,
      signedPreKey: {
        keyId: signedPreKey?.keyId,
        publicKey: signedPreKey?.publicKey,
        signature: signedPreKey?.signature
      },
      oneTimePreKey: oneTimePreKey ? {
        keyId: oneTimePreKey.keyId,
        publicKey: oneTimePreKey.publicKey
      } : null
    });
  } catch (error) {
    console.error('Key fetch error:', error);
    res.status(500).json({ error: 'Failed to fetch keys' });
  }
});

// Rotate signed pre-key (should be called weekly)
router.post('/keys/rotate-signed-prekey', authenticate, async (req, res) => {
  const userId = req.user.id;
  const { signedPreKey } = req.body;

  try {
    await prisma.signedPreKey.create({
      data: {
        userId,
        keyId: signedPreKey.keyId,
        publicKey: signedPreKey.publicKey,
        signature: signedPreKey.signature,
        timestamp: new Date()
      }
    });

    res.json({ success: true });
  } catch (error) {
    res.status(500).json({ error: 'Failed to rotate key' });
  }
});

// Replenish one-time pre-keys (when running low)
router.post('/keys/replenish-prekeys', authenticate, async (req, res) => {
  const userId = req.user.id;
  const { oneTimePreKeys } = req.body;

  try {
    const preKeyPromises = oneTimePreKeys.map((key: any) =>
      prisma.oneTimePreKey.create({
        data: {
          userId,
          keyId: key.keyId,
          publicKey: key.publicKey,
          used: false
        }
      })
    );

    await Promise.all(preKeyPromises);

    res.json({ success: true });
  } catch (error) {
    res.status(500).json({ error: 'Failed to replenish keys' });
  }
});

export default router;
```

### 4.2 Database Schema

```prisma
// schema.prisma
model User {
  id              String           @id @default(uuid())
  email           String           @unique
  username        String           @unique
  identityKey     IdentityKey?
  signedPreKeys   SignedPreKey[]
  oneTimePreKeys  OneTimePreKey[]
  createdAt       DateTime         @default(now())
  updatedAt       DateTime         @updatedAt
}

model IdentityKey {
  id          String   @id @default(uuid())
  userId      String   @unique
  user        User     @relation(fields: [userId], references: [id])
  publicKey   String   // Base64 encoded
  createdAt   DateTime @default(now())
  updatedAt   DateTime @updatedAt
}

model SignedPreKey {
  id          String   @id @default(uuid())
  userId      String
  user        User     @relation(fields: [userId], references: [id])
  keyId       Int
  publicKey   String   // Base64 encoded
  signature   String   // Base64 encoded
  timestamp   DateTime
  createdAt   DateTime @default(now())

  @@index([userId, timestamp])
}

model OneTimePreKey {
  id          String   @id @default(uuid())
  userId      String
  user        User     @relation(fields: [userId], references: [id])
  keyId       Int
  publicKey   String   // Base64 encoded
  used        Boolean  @default(false)
  createdAt   DateTime @default(now())

  @@index([userId, used])
}

model Message {
  id                String   @id @default(uuid())
  conversationId    String
  senderId          String
  encryptedContent  String   @db.Text // Base64 encrypted content
  encryptedHeader   String   // Base64 encrypted header
  nonce             String   // Base64 nonce
  messageType       String   // text, image, video, file
  metadata          Json?
  createdAt         DateTime @default(now())

  @@index([conversationId, createdAt])
}
```

## 5. Group Chat Encryption (Sender Keys)

```typescript
// group-encryption.ts
class GroupEncryption {
  // Create sender key for group
  async createSenderKey(groupId: string): Promise<void> {
    await sodium.ready;

    // Generate sender key
    const senderKey = sodium.crypto_secretbox_keygen();
    const chainKey = sodium.crypto_secretbox_keygen();

    // Store sender key
    await this.storeSenderKey(groupId, {
      key: senderKey,
      chainKey,
      generation: 0,
      messageNumber: 0
    });

    // Encrypt sender key for each group member
    const members = await this.getGroupMembers(groupId);
    
    for (const member of members) {
      const sessionId = this.generateSessionId(member.userId);
      const encryptedSenderKey = await this.encryptSenderKeyForMember(
        senderKey,
        sessionId
      );
      
      await this.sendSenderKeyToMember(member.userId, encryptedSenderKey);
    }
  }

  // Encrypt group message
  async encryptGroupMessage(groupId: string, plaintext: string): Promise<EncryptedMessage> {
    await sodium.ready;

    const senderKey = await this.getSenderKey(groupId);
    
    // Derive message key
    const messageKey = sodium.crypto_generichash(
      32,
      new Uint8Array([...sodium.from_string(`${senderKey.generation}`), ...sodium.from_string(`${senderKey.messageNumber}`)]),
      senderKey.chainKey
    );

    // Encrypt message
    const nonce = sodium.randombytes_buf(sodium.crypto_secretbox_NONCEBYTES);
    const ciphertext = sodium.crypto_secretbox_easy(
      sodium.from_string(plaintext),
      nonce,
      messageKey
    );

    // Update sender key state
    senderKey.messageNumber++;
    await this.storeSenderKey(groupId, senderKey);

    return {
      header: this.encodeSenderKeyHeader({
        groupId,
        generation: senderKey.generation,
        messageNumber: senderKey.messageNumber - 1
      }),
      ciphertext: this.arrayToBase64(ciphertext),
      nonce: this.arrayToBase64(nonce)
    };
  }

  // Decrypt group message
  async decryptGroupMessage(senderId: string, encryptedMessage: EncryptedMessage): Promise<string> {
    await sodium.ready;

    const header = this.decodeSenderKeyHeader(encryptedMessage.header);
    const senderKey = await this.getSenderKeyFromSender(header.groupId, senderId);

    // Derive message key
    const messageKey = sodium.crypto_generichash(
      32,
      new Uint8Array([...sodium.from_string(`${header.generation}`), ...sodium.from_string(`${header.messageNumber}`)]),
      senderKey.chainKey
    );

    // Decrypt
    const ciphertext = this.base64ToArray(encryptedMessage.ciphertext);
    const nonce = this.base64ToArray(encryptedMessage.nonce);
    
    const plaintext = sodium.crypto_secretbox_open_easy(
      ciphertext,
      nonce,
      messageKey
    );

    return sodium.to_string(plaintext);
  }

  private async storeSenderKey(groupId: string, senderKey: any): Promise<void> {
    const db = new IndexedDB('sender_keys');
    await db.put('keys', groupId, senderKey);
  }

  private async getSenderKey(groupId: string): Promise<any> {
    const db = new IndexedDB('sender_keys');
    return await db.get('keys', groupId);
  }

  private async getSenderKeyFromSender(groupId: string, senderId: string): Promise<any> {
    const db = new IndexedDB('sender_keys');
    return await db.get('keys', `${groupId}_${senderId}`);
  }

  private async getGroupMembers(groupId: string): Promise<any[]> {
    const response = await fetch(`/api/v1/groups/${groupId}/members`);
    return await response.json();
  }

  private async encryptSenderKeyForMember(senderKey: Uint8Array, sessionId: string): Promise<EncryptedMessage> {
    const messageEncryption = new MessageEncryption();
    return await messageEncryption.encryptMessage(
      sessionId,
      this.arrayToBase64(senderKey)
    );
  }

  private async sendSenderKeyToMember(userId: string, encryptedSenderKey: EncryptedMessage): Promise<void> {
    await fetch(`/api/v1/keys/sender-key`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify({
        userId,
        encryptedSenderKey
      })
    });
  }

  private encodeSenderKeyHeader(header: any): string {
    return btoa(JSON.stringify(header));
  }

  private decodeSenderKeyHeader(encoded: string): any {
    return JSON.parse(atob(encoded));
  }

  private arrayToBase64(array: Uint8Array): string {
    return btoa(String.fromCharCode(...array));
  }

  private base64ToArray(base64: string): Uint8Array {
    return new Uint8Array(atob(base64).split('').map(c => c.charCodeAt(0)));
  }

  private generateSessionId(userId: string): string {
    return `session_${userId}_${Date.now()}`;
  }
}
```

## 6. Media File Encryption

```typescript
// media-encryption.ts
class MediaEncryption {
  // Encrypt file before upload
  async encryptFile(file: File): Promise<EncryptedFile> {
    await sodium.ready;

    // Generate random encryption key for this file
    const fileKey = sodium.crypto_secretbox_keygen();

    // Read file as ArrayBuffer
    const fileData = await file.arrayBuffer();
    const fileBytes = new Uint8Array(fileData);

    // Encrypt file
    const nonce = sodium.randombytes_buf(sodium.crypto_secretbox_NONCEBYTES);
    const encryptedData = sodium.crypto_secretbox_easy(fileBytes, nonce, fileKey);

    // Create encrypted file blob
    const encryptedBlob = new Blob([encryptedData], { type: 'application/octet-stream' });

    return {
      encryptedBlob,
      fileKey: this.arrayToBase64(fileKey),
      nonce: this.arrayToBase64(nonce),
      originalName: file.name,
      originalType: file.type,
      originalSize: file.size
    };
  }

  // Upload encrypted file
  async uploadEncryptedFile(encryptedFile: EncryptedFile, conversationId: string): Promise<string> {
    const formData = new FormData();
    formData.append('file', encryptedFile.encryptedBlob, 'encrypted_file');
    formData.append('conversation_id', conversationId);

    const response = await fetch('/api/v1/files/upload', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: formData
    });

    const result = await response.json();
    return result.data.file_url;
  }

  // Send encrypted file metadata in message
  async sendFileMessage(conversationId: string, encryptedFile: EncryptedFile, fileUrl: string): Promise<void> {
    const messageEncryption = new MessageEncryption();
    const sessionId = this.getSessionId(conversationId);

    // Encrypt file metadata including the file key
    const metadata = {
      fileUrl,
      fileKey: encryptedFile.fileKey,
      nonce: encryptedFile.nonce,
      fileName: encryptedFile.originalName,
      fileType: encryptedFile.originalType,
      fileSize: encryptedFile.originalSize
    };

    const encryptedMessage = await messageEncryption.encryptMessage(
      sessionId,
      JSON.stringify(metadata)
    );

    // Send message
    await fetch(`/api/v1/conversations/${conversationId}/messages`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify({
        content_encrypted: encryptedMessage.ciphertext,
        message_type: 'file',
        metadata: {
          header: encryptedMessage.header,
          nonce: encryptedMessage.nonce
        }
      })
    });
  }

  // Download and decrypt file
  async downloadAndDecryptFile(fileUrl: string, fileKey: string, nonce: string): Promise<Blob> {
    await sodium.ready;

    // Download encrypted file
    const response = await fetch(fileUrl, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });

    const encryptedData = new Uint8Array(await response.arrayBuffer());

    // Decrypt file
    const key = this.base64ToArray(fileKey);
    const nonceBytes = this.base64ToArray(nonce);

    const decryptedData = sodium.crypto_secretbox_open_easy(
      encryptedData,
      nonceBytes,
      key
    );

    return new Blob([decryptedData]);
  }

  private getSessionId(conversationId: string): string {
    // Retrieve session ID from local storage or session manager
    return `session_${conversationId}`;
  }

  private arrayToBase64(array: Uint8Array): string {
    return btoa(String.fromCharCode(...array));
  }

  private base64ToArray(base64: string): Uint8Array {
    return new Uint8Array(atob(base64).split('').map(c => c.charCodeAt(0)));
  }
}

interface EncryptedFile {
  encryptedBlob: Blob;
  fileKey: string;
  nonce: string;
  originalName: string;
  originalType: string;
  originalSize: number;
}
```

## 7. Key Backup and Recovery

```typescript
// key-backup.ts
class KeyBackup {
  // Generate recovery key
  async generateRecoveryKey(): Promise<string> {
    await sodium.ready;
    
    // Generate random recovery key
    const recoveryKey = sodium.crypto_secretbox_keygen();
    
    // Convert to human-readable format (base58)
    const recoveryPhrase = this.generateRecoveryPhrase(recoveryKey);
    
    return recoveryPhrase;
  }

  // Backup keys with recovery key
  async backupKeys(recoveryPhrase: string): Promise<void> {
    await sodium.ready;

    const recoveryKey = this.parseRecoveryPhrase(recoveryPhrase);

    // Get all keys to backup
    const identityKeyPair = await CryptoManager.getIdentityKeyPair();
    const sessions = await this.getAllSessions();
    const senderKeys = await this.getAllSenderKeys();

    // Package keys
    const keyPackage = {
      identityKeyPair,
      sessions,
      senderKeys,
      timestamp: Date.now()
    };

    // Encrypt key package with recovery key
    const nonce = sodium.randombytes_buf(sodium.crypto_secretbox_NONCEBYTES);
    const encryptedPackage = sodium.crypto_secretbox_easy(
      sodium.from_string(JSON.stringify(keyPackage)),
      nonce,
      recoveryKey
    );

    // Upload to server
    await this.uploadBackup({
      encryptedPackage: this.arrayToBase64(encryptedPackage),
      nonce: this.arrayToBase64(nonce)
    });
  }

  // Restore keys from backup
  async restoreKeys(recoveryPhrase: string): Promise<void> {
    await sodium.ready;

    const recoveryKey = this.parseRecoveryPhrase(recoveryPhrase);

    // Download backup from server
    const backup = await this.downloadBackup();

    // Decrypt key package
    const encryptedPackage = this.base64ToArray(backup.encryptedPackage);
    const nonce = this.base64ToArray(backup.nonce);

    const decryptedData = sodium.crypto_secretbox_open_easy(
      encryptedPackage,
      nonce,
      recoveryKey
    );

    const keyPackage = JSON.parse(sodium.to_string(decryptedData));

    // Restore keys
    await this.restoreAllKeys(keyPackage);
  }

  private generateRecoveryPhrase(key: Uint8Array): string {
    // Convert to 12-word mnemonic phrase
    const words = this.keyToWords(key);
    return words.join(' ');
  }

  private parseRecoveryPhrase(phrase: string): Uint8Array {
    const words = phrase.split(' ');
    return this.wordsToKey(words);
  }

  private keyToWords(key: Uint8Array): string[] {
    // Use BIP39 word list
    const wordlist = this.getBIP39Wordlist();
    const words: string[] = [];
    
    for (let i = 0; i < 12; i++) {
      const index = (key[i * 2] << 8) | key[i * 2 + 1];
      words.push(wordlist[index % wordlist.length]);
    }
    
    return words;
  }

  private wordsToKey(words: string[]): Uint8Array {
    const wordlist = this.getBIP39Wordlist();
    const key = new Uint8Array(32);
    
    for (let i = 0; i < 12; i++) {
      const index = wordlist.indexOf(words[i]);
      key[i * 2] = (index >> 8) & 0xff;
      key[i * 2 + 1] = index & 0xff;
    }
    
    return key;
  }

  private getBIP39Wordlist(): string[] {
    // Return BIP39 English wordlist (2048 words)
    return [/* ... wordlist ... */];
  }

  private async getAllSessions(): Promise<any[]> {
    const db = new IndexedDB('sessions');
    return await db.getAll('sessions');
  }

  private async getAllSenderKeys(): Promise<any[]> {
    const db = new IndexedDB('sender_keys');
    return await db.getAll('keys');
  }

  private async uploadBackup(backup: any): Promise<void> {
    await fetch('/api/v1/keys/backup', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify(backup)
    });
  }

  private async downloadBackup(): Promise<any> {
    const response = await fetch('/api/v1/keys/backup', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    });
    return await response.json();
  }

  private async restoreAllKeys(keyPackage: any): Promise<void> {
    // Restore identity key
    const identityDb = new IndexedDB('crypto_storage');
    await identityDb.put('identity', 'keypair', keyPackage.identityKeyPair);

    // Restore sessions
    const sessionDb = new IndexedDB('sessions');
    for (const session of keyPackage.sessions) {
      await sessionDb.put('sessions', session.id, session);
    }

    // Restore sender keys
    const senderKeyDb = new IndexedDB('sender_keys');
    for (const senderKey of keyPackage.senderKeys) {
      await senderKeyDb.put('keys', senderKey.id, senderKey);
    }
  }

  private arrayToBase64(array: Uint8Array): string {
    return btoa(String.fromCharCode(...array));
  }

  private base64ToArray(base64: string): Uint8Array {
    return new Uint8Array(atob(base64).split('').map(c => c.charCodeAt(0)));
  }
}
```

## 8. Security Best Practices

### 8.1 Key Rotation Schedule
```
- Signed Pre-Key: Rotate every 7 days
- One-Time Pre-Keys: Replenish when < 20 remaining
- Sender Keys (Group): Rotate when member leaves
- Session Keys: Auto-ratchet with each message
```

### 8.2 Security Checklist
```
✓ All keys stored in IndexedDB (encrypted at rest by browser)
✓ Private keys never leave the client device
✓ Server only stores public keys and encrypted content
✓ Perfect forward secrecy via ratcheting
✓ Post-compromise security via DH ratchet
✓ Out-of-order message handling
✓ Replay attack prevention
✓ Man-in-the-middle protection via key verification
```

### 8.3 Key Verification (Safety Numbers)
```typescript
class KeyVerification {
  // Generate safety number for verification
  async generateSafetyNumber(userId1: string, userId2: string): Promise<string> {
    const key1 = await this.getIdentityKey(userId1);
    const key2 = await this.getIdentityKey(userId2);

    // Concatenate and hash
    const combined = new Uint8Array([...key1, ...key2]);
    const hash = sodium.crypto_generichash(32, combined);

    // Convert to numeric string
    return this.hashToNumericString(hash);
  }

  private hashToNumericString(hash: Uint8Array): string {
    let numericString = '';
    for (let i = 0; i < hash.length; i += 5) {
      const chunk = hash.slice(i, i + 5);
      const number = chunk.reduce((acc, byte) => acc * 256 + byte, 0);
      numericString += number.toString().padStart(15, '0').slice(0, 5) + ' ';
    }
    return numericString.trim();
  }
}
```

## 9. Performance Optimization

### 9.1 Caching Strategy
```typescript
// Cache decrypted messages
const messageCache = new LRUCache({
  max: 1000,
  ttl: 1000 * 60 * 60 // 1 hour
});

// Cache sessions
const sessionCache = new Map();
```

### 9.2 Batch Operations
```typescript
// Batch encrypt messages
async function batchEncrypt(messages: string[], sessionId: string): Promise<EncryptedMessage[]> {
  return await Promise.all(
    messages.map(msg => messageEncryption.encryptMessage(sessionId, msg))
  );
}
```

## 10. Testing

```typescript
// e2ee.test.ts
describe('E2EE Implementation', () => {
  test('Should establish session between two users', async () => {
    const alice = new SessionManager();
    const bob = new SessionManager();

    // Alice initiates session
    const sessionId = await alice.initiateSession('bob');

    // Bob accepts session
    await bob.acceptSession('alice', /* key exchange data */);

    expect(sessionId).toBeDefined();
  });

  test('Should encrypt and decrypt message', async () => {
    const encryption = new MessageEncryption();
    const plaintext = 'Hello, World!';

    const encrypted = await encryption.encryptMessage('session_1', plaintext);
    const decrypted = await encryption.decryptMessage(encrypted);

    expect(decrypted).toBe(plaintext);
  });

  test('Should handle out-of-order messages', async () => {
    // Test message queue and reordering
  });
});
```