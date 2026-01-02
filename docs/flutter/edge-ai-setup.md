# Flutter Edge AI Setup Guide (Hybrid Security Mode)

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 17.1. Tổng quan

Trong mô hình **Hybrid Security (Option B)** mà chúng ta đã chọn:
1.  **Khi E2EE = ON (Secure Mode):** Server (Go) không đọc được nội dung. Do đó, **Client (Flutter) PHẢI chạy AI tại chỗ (Edge AI)** để phân tích cảm xúc hoặc gợi ý trả lời. Dữ liệu không bao giờ rời khỏi thiết bị.
2.  **Khi E2EE = OFF (Intelligent Mode):** Client có thể chọn dùng AI từ Server (như GPT-4) hoặc tiếp tục dùng Edge AI để có phản hồi tức thời (Zero latency).

Tài liệu này hướng dẫn tích hợp **Edge AI** vào Flutter.

---

## 17.2. Lựa chọn Tech Stack (On-Device AI)

Để chạy AI trên di động mà không làm chậm ứng dụng, chúng ta dùng các thư viện tối ưu hóa:

| Tính năng | Thư viện | Lý do |
| :--- | :--- | :--- |
| **Smart Reply** | `google_ml_kit` (Smart Reply) | Sẵn có, không cần train lại, cực nhẹ. |
| **Sentiment Analysis** | `tflite_flutter` (TensorFlow Lite) | ML Kit chưa có API Sentiment. Dùng model BERT-lite đã Quantized. |
| **Speech-to-Text** | `google_ml_kit` (Speech Recognizer) | Nhận diện giọng nói offline đa ngôn ngữ. |
| **Translation** | `google_ml_kit` (On-device Translator) | Dịch thuật offline, không cần internet. |

---

## 17.3. Cài đặt Dependencies

Thêm vào `pubspec.yaml`:

```yaml
dependencies:
  flutter:
    sdk: flutter

  # Core AI Libraries
  google_ml_kit: ^0.16.0
  tflite_flutter: ^0.10.4
  
  # State Management
  flutter_riverpod: ^2.4.0
  
  # Utilities
  flutter_tts: ^3.8.0 # Text to Speech (Tùy chọn)
```

---

## 17.4. Kiến trúc AI Service

Tạo một service độc lập xử lý logic AI, tách biệt khỏi UI.

```bash
lib/features/ai/
├── domain/
│   └── repositories/
│       └── ai_repository.dart      # Interface: getSmartReply(), getSentiment()
├── data/
│   └── services/
│       ├── edge_ai_service.dart    # Implementation bằng ML Kit & TFLite
│       └── models/
│           └── sentiment_model.tflite # File model AI (để trong assets)
└── presentation/
    └── widgets/
        └── smart_reply_chips.dart   # UI hiển thị gợi ý
```

---

## 17.5. Tính năng 1: Smart Reply (Gợi ý trả lời)

Sử dụng `SmartReplySuggester` của ML Kit. Nó học từ lịch sử chat gần đây để gợi ý câu trả lời phù hợp.

**File: `lib/features/ai/data/services/edge_ai_service.dart`**

```dart
import 'package:google_ml_kit/google_ml_kit.dart';

class EdgeAIService {
  late final SmartReplySuggester _smartReplySuggester;
  late final TextClassifier _sentimentClassifier; // Sẽ cài ở phần sau

  // Init Service (khi app mở)
  Future<void> init() async {
    // Load model Smart Reply
    _smartReplySuggester = SmartReplySuggester();
    // Load model Sentiment (TFLite)
    // await _sentimentClassifier.loadModel(); 
  }

  // --- SMART REPLY ---

  /// Gợi ý câu trả lời dựa trên tin nhắn nhận được
  Future<List<String>> generateSmartReplies(String incomingText, {List<String>? history}) async {
    
    // 1. Chuẩn bị RemoteMessage (Object của ML Kit)
    final remoteMessage = RemoteMessage(
      text: incomingText,
      timestamp: DateTime.now().millisecondsSinceEpoch,
    );

    // 2. Nếu có lịch sử chat (Context), thêm vào để AI hiểu rõ hơn
    if (history != null) {
      // Lưu ý: ML Kit chỉ cần vài tin nhắn gần nhất
      // Code giả lập thêm context...
    }

    // 3. Gọi ML Kit
    final suggestions = await _smartReplySuggester.suggestReplies(
      conversation: [remoteMessage]
    );

    // 4. Convert kết quả sang List String
    return suggestions.map((e) => e.text).toList();
  }
}
```

---

## 17.6. Tính năng 2: Sentiment Analysis (Phân tích cảm xúc)

Vì ML Kit chưa hỗ trợ Sentiment mạnh mẽ, chúng ta sẽ dùng **TensorFlow Lite (TFLite)** để chạy một model BERT nhỏ.

