# QA Testing Strategy

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 21.1. Tổng quan

Mục tiêu của chiến lược kiểm thử này là áp dụng mô hình **Testing Pyramid**, nơi số lượng Unit Tests lớn nhất, Integration Tests ở giữa và E2E Tests ít nhất nhưng quan trọng nhất.

### Các tầng kiểm thử (Testing Levels)
1.  **Unit Tests:** Kiểm tra từng hàm/method riêng biệt (Tức thời, chi phí thấp).
2.  **Integration Tests:** Kiểm tra sự tương tác giữa các module (Service <-> DB, Service <-> Redis).
3.  **End-to-End (E2E) Tests:** Mô phỏng kịch bản người dùng thực (Login -> Chat -> Video Call).
4.  **Performance Tests:** Kiểm tra khả năng chịu tải (Load/Stress) của hệ thống.

---

## 21.2. Backend Testing Strategy (Go)

Go là ngôn ngữ rất phù hợp cho Testing nhờ thư viện `testing` chuẩn và tính hỗ trợ `table-driven tests`.

### 21.2.1. Unit Tests (Logic nghiệp vụ)

**Vị trí:** Bên cạnh file source (ví dụ: `crypto_service.go` -> `crypto_service_test.go`).

**Ví dụ: Test Mã hóa E2EE**

**File: `internal/crypto/crypto_service_test.go`**

```go
package crypto

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestGenerateKeyPair(t *testing.T) {
    // Test case: Tạo cặp khóa
    kp, err := GenerateIdentityKeyPair()
    
    require.NoError(t, err) // Dừng test ngay nếu có lỗi nghiêm trọng
    assert.NotNil(t, kp.PublicKey)
    assert.NotNil(t, kp.PrivateKey)
    assert.Len(t, kp.PublicKey, 32) // Ed25519 key phải là 32 bytes
}

func TestEncryptDecryptMessage(t *testing.T) {
    // Setup
    aliceKey := NewKeyPair()
    bobKey := NewKeyPair()
    
    plaintext := "Hello World"
    
    // Action: Alice mã hóa với Public Key của Bob
    ciphertext, err := EncryptMessage(plaintext, bobKey.PublicKey)
    require.NoError(t, err)
    
    // Assert: Ciphertext không được trùng plaintext
    assert.NotEqual(t, plaintext, ciphertext)
    
    // Action: Bob giải mã với Private Key của mình
    decrypted, err := DecryptMessage(ciphertext, aliceKey.PublicKey, bobKey.PrivateKey)
    require.NoError(t, err)
    
    // Verify: Kết quả phải giống ban đầu
    assert.Equal(t, plaintext, decrypted)
}
```

**Quy tắc:**
*   Sử dụng `require` cho lỗi nghiêm trọng (Setup, DB connection).
*   Sử dụng `assert` cho kiểm tra logic nghiệp vụ.
*   Đặt tên test theo format: `Test<FunctionName>_<Scenario>`.

---

### 21.2.2. Integration Tests (Database & External APIs)

Kiểm tra xem code Go có giao tiếp đúng với DB hay không. Sử dụng **Testcontainers** (nếu CI cho phép) hoặc **Mock** (nếu cần tốc độ).

**Công cụ:** `testify/suite`, `gock` (Mock HTTP), `mockery` (Mock Interface).

**Ví dụ: Test Repository (Cassandra)**

```go
func TestChatRepository_SaveMessage_Integration(t *testing.T) {
    // 1. Setup DB Test Container (Sử dụng docker-compose test hoặc Testcontainers)
    session := ConnectToTestCassandra()
    defer session.Close()

    repo := NewChatRepository(session)

    // 2. Test Data
    msg := &models.Message{
        MessageID: "msg-123",
        Content: "Test content",
        CreatedAt: time.Now(),
    }

    // 3. Action
    err := repo.SaveMessage(context.Background(), msg)

    // 4. Assert
    require.NoError(t, err)
    
    // 5. Verify bằng cách query lại
    var fetchedContent string
    err = session.Query(`SELECT content FROM messages WHERE message_id = ? LIMIT 1`, msg.MessageID).Scan(&fetchedContent)
    require.NoError(t, err)
    assert.Equal(t, "Test content", fetchedContent)
}
```

---

### 21.2.3. API Endpoint Tests (HTTP Handler)

Test REST API của Go.

**Ví dụ: Test API Login**

