# Flutter E2EE Client Side Implementation Guide

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 15.1. Tổng quan

Trong kiến trúc Hybrid Security của SecureConnect, Client (Flutter) là nơi duy nhất nắm giữ **Private Keys**. Mọi quá trình mã hóa tin nhắn (plaintext -> ciphertext) và giải mã (ciphertext -> plaintext) đều diễn ra trên thiết bị của người dùng trước khi gửi đi hoặc sau khi nhận về.

### Trách nhiệm của Client E2EE
1.  **Key Management:** Tạo cặp khóa (Ed25519/X25519) và lưu trữ an toàn trong Keychain/Keystore.
2.  **Key Exchange:** Lấy Public Keys của đối tác từ Go Backend.
3.  **Encryption:** Mã hóa nội dung tin nhắn dựa trên khóa phiên (Session Key).
4.  **Decryption:** Giải mã tin nhắn nhận được từ Server.

---

## 15.2. Cài đặt Dependencies

Chúng tôi sử dụng thư viện `cryptography` - một thư viện mã hóa mạnh mẽ, dễ dùng và được tích hợp tốt với Flutter. Ngoài ra cần `flutter_secure_storage` để lưu khóa bí mật.

**File `pubspec.yaml`:**
```yaml
dependencies:
  flutter:
    sdk: flutter
  
  # Security
  cryptography: ^2.3.0               # Thuật toán mã hóa
  flutter_secure_storage: ^8.0.0     # Lưu khóa bảo mật
  convert: ^3.1.1                    # Chuyển đổi Base64/Hex
  
  # Utilities
  uuid: ^4.0.0
  json_annotation: ^4.8.0
```

---

## 15.3. Kiến trúc Crypto Service

Tạo một service độc lập để xử lý logic mã hóa, tách biệt khỏi UI và State Management (Riverpod).

**Cấu trúc File:**
```bash
lib/security/
├── crypto_service.dart       # Service chính mã hóa/giải mã
├── secure_storage_service.dart # Wrapper cho FlutterSecureStorage
└── models/
    ├── key_bundle.dart       # Model Public Keys từ Server
    └── encrypted_message.dart # Model tin nhắn đã mã hóa
```

---

## 15.4. Lưu trữ Khóa Bí mật (Secure Storage)

Sử dụng `flutter_secure_storage` để lưu Private Keys. Khóa này không bao giờ được đưa ra khỏi vùng bộ nhớ bảo mật của hệ điều hành (iOS Keychain / Android Keystore).

**File: `lib/security/secure_storage_service.dart`**

```dart
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SecureStorageService {
  final _storage = const FlutterSecureStorage(
    aOptions: AndroidOptions(
      encryptedSharedPreferences: true,
    ),
  );

  // Lưu Private Key (Identity)
  Future<void> savePrivateKey(String keyId, String privateKey) async {
    await _storage.write(key: keyId, value: privateKey);
  }

  // Lấy Private Key
  Future<String?> getPrivateKey(String keyId) async {
    return await _storage.read(key: keyId);
  }

  // Xóa khóa (Logout / Delete Account)
  Future<void> deleteAll() async {
    await _storage.deleteAll();
  }
}
```

---

## 15.5. Key Management & Generation (X25519)

Sử dụng chuẩn **X25519** để tạo cặp khóa dùng cho trao đổi khóa (Key Exchange) và **Ed25519** để ký (Signing). Để đơn giản hóa ví dụ, ta tập trung vào X25519 cho mã hóa nội dung.

**File: `lib/security/crypto_service.dart` (Phần Key Management)**

