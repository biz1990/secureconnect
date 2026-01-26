# Flutter WebRTC UI Implementation Guide

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 16.1. Tổng quan

Màn hình Video Call (Flutter) chịu trách nhiệm:
1.  Kết nối Media Device (Camera/Microphone).
2.  Tạo `RTCPeerConnection` để kết nối P2P hoặc SFU.
3.  Giao tiếp Signaling (SDP Offer/Answer, ICE Candidate) với Backend Go qua WebSocket.
4.  Hiển thị luồng video Local và Remote.
5.  Xử lý các điều khiển (Mute, Camera, Share Screen).

---

## 16.2. Cài đặt Dependencies

Thêm vào `pubspec.yaml`:

```yaml
dependencies:
  flutter:
    sdk: flutter

  # WebRTC Core
  flutter_webrtc: ^0.9.40
  permission_handler: ^11.0.0 # Xử lý quyền Camera/Mic trên Mobile

  # UI Utils
  flutter_bloc: ^8.1.3 # Hoặc flutter_riverpod (nếu dùng chung)
  flutter_screenutil: ^5.8.4 # Responsive UI
  lottie: ^2.6.0 # Animation cho trạng thái đang gọi
```

### Cấu hình quyền (Permissions)

**Android (`android/app/src/main/AndroidManifest.xml`):**
```xml
<uses-permission android:name="android.permission.INTERNET"/>
<uses-permission android:name="android.permission.CAMERA"/>
<uses-permission android:name="android.permission.RECORD_AUDIO"/>
<uses-permission android:name="android.permission.MODIFY_AUDIO_SETTINGS"/>
<uses-permission android:name="android.permission.WAKE_LOCK"/>
<uses-permission android:name="android.permission.BLUETOOTH"/>
```

**iOS (`ios/Runner/Info.plist`):**
```xml
<key>NSCameraUsageDescription</key>
<string>Ứng dụng cần quyền truy cập Camera để gọi Video</string>
<key>NSMicrophoneUsageDescription</key>
<string>Ứng dụng cần quyền truy cập Micro để nghe giọng nói</string>
```

---

## 16.3. Kiến trúc Thư mục (Project Structure)

```bash
lib/features/call/
├── presentation/
│   ├── pages/
│   │   └── video_call_page.dart       # Màn hình chính
│   ├── widgets/
│   │   ├── local_video_view.dart     # Video của mình
│   │   ├── remote_video_view.dart    # Video người khác
│   │   └── call_controls.dart        # Nút bấm điều khiển
│   └── providers/
│       └── call_state_provider.dart # State Management (Bloc/Riverpod)
├── domain/
│   └── entities/
│       └── call_state.dart
└── data/
    └── services/
        ├── webrtc_manager.dart       # Logic WebRTC Core
        └── signaling_client.dart     # WebSocket Client
```

---

## 16.4. WebRTC Manager (Logic Core)

Lớp này đóng gói logic phức tạp của `flutter_webrtc`, giúp UI sạch sẽ.

**File: `lib/features/call/data/services/webrtc_manager.dart`**