```go
func TestAuthHandler_Login(t *testing.T) {
    // 1. Setup Router & Mock DB
    router := gin.Default()
    mockRepo := new(MockAuthRepository)
    handler := NewAuthHandler(mockRepo)
    
    router.POST("/login", handler.Login)
    
    // 2. Tạo Fake Request Body
    body := `{"email": "test@example.com", "password": "wrongpass"}`
    req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    // 3. Record Response Writer
    w := httptest.NewRecorder()
    
    // 4. Execute
    router.ServeHTTP(w, req)
    
    // 5. Assert HTTP Status
    assert.Equal(t, 401, w.Code)
    
    // 6. Assert Response Body
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.False(t, response["success"].(bool))
}
```

---

## 21.3. Frontend Testing Strategy (Flutter)

Flutter có 3 loại test chính: Unit, Widget, và Integration.

### 21.3.1. Unit Tests (Logic thuần túy)

Test các hàm helper, crypto service, hoặc models.

**File: `test/services/crypto_service_test.dart`**

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:secureconnect/services/crypto_service.dart';

void main() {
  group('CryptoService', () {
    test('should encrypt and decrypt message successfully', () {
      final crypto = CryptoService();
      
      final plainText = "Hello SecureConnect";
      final key = "test-key-123"; // Dummy key for unit test
      
      // Encrypt
      final encrypted = crypto.encrypt(plainText, key);
      
      // Verify ciphertext is different from plaintext
      expect(encrypted, isNot(plainText));
      expect(encrypted, isNotEmpty);
      
      // Decrypt
      final decrypted = crypto.decrypt(encrypted, key);
      
      // Verify
      expect(decrypted, equals(plainText));
    });
    
    test('should throw exception if key is wrong', () {
      final crypto = CryptoService();
      final plainText = "Secret";
      final key = "key-a";
      final wrongKey = "key-b";
      
      final encrypted = crypto.encrypt(plainText, key);
      
      expect(
        () => crypto.decrypt(encrypted, wrongKey),
        throwsA(isA<DecryptionError>()),
      );
    });
  });
}
```

---

### 21.3.2. Widget Tests (UI Testing)

Kiểm tra giao diện người dùng có render đúng không, bấm nút có hoạt động không. Không gọi API thật (Mock Provider).

**File: `test/pages/login_page_test.dart`**

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:secureconnect/presentation/pages/login_page.dart';
import 'package:secureconnect/presentation/providers/auth_provider.dart';

void main() {
  testWidgets('Email field validation works', (WidgetTester tester) async {
    // 1. Tạo Mock Provider
    final container = ProviderContainer(
      overrides: [
        authProvider.overrideWith((ref) => MockAuthNotifier()),
      ],
    );

    // 2. Tạo Widget dưới ProviderScope
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(home: LoginPage()),
      ),
    );

    // 3. Tìm Input Field
    final emailField = find.byKey(Key('login_email_field'));
    
    // 4. Enter text
    await tester.enterText(emailField, 'invalid-email');
    await tester.pump(); // Rebuild widget

    // 5. Verify hiển thị lỗi (giả lập UI có logic validate)
    expect(find.text('Invalid email format'), findsOneWidget);
  });

  testWidgets('Tapping login button calls provider', (WidgetTester tester) async {
    // Setup Mock...
    
    // Pump Widget...
    
    final loginButton = find.text('Login');
    
    // Tap button
    await tester.tap(loginButton);
    await tester.pumpAndSettle(); // Chờ animation/Future xong

    // Verify function login trong Provider đã được gọi 1 lần
    verify(mockNotifier.login(any, any)).called(1);
  });
}
```

---

### 21.3.3. Integration Tests (Flutter)

Test toàn bộ luồng từ UI -> Repository -> Fake HTTP Client (hoặc Mock Server). **Không sử dụng Provider mock**, dùng Provider thật.

**File: `integration_test/app_test.dart`**

```dart
void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('Login flow works end-to-end', (WidgetTester tester) async {
    // 1. Tạo HTTP Mock Server (thư viện http_mock_adapter)
    final client = MockClient();
    client.post(Url('https://api.secureconnect.com/v1/auth/login'),
        body: '{"email":"a@b.c","password":"123"}')
        .reply(200, body: '{"success": true, "data": {"access_token": "..."} }');

    // 2. Inject Mock Client vào Repository
    // (Cần setup Dependency Injection)

    // 3. Run App
    app.main();
    await tester.pumpAndSettle();

    // 4. Interact
    await tester.enterText(find.byKey('email'), 'a@b.c');
    await tester.enterText(find.byKey('password'), '123');
    await tester.tap(find.text('Login'));
    await tester.pumpAndSettle();

    // 5. Verify navigated to Home Page
    expect(find.text('Welcome'), findsOneWidget);
  });
}
```

---

## 21.4. End-to-End (E2E) Testing

Mô phỏng kịch bản người dùng thực tế đi qua toàn bộ hệ thống.

### Công cụ
*   **Mobile (Native):** Detox (Gray box testing, rất nhanh).
*   **Flutter:** Flutter Driver (Đang bị thay thế bằng `integration_test`).

