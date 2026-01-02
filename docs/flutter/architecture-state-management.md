# Flutter Architecture & State Management

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 14.1. Tổng quan

Hệ thống Flutter là giao diện người dùng, chịu trách nhiệm hiển thị, tương tác, và xử lý một phần logic bảo mật (E2EE) và AI.

Để đảm bảo khả năng bảo trì với quy mô lớn, chúng tôi áp dụng:
1.  **Architecture:** **Clean Architecture** (Phân tách rõ ràng giữa UI, Business Logic, và Data Access).
2.  **State Management:** **Riverpod** v2.x (Thay thế Provider/GetX). Riverpod cung cấp tính Type Safety (kiểm tra lỗi lúc biên dịch), không phụ thuộc `BuildContext`, và dễ test.

---

## 14.2. Cấu trúc thư mục dự án (Project Structure)

Chúng tôi sử dụng cấu trúc **Feature-based** kết hợp Clean Architecture.

```bash
lib/
├── core/                      # Chia sẻ cho toàn bộ app
│   ├── constants/             # Colors, Fonts, API Endpoints
│   ├── theme/                 # Theme configuration
│   ├── utils/                 # Helpers (Date formatter, Size config)
│   ├── router/                # App Routes (GoRouter)
│   └── errors/                # Exception/Failure classes
│
├── data/                      # Lớp Dữ liệu (Data Layer)
│   ├── models/                # Response/Request DTOs (từ Go Backend)
│   │   ├── user_model.dart
│   │   ├── message_model.dart
│   │   └── ...
│   ├── repositories/          # Kết nối API & Local DB
│   │   ├── auth_repository.dart
│   │   ├── chat_repository.dart
│   │   └── chat_repository_impl.dart
│   └── services/              # API Clients (Dio, WebSocketChannel)
│       ├── api_client.dart
│       └── websocket_client.dart
│
├── domain/                    # Lớp Nghiệp vụ (Domain Layer) - Logic thuần
│   ├── entities/              # Entities nghiệp vụ (có thể khác Models)
│   ├── repositories/          # Interface của Repository (trừu tượng)
│   └── usecases/              # Logic nghiệp vụ (Ví dụ: GetMessages, Login)
│
└── presentation/              # Lớp Trình bày (UI & State)
    ├── providers/             # Riverpod State Notifiers
    │   ├── auth_provider.dart
    │   ├── chat_provider.dart
    │   └── call_provider.dart
    └── pages/                 # UI Screens
        ├── auth/
        │   ├── login_page.dart
        │   └── register_page.dart
        ├── chat/
        │   ├── conversation_list_page.dart
        │   └── chat_detail_page.dart
        └── call/
            └── video_call_page.dart
```

---

## 14.3. Cài đặt Dependencies

Thêm vào `pubspec.yaml`:

```yaml
dependencies:
  flutter_riverpod: ^2.4.0  # State Management
  riverpod_annotation: ^2.1.0 # Code generation
  go_router: ^12.0.0         # Navigation
  dio: ^5.0.0                # Networking
  hive: ^2.2.3               # Local Database
  flutter_secure_storage: ^8.0.0 # Secure storage for Keys
  web_socket_channel: ^2.4.0  # WebSocket
```

---

## 14.4. Chiến lược State Management với Riverpod

### 14.4.1. Auth State (Quản lý xác thực)

Sử dụng `StateNotifier` để quản lý trạng thái đăng nhập và JWT Token.

**File: `presentation/providers/auth_provider.dart`**

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/user_model.dart';
import '../../data/repositories/auth_repository.dart';

// Enum trạng thái
enum AuthStatus { initial, authenticated, unauthenticated, loading }

// State
class AuthState {
  final AuthStatus status;
  final User? user;
  final String? errorMessage;

  AuthState({
    required this.status,
    this.user,
    this.errorMessage,
  });

  // copyWith method để update state bất biến (immutable)
  AuthState copyWith({AuthStatus? status, User? user, String? errorMessage}) {
    return AuthState(
      status: status ?? this.status,
      user: user ?? this.user,
      errorMessage: errorMessage ?? this.errorMessage,
    );
  }
}

// Notifier (Controller)
class AuthNotifier extends StateNotifier<AuthState> {
  final AuthRepository _authRepository;

  AuthNotifier(this._authRepository) : super(AuthState(status: AuthStatus.initial));

  // Kiểm tra đăng nhập khi app mở
  Future<void> checkAuthStatus() async {
    final token = await _authRepository.getAccessToken();
    if (token != null) {
      // Validate token với backend
      final user = await _authRepository.getCurrentUser();
      if (user != null) {
        state = AuthState(status: AuthStatus.authenticated, user: user);
        return;
      }
    }
    state = AuthState(status: AuthStatus.unauthenticated);
  }

  // Đăng nhập
  Future<void> login(String email, String password) async {
    state = AuthState(status: AuthStatus.loading);
    try {
      final user = await _authRepository.login(email, password);
      state = AuthState(status: AuthStatus.authenticated, user: user);
    } catch (e) {
      state = AuthState(status: AuthStatus.unauthenticated, errorMessage: e.toString());
    }
  }