**Bước 1: Tải Model**
Bạn cần file `.tflite` (ví dụ: `mobilebert_sentiment.tflite`). Đặt nó vào thư mục `assets/models/`.

**Bước 2: Cấu hình Pubspec**
```yaml
flutter:
  assets:
    - assets/models/mobilebert_sentiment.tflite
```

**Bước 3: Code TFLite**

**File: `lib/features/ai/data/services/tflite_sentiment.dart`**

```dart
import 'dart:io';
import 'package:tflite_flutter/tflite_flutter.dart';
import 'package:tflite_flutter_helper/tflite_flutter_helper.dart';

class SentimentAnalyzer {
  final String _modelPath = 'assets/models/mobilebert_sentiment.tflite';
  Interpreter? _interpreter;
  List<String>? _labels; // ["negative", "positive", "neutral"]

  Future<void> loadModel() async {
    try {
      _interpreter = await Tflite.loadModel(
        model: _modelPath,
      );
      
      // Load labels nếu file phụ tùng
      // _labels = await File('assets/models/labels.txt').readAsLines();
    } catch (e) {
      print('Error loading TFLite model: $e');
    }
  }

  /// Phân tích cảm xúc một câu văn bản
  Future<Map<String, double>> analyzeSentiment(String text) async {
    if (_interpreter == null) return {};

    // 1. Tiền xử lý (Tokenizer) - Phức t nhất phần này
    // Bạn cần convert text sang input tensor của model (ví dụ: input_ids, attention_mask)
    // Thường dùng thư viện "bert_tokenizer" Dart hoặc viết hàm map thủ công.
    var input = tokenizeText(text); 

    // 2. Run Inference
    var output = List.filled(1 * 3, 0.0).reshape([1, 3]); // 3 lớp: negative, neutral, positive
    await _interpreter!.run(input, output);

    // 3. Post-processing
    // Output sẽ là mảng xác suất: [0.1, 0.2, 0.7]
    return {
      "negative": output[0][0],
      "neutral": output[0][1],
      "positive": output[0][2],
    };
  }

  // Giả lập hàm tokenize (Thực tế cần phức tạp hơn)
  List<List<double>> tokenizeText(String text) {
    // ... Logic convert text sang số ...
    return []; 
  }
  
  void dispose() {
    _interpreter?.close();
  }
}
```

---

## 17.7. Tính năng 3: Speech-to-Text (Chuyển giọng nói thành văn bản)

Hỗ trợ gửi tin nhắn thoại hoặc tạo caption cho video call.

**File: `lib/features/ai/data/services/speech_service.dart`**

```dart
import 'package:google_ml_kit/google_ml_kit.dart';

class SpeechService {
  late final SpeechToText _speechToText;
  bool _isListening = false;

  SpeechService() {
    _speechToText = SpeechToText();
  }

  Future<void> startListening(Function(String) onResult) async {
    // 1. Kiểm tra quyền Microphone
    // await Permission.microphone.request();

    // 2. Cấu hình Recognizer
    _speechToText.listen(
      onResult: (result) {
        if (result.finalResult) {
          // Đã nói xong câu hoàn chỉnh
          onResult(result.recognizedWords);
        }
      },
      listenFor: Duration(seconds: 30), // Nghe tối đa 30s
      pauseFor: Duration(seconds: 3),
      partialResults: true, // Hiển thị từ đang nói (Realtime)
      localeId: 'vi_VN', // Ngôn ngữ Tiếng Việt
      onSoundLevelChange: (level) {
        // Hiển thị sóng âm UI
      },
    );
    
    _isListening = true;
  }

  void stopListening() {
    _speechToText.stop();
    _isListening = false;
  }
}
```

---

## 17.8. Tích hợp vào Logic Hybrid (Quan trọng)

Đây là phần "huyết mạch" của Option B. Client phải tự quyết định chạy AI ở đâu.

**File: `lib/features/chat/presentation/providers/chat_provider.dart` (Snippet)**