### Kịch bản E2E Quan trọng
1.  **Đăng ký & E2EE Handshake:**
    *   User A cài app -> Tạo keys -> Upload keys.
    *   User B cài app -> Lấy keys của A -> Tạo session -> Nhắn tin mã hóa -> B giải mã.
2.  **Video Call:**
    *   User A gọi -> User B nhận -> Signaling exchange -> Connected.

### Tự động hóa E2E trên CI
Sử dụng Firebase Test Lab hoặc AWS Device Farm để chạy test trên nhiều thiết bị Android/iOS thực.

---

## 21.5. Performance & Load Testing (Kiểm tra hiệu năng)

Hệ thống Chat/WebRTC chịu tải rất lớn, cần kiểm tra kỹ.

### 21.5.1. Công cụ: k6 (Go JS wrapper)
Sử dụng k6 để giả lập hàng nghìn user.

**File: `tests/load/chat_load_test.js`**

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// 1. Cấu hình tải
export let options = {
    stages: [
        { duration: '2m', target: 100 },  // Ramp up lên 100 users trong 2 phút
        { duration: '5m', target: 100 },  // Giữ 100 users trong 5 phút
        { duration: '2m', target: 2000 }, // Stress test: Tăng vọt lên 2000 users
        { duration: '5m', target: 0 },    // Taper down
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'], // 95% request phải < 500ms
        http_req_failed: ['rate<0.01'], // Tỷ lệ lỗi < 1%
    },
};

// 2. Login trước để lấy token
const BASE_URL = 'https://api.secureconnect.com';

export function setup() {
    let loginRes = http.post(`${BASE_URL}/v1/auth/login`, JSON.stringify({
        email: 'test@example.com',
        password: 'password'
    }), { headers: { 'Content-Type': 'application/json' }});
    
    return { token: loginRes.json('data.access_token') };
}

export default function(data) {
    // 3. Gửi tin nhắn liên tục
    let payload = JSON.stringify({
        conversation_id: "conv-123",
        content: "Hello Load Test",
        is_encrypted: false
    });

    let params = {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${data.token}`
        },
    };

    let res = http.post(`${BASE_URL}/v1/messages`, payload, params);
    
    check(res, {
        'status is 201': (r) => r.status == 201,
        'message sent': (r) => r.json('success') === true,
    });
    
    sleep(1); // Nghỉ 1 giây giữa các tin nhắn
}
```

### 21.5.2. WebRTC Stress Test
Test Signaling Server (WebSocket) chịu bao nhiêu kết nối.
*   Dùng script Go tạo hàng nghìn kết nối WebSocket giả lập (`goroutines`).
*   Giả lập trao đổi Offer/Answer.
*   Metric quan trọng: **Latency Signaling** và **Memory usage của SFU**.

---

## 21.6. Security Testing (Kiểm tra bảo mật)

Hệ thống E2EE và Payment đòi hỏi kiểm tra bảo mật nghiêm ngặt.

### 21.6.1. Static Application Security Testing (SAST)
Sử dụng **SonarQube** hoặc **Gosec** để scan code Go tìm lỗ hổng (SQL Injection, Hardcoded passwords).
*   Tích hợp vào GitHub Actions (Pipeline).

### 21.6.2. E2EE Validation Tests
Test thủ công hoặc tự động để đảm bảo Key Exchange đúng:
*   Lấy Public Key từ DB -> Modifying 1 byte -> Thử giải mã -> **Phải Fail**.
*   Lấy Private Key khác -> Thử giải mã -> **Phải Fail**.

### 21.6.3. Penetration Testing
Hàng năm hoặc trước mỗi bản release lớn:
*   Thuê dịch vụ Pentest bên thứ ba (ví dụ: PwC, KPMG).
*   Dùng **OWASP ZAP** để quét Web API tìm lỗi XSS, CSRF, Broken Authentication.

---

## 21.7. Test Coverage (Độ phủ)

Đặt mục tiêu Coverage để đảm bảo chất lượng code.

*   **Backend (Go):** **Min 80%** dòng code. Mục tiêu 90% cho các module E2EE và Payment.
*   **Frontend (Flutter):** **Min 70%** dòng code. Widget test thì khó đạt 100%, nhưng Logic phải cao.

### Công cụ tính Coverage
*   **Go:** `go test -coverprofile=coverage.out` -> Visualize bằng `go tool cover -html=coverage.out`.
*   **Flutter:** `flutter test --coverage`. Tạo file `lcov.info` và upload lên **Codecov** hoặc **Coveralls** để xem lịch sử.

---

*Liên kết đến tài liệu tiếp theo:* `maintenance/troubleshooting-guide.md`