  // Đăng xuất
  Future<void> logout() async {
    await _authRepository.logout();
    state = AuthState(status: AuthStatus.unauthenticated);
  }
}

// Provider Declaration
final authRepositoryProvider = Provider<AuthRepository>((ref) => AuthRepositoryImpl());

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  final repo = ref.watch(authRepositoryProvider);
  return AuthNotifier(repo);
});
```

---

### 14.4.2. Chat State (Quản lý tin nhắn & Real-time)

Kết hợp giữa **StateNotifier** (để load trang, gửi tin) và **StreamProvider** (để nhận tin nhắn mới qua WebSocket).

**File: `presentation/providers/chat_provider.dart`**

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/message_model.dart';
import '../../data/repositories/chat_repository.dart';

class ChatState {
  final List<Message> messages;
  final bool isLoading;
  final String? errorMessage;
  final bool hasMore; // Hết tin nhắn chưa?

  ChatState({
    required this.messages,
    required this.isLoading,
    this.errorMessage,
    this.hasMore = true,
  });

  ChatState copyWith({List<Message>? messages, bool? isLoading, String? errorMessage, bool? hasMore}) {
    return ChatState(
      messages: messages ?? this.messages,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage ?? this.errorMessage,
      hasMore: hasMore ?? this.hasMore,
    );
  }
}

class ChatNotifier extends StateNotifier<ChatState> {
  final ChatRepository _chatRepository;
  final String _conversationId;

  ChatNotifier(this._chatRepository, this._conversationId)
      : super(ChatState(messages: [], isLoading: false));

  // Load tin nhắn (Pagination)
  Future<void> loadMessages({bool refresh = false}) async {
    if (refresh) state = ChatState(messages: [], isLoading: false);

    if (!state.hasMore || state.isLoading) return;

    state = state.copyWith(isLoading: true);

    try {
      final newMessages = await _chatRepository.getMessages(
        conversationId: _conversationId,
        limit: 50,
        cursor: state.messages.isEmpty ? null : state.messages.last.id, // Simplified cursor logic
      );

      state = state.copyWith(
        messages: refresh ? newMessages : [...state.messages, ...newMessages],
        isLoading: false,
        hasMore: newMessages.length >= 50, // Nếu trả về ít hơn 50 thì hết tin
      );
    } catch (e) {
      state = state.copyWith(isLoading: false, errorMessage: e.toString());
    }
  }

  // Gửi tin nhắn
  Future<void> sendMessage(String content, bool isEncrypted) async {
    // 1. Gọi API REST (để lưu vào DB)
    await _chatRepository.sendMessage(
      conversationId: _conversationId,
      content: content,
      isEncrypted: isEncrypted,
    );

    // 2. Tạo tin nhắn tạm (Optimistic UI)
    // (UI sẽ hiện ngay lập tức, sau đó cập nhật khi nhận WS confirm)
    // ...
  }
}

// Provider
final chatProvider =
    StateNotifierProvider.family<ChatNotifier, ChatState, String>((ref, conversationId) {
  final repo = ref.watch(chatRepositoryProvider);
  return ChatNotifier(repo, conversationId);
});

// Stream Provider cho WebSocket (Nhận tin nhắn mới thực-time)
final messageStreamProvider =
    StreamProvider.family.autoDispose<WebSocketMessage, String>((ref, conversationId) {
  final repo = ref.watch(chatRepositoryProvider);
  return repo.getMessageStream(conversationId); // Trả về Stream<Message>
});
```

---

### 14.4.3. Video Call State (Quản lý WebRTC)

State này cần quản lý trạng thái `RTCPeerConnection`, `LocalStream`, và `RemoteStream`.

**File: `presentation/providers/call_provider.dart`**

```dart
import 'dart:io';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

enum CallState { calling, connected, ended, error }

class CallNotifier extends StateNotifier<CallState> {
  final CallRepository _callRepository;
  
  // Các object WebRTC (thường lưu trong một class riêng WebRTCManager)
  // Ở đây ví dụ trạng thái đơn giản
  CallNotifier(this._callRepository) : super(CallState.calling);

  Future<void> joinCall(String callId) async {
    // Init WebRTC Logic
    // Connect to WebSocket Signaling
    // ...
  }

  Future<void> endCall() async {
    // Close Connections
    state = CallState.ended;
  }
}
```

---

## 14.5. Tích hợp Repository & Data Sources

Để State không phụ thuộc trực tiếp vào Network/UI, ta dùng **Repository Pattern**.

**File: `data/repositories/chat_repository_impl.dart`**