```dart
class ChatNotifier extends StateNotifier<ChatState> {
  final EdgeAIService _edgeAI;
  final ApiClient _apiClient; // Server AI

  ChatNotifier(this._edgeAI, this._apiClient);

  Future<void> processIncomingMessage(Message msg) async {
    // 1. Nếu tin nhắn đã mã hóa (E2EE ON) -> CHẠY EDGE AI (Bắt buộc)
    if (msg.isEncrypted) {
      // Lưu ý: msg.content là ciphertext. 
      // Client phải GIẢI MÃ trước khi cho AI đọc.
      String plainText = await _cryptoService.decrypt(msg.content);
      
      // A. Smart Reply (Local)
      final suggestions = await _edgeAI.generateSmartReplies(plainText);
      state = state.copyWith(smartReplies: suggestions);

      // B. Sentiment Analysis (Local)
      final sentiment = await _edgeAI.analyzeSentiment(plainText);
      state = state.copyWith(lastSentiment: sentiment);
    
    } else {
      // 2. Nếu tin nhắn thuần (E2EE OFF) -> TÙY CHỌN
      
      // Option A: Gọi Server AI (Mạnh mẽ hơn, tốn thời gian)
      // final serverResult = await _apiClient.analyzeText(msg.content);
      // state = state.copyWith(smartReplies: serverResult.suggestions);

      // Option B: Vẫn dùng Edge AI (Nhanh hơn, miễn phí)
      final suggestions = await _edgeAI.generateSmartReplies(msg.content);
      state = state.copyWith(smartReplies: suggestions);
    }
  }
}
```

---

## 17.9. UI Implementation: Smart Reply Chips

Hiển thị các nút bấm gợi ý ngay trên thanh nhập liệu.

**File: `lib/features/ai/presentation/widgets/smart_reply_chips.dart`**

```dart
import 'package:flutter/material.dart';

class SmartReplyChips extends StatelessWidget {
  final List<String> suggestions;
  final Function(String) onReplyTap;

  const SmartReplyChips({
    super.key,
    required this.suggestions,
    required this.onReplyTap,
  });

  @override
  Widget build(BuildContext context) {
    if (suggestions.isEmpty) return SizedBox.shrink();

    return Container(
      height: 50,
      padding: EdgeInsets.symmetric(vertical: 10),
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: 16),
        itemCount: suggestions.length,
        separatorBuilder: (_) => SizedBox(width: 10),
        itemBuilder: (context, index) {
          return _Chip(
            label: suggestions[index],
            onTap: () => onReplyTap(suggestions[index]),
          );
        },
      ),
    );
  }

  Widget _Chip({required String label, required VoidCallback onTap}) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        decoration: BoxDecoration(
          color: Colors.white.withOpacity(0.2),
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: Colors.white70),
        ),
        child: Text(
          label,
          style: TextStyle(color: Colors.white, fontSize: 14),
        ),
      ),
    );
  }
}
```

**Cách dùng trong `chat_detail_page.dart`:**
```dart
// Trong Column
Column(
  children: [
    Expanded(child: MessageList(...)),
    
    // Smart Reply Area
    SmartReplyChips(
      suggestions: chatState.smartReplies,
      onReplyTap: (text) {
        // Bấm gợi ý -> Điền vào input -> Gửi đi
        inputController.text = text;
        _sendMessage();
      },
    ),

    // Input Field
    MessageInputField(...),
  ]
)
```

---

## 17.10. UI Implementation: Sentiment Indicator

Hiển thị icon cảm xúc cạnh tin nhắn.

```dart
Widget _buildSentimentIcon(String sentiment) {
  IconData icon;
  Color color;

  switch (sentiment.toLowerCase()) {
    case 'positive':
      icon = Icons.sentiment_very_satisfied;
      color = Colors.green;
      break;
    case 'negative':
      icon = Icons.sentiment_very_dissatisfied;
      color = Colors.red;
      break;
    default:
      icon = Icons.sentiment_satisfied;
      color = Colors.grey;
  }

  return Icon(icon, color: color, size: 16);
}
```

---

## 17.11. Lợi ích của Edge AI (Tại sao nên dùng trong Option B?)

1.  **Privacy (Riêng tư):** Dữ liệu không bao giờ gửi lên server. Ngay cả khi Server bị hack, nội dung chat của bạn vẫn an toàn.
2.  **Latency (Độ trễ):** AI chạy ngay trên CPU/GPU điện thoại (10-50ms) thay vì chờ phản hồi Server (500ms-2s).
3.  **Offline Mode (Ngoại tuyến):** Bạn vẫn có gợi ý trả lời và phân tích cảm xúc khi đang đi máy bay hoặc ở vùng mất sóng.
4.  **Chi phí:** Miễn phí cho App Owner (không tốn tiền gọi API OpenAI).

---

## 17.12. Lưu ý về Hiệu năng (Performance)

*   **Load Model:** Tải model TFLite (BERT) tốn ~30-50MB RAM và vài giây khởi tạo. Nên load ở thời điểm app mở (Splash Screen).
*   **Thread:** Chạy inference AI trên **Isolate** riêng biệt (bằng `compute`) để không làm giật UI khi gõ phím.
*   **Battery:** AI (đặc biệt Sentiment/Translation) tiêu tốn pin. Cần có "Smart Toggle" để người dùng tắt tính năng này nếu cần tiết kiệm pin.

---

*Liên kết đến tài liệu tiếp theo:* `devops/docker-setup.md`