```dart
import 'dart:typed_data';
import 'package:cryptography/cryptography.dart';
import 'package:uuid/uuid.dart';

class CryptoService {
  final SecureStorageService _storage;
  final _ed25519 = Ed25519(); // Để ký ( Signing )
  final _x25519 = X25519();   // Để trao đổi khóa ( Key Exchange )

  CryptoService(this._storage);

  // 1. Tạo cặp khóa cho User (Chạy 1 lần khi cài app)
  Future<void> generateIdentityKeys() async {
    // Tạo Key Pair Signing (Ed25519) - Dùng để xác thực
    final kpSign = _ed25519.newKeyPair();
    
    // Tạo Key Pair Exchange (X25519) - Dùng để mã hóa tin nhắn
    final kpExchange = _x25519.newKeyPair();

    // Lưu Private Keys vào Secure Storage (Hex String)
    await _storage.savePrivateKey('private_sign', kpSign.privateKey.export());
    await _storage.savePrivateKey('private_exchange', kpExchange.privateKey.export());

    // Trả về Public Keys để gửi lên Server
    return {
      'identity_key': kpSign.publicKey.export(),   // Base64/Hex
      'exchange_key': kpExchange.publicKey.export(),
    };
  }

  // 2. Lấy Key Pair của bản thân từ bộ nhớ
  Future<KeyPair> getMyIdentityKeys() async {
    final privSignStr = await _storage.getPrivateKey('private_sign');
    final privExchStr = await _storage.getPrivateKey('private_exchange');
    
    if (privSignStr == null || privExchStr == null) {
      throw Exception('Keys not found. Please login again or generate keys.');
    }

    // Tạo lại KeyPair từ String
    // (Lưu ý: Cần helper để parse Hex/Bytes -> PrivateKey object tùy thư viện)
    // Ở đây giả lập đã có method parse
    final privSign = _ed25519.newKeyPairFromSeed(/* parse hex */);
    final privExch = _x25519.newKeyPairFromSeed(/* parse hex */);

    return { 'sign': privSign, 'exchange': privExch };
  }
}
```

---

## 15.6. Logic Mã hóa (Encryption)

Logic này sẽ được gọi trước khi gửi tin nhắn lên Server.

**Luồng:**
1.  Lấy Public Key (Exchange) của người nhận từ Server (`/keys/{user_id}`).
2.  Sử dụng thuật toán **ECDH (Elliptic Curve Diffie-Hellman)** để tính toán **Shared Secret** (Khoá bí mật chung).
3.  Sử dụng **Shared Secret** để mã hóa tin nhắn bằng thuật toán đối xứng (AES-GCM).

**File: `lib/security/crypto_service.dart` (Phần Encrypt)**

```dart
import 'package:cryptography/cryptography.dart';

// ... (kế tiếp từ trên)

  // Mã hóa tin nhắn
  Future<EncryptedMessage> encryptMessage(String plainText, String recipientPublicKey) async {
    
    // 1. Lấy Private Key của mình
    final myKeys = await getMyIdentityKeys();
    
    // 2. Tạo lại Public Key Object của người nhận từ String
    final recipientPubKey = _x25519.newKeyPairFromSeed(/* parse hex */).publicKey;

    // 3. Tính Shared Secret (Bí mật chung giữa tôi và người nhận)
    final sharedSecret = _x25519.sharedSecret(
        keyPair: myKeys['exchange'],
        remotePublicKey: recipientPubKey,
    );

    // 4. Tạo khóa phiên từ Shared Secret (Derive Key)
    // Thực tế sẽ dùng HKDF, ở đây ví dụ đơn giản dùng 32 bytes đầu
    final secretKey = sharedSecret.extractBytes().sublist(0, 32);

    // 5. Mã hóa tin nhắn bằng AES-GCM (Authenticated Encryption)
    final algorithm = AesGcm.with256bits();
    final secretKeyObj = SecretKey(secretKey);
    
    // Tạo SecretBox (Ciphertext)
    final secretBox = await algorithm.encrypt(plainText, secretKey: secretKeyObj);

    // 6. Chuyển đổi thành Base64 để gửi đi
    return EncryptedMessage(
        ciphertext: secretBox.cipherText.base64,
        nonce: secretBox.nonce.base64,
        mac: secretBox.mac.base64, // Message Authentication Code
    );
  }
```

---

## 15.7. Logic Giải mã (Decryption)

Logic này được chạy khi nhận được tin nhắn từ Server.

**File: `lib/security/crypto_service.dart` (Phần Decrypt)**

```dart
import 'package:cryptography/cryptography.dart';

// ... (kế tiếp)

  // Giải mã tin nhắn
  Future<String> decryptMessage(EncryptedMessage encryptedMsg, String senderPublicKey) async {
    
    // 1. Lấy Private Key của mình
    final myKeys = await getMyIdentityKeys();

    // 2. Tạo lại Public Key Object của người gửi từ String
    final senderPubKey = _x25519.newKeyPairFromSeed(/* parse hex */).publicKey;

    // 3. Tính lại Shared Secret (Giống như khi họ mã hóa)
    final sharedSecret = _x25519.sharedSecret(
        keyPair: myKeys['exchange'],
        remotePublicKey: senderPubKey,
    );
    final secretKey = sharedSecret.extractBytes().sublist(0, 32);
    final secretKeyObj = SecretKey(secretKey);

    // 4. Chuẩn bị SecretBox từ dữ liệu nhận được
    final algorithm = AesGcm.with256bits();
    final cipherText = SecretBox.fromBase64(
      encryptedMsg.nonce,
      encryptedMsg.cipherText,
      mac: Mac.fromBase64(encryptedMsg.mac),
    );

    // 5. Giải mã
    try {
      final plainText = await algorithm.decrypt(cipherText, secretKey: secretKeyObj);
      return plainText;
    } catch (e) {
      throw Exception('Decryption failed. Wrong key or corrupted message.');
    }
  }
```