```dart
import 'package:dio/dio.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

class ChatRepositoryImpl implements ChatRepository {
  final Dio _dio;
  
  ChatRepositoryImpl(this._dio);

  @override
  Future<List<Message>> getMessages({required String convId, int? limit, String? cursor}) async {
    try {
      final response = await _dio.get('/messages', queryParameters: {
        'conversation_id': convId,
        'limit': limit ?? 50,
        'cursor': cursor
      });
      
      // Parse JSON -> List<Message>
      return (response.data['data']['messages'] as List)
          .map((e) => Message.fromJson(e))
          .toList();
    } catch (e) {
      throw Exception('Failed to load messages');
    }
  }

  @override
  Stream<WebSocketMessage> getMessageStream(String convId) {
    // Tạo kết nối WebSocket
    final channel = WebSocketChannel.connect(
      Uri.parse('wss://api.secureconnect.com/v1/ws/chat?token=...'),
    );
    
    // Gửi subscribe message
    channel.sink.add({'type': 'subscribe', 'conversation_id': convId});
    
    // Trả về Stream cho Consumer
    return channel.stream.map((event) => WebSocketMessage.fromJson(event));
  }
}
```

---

## 14.6. Chiến lược Xử lý Lỗi & Loading (UI Handling)

Trong Widget, sử dụng `ref.watch` để lắng nghe thay đổi state và vẽ UI tương ứng.

**File: `presentation/pages/chat_detail_page.dart`**

```dart
class ChatDetailPage extends ConsumerWidget {
  final String conversationId;

  const ChatDetailPage({super.key, required this.conversationId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // 1. Watch State (List messages)
    final chatState = ref.watch(chatProvider(conversationId));

    // 2. Watch Stream (New messages real-time)
    // ref.listen(messageStreamProvider(conversationId), (prev, next) {
    //   ref.read(chatProvider(conversationId).notifier).addNewMessage(next);
    // });

    return Scaffold(
      appBar: AppBar(title: Text('Chat')),
      body: Column(
        children: [
          // List Messages
          Expanded(
            child: chatState.isLoading
                ? Center(child: CircularProgressIndicator())
                : ListView.builder(
                    itemCount: chatState.messages.length,
                    itemBuilder: (context, index) {
                      final msg = chatState.messages[index];
                      return MessageBubble(
                        content: msg.content, // UI sẽ tự gọi Crypto Service để giải mã nếu cần
                        isEncrypted: msg.isEncrypted,
                      );
                    },
                  ),
          ),
          // Input Field
          MessageInputField(
            onSend: (text, isEncrypted) {
              ref.read(chatProvider(conversationId).notifier).sendMessage(text, isEncrypted);
            },
          ),
        ],
      ),
    );
  }
}
```

---

## 14.7. Định tuyến (Routing) với GoRouter

Tích hợp state management vào router để bảo vệ các trang cần đăng nhập (Protected Routes).

**File: `core/router/app_router.dart`**

```dart
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authProvider);
  
  return GoRouter(
    initialLocation: '/splash',
    refreshListenable: authState.notifier, // Listen auth changes to refresh route
    
    routes: [
      GoRoute(
        path: '/splash',
        builder: (context, state) => SplashPage(),
      ),
      GoRoute(
        path: '/login',
        builder: (context, state) => LoginPage(),
      ),
      GoRoute(
        path: '/home',
        redirect: (context, state) {
          // Nếu chưa đăng nhập -> Redirect về login
          if (authState.status != AuthStatus.authenticated) return '/login';
          return null;
        },
        builder: (context, state) => HomePage(),
      ),
      GoRoute(
        path: '/chat/:id',
        builder: (context, state) => ChatDetailPage(
          conversationId: state.pathParameters['id']!,
        ),
      ),
    ],
  );
});
```

---

## 14.8. Tối ưu hóa cho Hybrid E2EE

Trong UI, bạn cần hiển thị trạng thái `isEncrypted` rõ ràng để người dùng biết tin nhắn đã được AI xử lý hay chưa.

*   **Nếu `isEncrypted = true`:**
    *   Hiện icon **Lock** màu vàng.
    *   Không hiện AIMetadata (null).
    *   Không hiện Smart Replies (chỉ có input text).
*   **Nếu `isEncrypted = false`:**
    *   Hiện icon **Robot/Brain**.
    *   Hiện sentiment analysis (Emoji cảm xúc).
    *   Hiện nút **Smart Replies** (Gợi ý trả lời từ metadata).

---

## 14.9. Testing (Kiểm thử)

Riverpod cực kỳ dễ test vì không phụ thuộc vào BuildContext.

```dart
test('should load messages successfully', () async {
  // Setup Mock Repository
  final mockRepo = MockChatRepository();
  when(mockRepo.getMessages(conversationId: '123'))
      .thenAnswer((_) async => [Message(content: 'Hello')]);

  // Create Container
  final container = ProviderContainer(
    overrides: [
      chatRepositoryProvider.overrideWithValue(mockRepo),
    ],
  );

  // Act
  final notifier = container.read(chatProvider('123').notifier);
  await notifier.loadMessages();

  // Expect
  expect(container.read(chatProvider('123')).messages.length, 1);
});
```

---

*Liên kết đến tài liệu tiếp theo:* `flutter/e2ee-client-side-guide.md`