```dart
import 'package:flutter/foundation.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:uuid/uuid.dart';

class WebRTCManager {
  // Peer Connection
  RTCPeerConnection? _peerConnection;
  
  // Streams
  MediaStream? _localStream;
  List<MediaStream> _remoteStreams = [];
  
  // Renderers
  final _localRenderer = RTCVideoRenderer();
  final _remoteRenderer = RTCVideoRenderer();
  
  // Config
  final Map<String, dynamic> _iceServers;

  // Callbacks cho UI
  Function(RTCVideoRenderer)? onLocalStreamReady;
  Function(RTCVideoRenderer)? onRemoteStreamReady;
  Function(String)? onIceCandidate;
  Function(RTCSessionDescription)? onSignalingMessage; // Send SDP Offer/Answer

  WebRTCManager(this._iceServers);

  // 1. Khởi tạo Local Stream (Camera/Mic)
  Future<void> initLocalStream({bool audio = true, bool video = true}) async {
    final Map<String, dynamic> constraints = {
      'audio': audio,
      'video': video
        ? {
            'facingMode': 'user', // Camera trước
            'width': {'ideal': 1280},
            'height': {'ideal': 720}
          }
        : false
    };

    try {
      _localStream = await navigator.mediaDevices.getUserMedia(constraints);
      _localRenderer.srcObject = _localStream;
      
      // Notify UI
      onLocalStreamReady?.call(_localRenderer);
    } catch (e) {
      debugPrint('Error accessing media devices: $e');
      rethrow;
    }
  }

  // 2. Tạo Peer Connection
  Future<void> createPeerConnection() async {
    final configuration = RTCConfiguration(
      iceServers: [
        RTCIceServer(url: "stun:stun.l.google.com:19302"),
        ...(_iceServers['turn_servers'] ?? []).map((e) => RTCIceServer(
            url: e['url'],
            username: e['username'],
            credential: e['credential']
        ))
      ],
    );

    _peerConnection = await createPeerConnection(configuration);

    // Xử lý ICE Candidate
    _peerConnection!.onIceCandidate = (candidate) {
      if (candidate == null) return;
      // Gửi qua WebSocket Signaling
      onIceCandidate?.call(candidate.toMap());
    };

    // Xử lý Remote Stream (Khi người khác gửi video)
    _peerConnection!.onTrack = (event) {
      if (event.track.kind == 'video' || event.track.kind == 'audio') {
        // Nếu là video, render ra
        if (event.streams.isNotEmpty) {
          final remoteStream = event.streams[0];
          if (!_remoteStreams.contains(remoteStream)) {
            _remoteStreams.add(remoteStream);
            _remoteRenderer.srcObject = remoteStream;
            onRemoteStreamReady?.call(_remoteRenderer);
          }
        }
      }
    };
    
    // Xử lý thay đổi trạng thái kết nối
    _peerConnection!.onConnectionState = (state) {
      debugPrint('Connection State: ${state.name}');
    };
  }

  // 3. Tạo Offer (Gọi đi)
  Future<RTCSessionDescription> createOffer(String callId) async {
    final offer = await _peerConnection!.createOffer();
    await _peerConnection!.setLocalDescription(offer);
    return offer;
  }

  // 4. Xử lý Offer (Nhận cuộc gọi)
  Future<void> handleOffer(RTCSessionDescription offer) async {
    await _peerConnection!.setRemoteDescription(offer);
    
    // Tạo Answer
    final answer = await _peerConnection!.createAnswer();
    await _peerConnection!.setLocalDescription(answer);
    
    // Trả Answer về Signaling
    onSignalingMessage?.call(answer);
  }

  // 5. Xử lý Answer
  Future<void> handleAnswer(RTCSessionDescription answer) async {
    await _peerConnection!.setRemoteDescription(answer);
  }

  // 6. Thêm ICE Candidate nhận được từ Server
  Future<void> addCandidate(Map<String, dynamic> candidateMap) async {
    final candidate = RTCIceCandidate(candidateMap);
    await _peerConnection!.addCandidate(candidate);
  }

  // --- CONTROLS ---
  
  // Bật/Tắt Microphone
  void toggleAudio(bool enabled) {
    _localStream?.getAudioTracks().forEach((track) => track.enabled = enabled);
  }

  // Bật/Tắt Camera
  void toggleVideo(bool enabled) {
    _localStream?.getVideoTracks().forEach((track) => track.enabled = enabled);
  }

  // Chuyển Camera (Front <-> Back)
  Future<void> switchCamera() async {
    if (_localStream != null) {
      final videoTrack = _localStream!.getVideoTracks()[0];
      // Helper method để switch track trong flutter_webrtc
      await Helper.switchCamera(videoTrack);
    }
  }
  
  // Kết thúc cuộc gọi
  Future<void> dispose() async {
    _localStream?.getTracks().forEach((track) => track.stop());
    await _localRenderer.dispose();
    await _remoteRenderer.dispose();
    await _peerConnection?.close();
    _peerConnection = null;
  }
}
```

---

## 16.5. Signaling Client (WebSocket)

Kết nối với `wss://api.secureconnect.com/v1/ws/signaling`.

**File: `lib/features/call/data/services/signaling_client.dart`**

```dart
import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';

class SignalingClient {
  final String _wsUrl;
  WebSocketChannel? _channel;
  final Function(Map<String, dynamic>) _onMessage;

  SignalingClient(this._wsUrl, this._onMessage);

  void connect() {
    _channel = WebSocketChannel.connect(Uri.parse(_wsUrl));
    
    _channel!.stream.listen(
      (message) {
        final data = jsonDecode(message);
        _onMessage(data);
      },
      onError: (error) => print('WS Error: $error'),
      onDone: () => print('WS Closed'),
    );
  }

  void send(Map<String, dynamic> payload) {
    _channel?.sink.add(jsonEncode(payload));
  }

  void close() {
    _channel?.sink.close();
  }
}
```

---

## 16.6. State Management (Riverpod/Bloc)

Quản lý trạng thái cuộc gọi (Ringing -> Connected -> Ended).

**File: `lib/features/call/presentation/providers/call_state_provider.dart` (Dùng Riverpod ví dụ)**

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import '../../data/services/webrtc_manager.dart';

enum CallStatus { idle, ringing, connected, ended, failed }

class CallState {
  final CallStatus status;
  final RTCVideoRenderer? localRenderer;
  final RTCVideoRenderer? remoteRenderer;
  final bool isAudioEnabled;
  final bool isVideoEnabled;
  final bool isEncrypted; // E2EE Mode