---

## 15.8. Mô hình Dữ liệu (Models)

**File: `lib/security/models/encrypted_message.dart`**

```dart
class EncryptedMessage {
  final String ciphertext; // Nội dung đã mã hóa (Base64)
  final String nonce;     // Nonce/IV dùng để mã hóa (Base64)
  final String mac;       // Message Authentication Code (Base64)

  EncryptedMessage({
    required this.ciphertext,
    required this.nonce,
    required this.mac,
  });
}
```

---

## 15.9. Tích hợp vào Riverpod (State Management)

Đóng gói `CryptoService` vào một Provider để dùng ở bất cứ đâu trong ứng dụng.

**File: `lib/security/crypto_provider.dart`**

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'crypto_service.dart';
import 'secure_storage_service.dart';

// Provider cho Storage
final secureStorageProvider = Provider<SecureStorageService>((ref) {
  return SecureStorageService();
});

// Provider cho Crypto Service
final cryptoServiceProvider = Provider<CryptoService>((ref) {
  final storage = ref.watch(secureStorageProvider);
  return CryptoService(storage);
});
```

---

## 15.10. Tích hợp vào UI (Chat Screen)

Ở màn hình Chat, khi người dùng bấm gửi:

```dart
class ChatDetailPage extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final crypto = ref.watch(cryptoServiceProvider);

    void _sendMessage(String text) async {
      String finalPayload = "";

      if (isEncryptedMode) {
        // 1. Lấy Public Key của người nhận (từ API hoặc cache)
        final recipientPubKey = "public_key_hex_string..."; 
        
        // 2. Gọi Crypto Service mã hóa
        final encryptedMsg = await crypto.encryptMessage(text, recipientPubKey);
        
        // 3. Gói vào JSON
        finalPayload = jsonEncode(encryptedMsg.toJson());
      } else {
        // Mode Opt-out: Gửi thẳng văn bản
        finalPayload = text;
      }

      // 4. Gửi lên Backend Go
      await api.sendMessage(
        conversationId: widget.convId,
        content: finalPayload,
        isEncrypted: isEncryptedMode,
      );
    }

    // ... UI Code
  }
}
```

**Khi nhận tin nhắn (WebSocket/REST Response):**

```dart
void _handleNewMessage(Message msg) async {
  String displayText = "";

  if (msg.isEncrypted) {
    // 1. Lấy Public Key của người gửi
    final senderPubKey = "...";

    // 2. Giải mã
    displayText = await crypto.decryptMessage(
      EncryptedMessage.fromRawJson(msg.content), // Parse ciphertext
      senderPubKey,
    );
  } else {
    // Hiển thị thẳng
    displayText = msg.content;
  }

  // Update UI
}
```

---

## 15.11. Lưu ý Bảo mật (Security Notes)

1.  **Luôn kiểm tra Key tồn tại:** Nếu Private Keys bị mất (thành viên xóa app cài lại), họ sẽ mất khả năng đọc tin nhắn cũ. Hãy hiện thông báo cảnh báo cho user về việc **Backup Keys** (cần một module Backup/Restore riêng).
2.  **Background Tasks:** Nếu dùng `Isolates` để mã hóa/giải mã (để không làm chậm UI), cẩn thận khi truyền dữ liệu bảo mật giữa Isolate và Main Thread.
3.  **Safety Numbers:** Để bảo vệ khỏi Man-in-the-Middle (MITM), hãy hiện **Safety Number** (hash fingerprint của Public Keys) để 2 user có thể so sánh offline (qua cuộc gọi video hoặc gặp mặt).
4.  **Không Debug Log:** Không bao giờ `print()` Private Keys hoặc Plaintext ra console trong production.

---

*Liên kết đến tài liệu tiếp theo:* `flutter/webrtc-ui-implementation.md`