  CallState({
    required this.status,
    this.localRenderer,
    this.remoteRenderer,
    this.isAudioEnabled = true,
    this.isVideoEnabled = true,
    this.isEncrypted = true,
  });

  CallState copyWith({
    CallStatus? status,
    RTCVideoRenderer? localRenderer,
    RTCVideoRenderer? remoteRenderer,
    bool? isAudioEnabled,
    bool? isVideoEnabled,
    bool? isEncrypted,
  }) {
    return CallState(
      status: status ?? this.status,
      localRenderer: localRenderer ?? this.localRenderer,
      remoteRenderer: remoteRenderer ?? this.remoteRenderer,
      isAudioEnabled: isAudioEnabled ?? this.isAudioEnabled,
      isVideoEnabled: isVideoEnabled ?? this.isVideoEnabled,
      isEncrypted: isEncrypted ?? this.isEncrypted,
    );
  }
}

class CallNotifier extends StateNotifier<CallState> {
  late WebRTCManager _webrtcManager;
  late SignalingClient _signalingClient;
  late String _callId;

  CallNotifier() : super(CallState(status: CallStatus.idle));

  Future<void> startCall(String callId, Map<String, dynamic> iceServers, bool isEncrypted) async {
    _callId = callId;
    state = state.copyWith(isEncrypted: isEncrypted, status: CallStatus.ringing);

    // 1. Init WebRTC
    _webrtcManager = WebRTCManager(iceServers);
    await _webrtcManager.initLocalStream();
    await _webrtcManager.createPeerConnection();

    // Setup Callbacks
    _webrtcManager.onLocalStreamReady = (renderer) {
      state = state.copyWith(localRenderer: renderer);
    };
    _webrtcManager.onRemoteStreamReady = (renderer) {
      state = state.copyWith(remoteRenderer: renderer, status: CallStatus.connected);
    };
    
    // 2. Setup Signaling
    final token = '...'; // Get from Auth Provider
    _signalingClient = SignalingClient(
      'wss://api.secureconnect.com/v1/ws/signaling?token=$token',
      _handleSignalingMessage
    );
    _signalingClient.connect();

    // 3. Create Offer & Send
    final offer = await _webrtcManager.createOffer(callId);
    _signalingClient.send({
      'type': 'offer',
      'call_id': callId,
      'sdp': offer.sdp,
    });
  }

  void _handleSignalingMessage(Map<String, dynamic> msg) {
    switch (msg['type']) {
      case 'answer':
        final desc = RTCSessionDescription(msg['sdp'], 'answer');
        _webrtcManager.handleAnswer(desc);
        break;
      case 'ice_candidate':
        _webrtcManager.addCandidate(msg['candidate']);
        break;
      case 'user_joined':
        // Xử lý logic hiển thị người tham gia
        break;
    }
  }

  void toggleAudio() {
    _webrtcManager.toggleAudio(!state.isAudioEnabled);
    state = state.copyWith(isAudioEnabled: !state.isAudioEnabled);
  }

  void endCall() async {
    _signalingClient.send({'type': 'leave', 'call_id': _callId});
    await _webrtcManager.dispose();
    state = CallState(status: CallStatus.ended);
  }
}

final callProvider =
    StateNotifierProvider<CallNotifier, CallState>((ref) => CallNotifier());
```

---

## 16.7. UI Implementation (Video Call Page)

Sử dụng `RTCVideoRendererView` để hiển thị video và `Stack` để xếp Local lên trên Remote.

**File: `lib/features/call/presentation/pages/video_call_page.dart`**

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'providers/call_state_provider.dart';

class VideoCallPage extends ConsumerWidget {
  final String callId;
  final Map<String, dynamic> iceServers;
  final bool isEncrypted; // From settings/conversation metadata

  const VideoCallPage({
    super.key,
    required this.callId,
    required this.iceServers,
    required this.isEncrypted,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final callState = ref.watch(callProvider);
    
    // Auto start when widget init
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(callProvider.notifier).startCall(callId, iceServers, isEncrypted);
    });

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        elevation: 0,
        actions: [
          if (!isEncrypted)
            const Padding(
              padding: EdgeInsets.all(8.0),
              child: Chip(
                label: Text("AI Mode Active", style: TextStyle(color: Colors.black)),
                backgroundColor: Colors.red,
              ),
            ),
          IconButton(
            icon: const Icon(Icons.switch_camera, color: Colors.white),
            onPressed: () => ref.read(callProvider.notifier).switchCamera(),
          ),
        ],
      ),
      body: Stack(
        children: [
          // 1. REMOTE VIDEO (Nền)
          if (callState.remoteRenderer != null)
            Center(
              child: RTCVideoView(
                callState.remoteRenderer!,
                objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitCover,
              ),
            )
          else
            const Center(child: CircularProgressIndicator()),

          // 2. LOCAL VIDEO (Góc phải - PiP)
          if (callState.localRenderer != null)
            Positioned(
              top: 20,
              right: 20,
              width: 120,
              height: 160,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: RTCVideoView(
                  callState.localRenderer!,
                  mirror: true, // Gương cho camera trước
                  objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitCover,
                ),
              ),
            ),

          // 3. CONTROLS (Thanh dưới)
          if (callState.status == CallStatus.connected)
            Align(
              alignment: Alignment.bottomCenter,
              child: Container(
                margin: const EdgeInsets.only(bottom: 30),
                padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 15),
                decoration: BoxDecoration(
                  color: Colors.grey[800]!.withOpacity(0.7),
                  borderRadius: BorderRadius.circular(30),
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                    // Mute Audio
                    ControlButton(
                      icon: callState.isAudioEnabled ? Icons.mic : Icons.mic_off,
                      onTap: () => ref.read(callProvider.notifier).toggleAudio(),
                      color: callState.isAudioEnabled ? Colors.white : Colors.red,
                    ),
                    // Mute Video
                    ControlButton(
                      icon: callState.isVideoEnabled ? Icons.videocam : Icons.videocam_off,
                      onTap: () => ref.read(callProvider.notifier).toggleVideo(),
                      color: callState.isVideoEnabled ? Colors.white : Colors.red,
                    ),
                    // End Call
                    ControlButton(
                      icon: Icons.call_end,
                      onTap: () {
                        ref.read(callProvider.notifier).endCall();
                        Navigator.pop(context);
                      },
                      color: Colors.red,
                      isEndCall: true,
                    ),
                    // Screen Share (Placeholder logic)
                    ControlButton(
                      icon: Icons.screen_share,
                      onTap: () => print("Screen Share"),
                      color: Colors.white,
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}

// Widget nút bấm tái sử dụng
class ControlButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;
  final Color color;
  final bool isEndCall;

  const ControlButton({
    super.key,
    required this.icon,
    required this.onTap,
    required this.color,
    this.isEndCall = false,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(30),
      child: Container(
        padding: EdgeInsets.all(isEndCall ? 15 : 12),
        decoration: BoxDecoration(
          color: isEndCall ? Colors.red : Colors.grey[700],
          shape: BoxShape.circle,
        ),
        child: Icon(icon, color: color, size: 24),
      ),
    );
  }
}
```

---

## 16.8. Tích hợp Screen Sharing

Để chia sẻ màn hình, bạn cần lấy stream màn hình (`getDisplayMedia`) và thay thế track video hiện tại vào PeerConnection.

```dart
// Thêm vào WebRTCManager
Future<void> startScreenShare() async {
  // Chỉ hoạt động tốt trên Web. Mobile cần các plugin khác hoặc custom channels.
  final stream = await navigator.mediaDevices.getDisplayMedia({
    'video': {'cursor': 'always'},
    'audio': false
  });
  
  // Lấy Video Track của screen share
  final videoTrack = stream.getVideoTracks()[0];
  
  // Thay thế (Replace) track trong PeerConnection
  // Cần tìm sender của track video cũ và replace
  // Code chi tiết có trong thư viện flutter_webrtc examples
}

Future<void> stopScreenShare() {
  // Switch back to camera
}
```

---

## 16.9. Giao diện Tối ưu cho Video Calls

1.  **Orientation Lock:** Khóa xoay màn hình (Landscape cho Group Call, Portrait cho 1-1) để tránh việc Camera xoay theo thiết bị gây lỗi render.
    ```dart
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.portraitUp,
      DeviceOrientation.portraitDown,
    ]);
    ```
2.  **Wakelock:** Giữ màn hình sáng khi đang gọi video.
    ```dart
    WakelockPlus.enable(); // Dùng wakelock_plus package
    ```
3.  **Picture-in-Picture (PiP):** Nếu người dùng bấm Home trên Mobile, hãy hiển thị Picture-in-Picture (thư viện `flutter_pip`). Điều này đòi hỏi backend SFU (Pion) tiếp tục gửi stream.

---

## 16.10. Bảo mật & UI Feedback

*   **Recording Indicator:** Nếu `is_encrypted = false` (Opt-out Mode), UI phải hiển thị **Icon Red Dot** hoặc nhấp nháy ở góc màn hình để báo cho user biết cuộc gọi có thể đang được ghi âm.
*   **Secure Connection:** Hiển thị biểu tượng ổ khóa vàng (🔒) khi E2EE đang hoạt động.

---

*Liên kết đến tài liệu tiếp theo:* `flutter/edge-ai-setup.md`