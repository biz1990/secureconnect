# Backend AI Service Integration Guide (Go)

**Project:** SecureConnect SaaS Platform  
**Version:** 1.0  
**Status:** Draft  
**Author:** System Architect

## 12.1. Tổng quan

Trong mô hình **Hybrid Security** (Opt-out E2EE), Backend (Go) đóng vai trò trung gian. Khi tin nhắn đến:
1.  Nếu `is_encrypted = true`: Go bỏ qua, lưu vào Cassandra.
2.  Nếu `is_encrypted = false`: Go gọi **AI Service** để phân tích nội dung (Smart Reply, Sentiment, Translation).

### Kiến trúc tích hợp
*   **Core Service (Go):** Chứa business logic và gọi AI.
*   **AI Provider:** Có thể là **OpenAI API**, **Anthropic**, hoặc một internal **Python Microservice** (nếu dùng model tự-host như Llama).
*   **Caching Layer (Redis):** Lưu kết quả AI của các tin nhắn giống nhau để tiết kiệm chi phí API.

---

## 12.2. Các kịch bản AI (AI Scenarios)

Hệ thống hỗ trợ các tính năng AI sau:

1.  **Smart Reply (Gợi ý trả lời):** Gợi ý 3 câu trả lời phù hợp dựa trên tin nhắn cuối cùng.
2.  **Sentiment Analysis (Phân tích cảm xúc):** Xác định tin nhắn tích cực, tiêu cực hay trung lập.
3.  **Translation (Dịch thuật):** Dịch tự động tin nhắn sang ngôn ngữ người dùng.
4.  **Summarization (Tóm tắt):** Tóm tắt lịch sử chat dài (dành cho group chat).

---

## 12.3. Cài đặt thư viện Go (HTTP Client)

Sử dụng thư viện HTTP client của Go hoặc thư viện SDK có sẵn.

```bash
go get github.com/sashabaranov/go-openai # SDK không chính thức nhưng phổ biến cho Go
# Hoặc dùng HTTP client thuần: net/http
```

---

## 12.4. Data Models (Dữ liệu AI)

### File: `internal/ai/models.go`

```go
package ai

// AIMetadata là cấu trúc được lưu vào Cassandra/Postgres
type AIMetadata struct {
    Sentiment      *string   `json:"sentiment,omitempty"`       // "positive", "neutral", "negative"
    Confidence     *float64  `json:"confidence,omitempty"`
    Translations   map[string]string `json:"translations,omitempty"` // {"vi": "Xin chào", "en": "Hello"}
    SmartReplies   []string  `json:"smart_replies,omitempty"`
    Summary        *string   `json:"summary,omitempty"`
}

// ChatHistory để gửi cho LLM (để hiểu ngữ cảnh)
type ChatHistory struct {
    Role    string `json:"role"` // "user", "assistant"
    Content string `json:"content"`
}
```

---

## 12.5. AI Service Interface (Business Logic)

Định nghĩa interface để dễ dàng thay thế Provider (ví dụ: hôm nay dùng OpenAI, mai dùng Anthropic).

### File: `internal/ai/service.go`

```go
package ai

import (
    "context"
    "errors"
    "time"
)

type AIService interface {
    // AnalyzeSentiment phân tích cảm xúc của một đoạn văn bản
    AnalyzeSentiment(ctx context.Context, text string) (sentiment string, confidence float64, err error)

    // GenerateSmartReplies gợi ý câu trả lời
    GenerateSmartReplies(ctx context.Context, text string, history []ChatHistory) ([]string, error)

    // TranslateText dịch thuật
    TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error)
}

// OpenAIService là implementation cụ thể
type OpenAIService struct {
    apiKey string
    client *openai.Client // Giả sử dùng SDK hoặc http.Client
}

func NewOpenAIService(apiKey string) *OpenAIService {
    return &OpenAIService{
        apiKey: apiKey,
        client: openai.NewClient(apiKey),
    }
}
```

---

## 12.6. Triển khai Sentiment Analysis (Chi tiết)

Ví dụ gọi API OpenAI để phân tích cảm xúc.

### File: `internal/ai/sentiment.go`

```go
package ai

import (
    "context"
    "fmt"
    "strings"
)

func (s *OpenAIService) AnalyzeSentiment(ctx context.Context, text string) (string, float64, error) {
    // 1. Kiểm tra Cache Redis trước (Để tiết kiệm tiền)
    // cacheKey := fmt.Sprintf("sentiment:%s", text)
    // if val := redis.Get(cacheKey); val != nil { return val ... }

    // 2. Tạo Prompt cho LLM
    // Sử dụng technique "Prompt Engineering" để ép trả về JSON
    prompt := fmt.Sprintf(`
        Analyze the sentiment of the following message.
        Respond strictly in JSON format with keys: "sentiment" (positive, neutral, negative) and "confidence" (float 0.0 to 1.0).
        Message: "%s"
    `, text)

    // 3. Gọi API OpenAI (Dùng GPT-3.5-turbo hoặc GPT-4)
    // Đây là mã giả lập (Pseudo code) dựa trên SDK
    req := openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "You are a helpful sentiment analysis assistant."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0.1, // Thấp để kết quả ổn định
    }

    resp, err := s.client.CreateChatCompletion(ctx, req)
    if err != nil {
        return "", 0, err
    }

    // 4. Parse JSON response từ AI
    content := resp.Choices[0].Message.Content
    
    // Logic parse JSON string -> struct SentimentResponse
    // ...
    
    sentiment := "neutral" // Giả lập sau khi parse
    confidence := 0.95

    // 5. Lưu vào Cache (TTL 1 ngày)
    // redis.Set(cacheKey, result, 24*time.Hour)

    return sentiment, confidence, nil
}
```

---

## 12.7. Triển khai Smart Reply (Gợi ý trả lời)

### File: `internal/ai/smart_reply.go`

```go
func (s *OpenAIService) GenerateSmartReplies(ctx context.Context, text string, history []ChatHistory) ([]string, error) {
    // 1. Chuẩn bị Context (Lấy 5 tin nhắn gần nhất)
    contextMessages := make([]openai.ChatCompletionMessage, 0)
    contextMessages = append(contextMessages, openai.ChatCompletionMessage{
        Role: "system", Content: "Suggest 3 short, casual replies to the last message. Format as JSON list: ["reply 1", "reply 2", "reply 3"]",
    })

    // Convert history sang format OpenAI
    for _, h := range history {
        role := "user"
        if h.Role == "assistant" { role = "assistant" }
        contextMessages = append(contextMessages, openai.ChatCompletionMessage{
            Role: role, Content: h.Content,
        })
    }

    // Thêm tin nhắn hiện tại
    contextMessages = append(contextMessages, openai.ChatCompletionMessage{
        Role: "user", Content: text,
    })

    req := openai.ChatCompletionRequest{
        Model:       openai.GPT3Dot5Turbo,
        Messages:    contextMessages,
        Temperature: 0.7, // Cao hơn để đa dạng câu trả lời
        MaxTokens:   50,  // Giới hạn ký tự để câu trả lời ngắn
    }

    resp, err := s.client.CreateChatCompletion(ctx, req)
    if err != nil {
        return nil, err
    }

    // Parse response to list of strings
    // resp.Choices[0].Message.Content => ["Thật tuyệt!", "Tuyệt vời", "Cảm ơn"]
    replies := []string{"Thật tuyệt!", "Tuyệt vời", "Cảm ơn"} // Mockup

    return replies, nil
}
```

---

## 12.8. Tích hợp vào Chat Service (The Glue)

Đây là phần quan trọng nhất: Nơi `MessageService` quyết định có gọi AI hay không.

### File: `internal/chat/service.go` (Snippet)

```go
package chat

import (
    "context"
    "myproject/internal/ai"
    "myproject/internal/database"
)

type ChatService struct {
    repo       database.MessageRepository
    aiService  ai.AIService // Inject AI Service
    redis      *redis.Client
}

func NewChatService(repo database.MessageRepository, ai ai.AIService, redis *redis.Client) *ChatService {
    return &ChatService{repo: repo, aiService: ai, redis: redis}
}

func (s *ChatService) ProcessIncomingMessage(ctx context.Context, msg *models.Message) error {
    // 1. Lưu tin nhắn vào Database (Cassandra)
    if err := s.repo.Save(ctx, msg); err != nil {
        return err
    }

    // 2. LOGIC HYBRID SECURITY
    // Nếu tin nhắn đã mã hóa E2EE -> KHÔNG GỌI AI
    if msg.IsEncrypted {
        return nil 
    }

    // 3. Nếu là Plaintext (Opt-out E2EE) -> GỌI AI
    // Chạy xử lý AI bất đồng bộ (Async) để không làm chậm việc lưu tin nhắn
    go s.runAIProcessing(context.Background(), msg)

    return nil
}

func (s *ChatService) runAIProcessing(ctx context.Context, msg *models.Message) {
    // Timeout sau 5 giây để tránh treo hệ thống
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    var metadata ai.AIMetadata

    // A. Phân tích cảm xúc
    sentiment, conf, err := s.aiService.AnalyzeSentiment(ctx, msg.Content)
    if err == nil {
        metadata.Sentiment = &sentiment
        metadata.Confidence = &conf
    }

    // B. Smart Reply (Chỉ khi là tin nhắn dạng văn bản)
    if msg.ContentType == "text" {
        // Cần lấy history từ Redis/DB (Giả lập lấy history)
        history := []ai.ChatHistory{} // ... fetch history
        replies, err := s.aiService.GenerateSmartReplies(ctx, msg.Content, history)
        if err == nil {
            metadata.SmartReplies = replies
        }
    }

    // C. Cập nhật metadata vào Database
    // Cập nhật cột 'metadata' trong bảng Cassandra
    s.repo.UpdateMetadata(ctx, msg.MessageID, metadata)

    // D. Push cập nhật lại cho người gửi qua WebSocket
    // Để họ thấy các nút gợi ý trả lời hiện ra
    // wsHub.SendToUser(msg.SenderID, WSMessage{Type: "ai_ready", Data: metadata})
}
```

---

## 12.9. Chiến lược Timeout & Fallback (Quan trọng cho Production)

Gọi AI (LLM) có thể mất từ 500ms đến 10 giây. Không được để lỗi AI làm sập Chat.

1.  **Context Timeout:** Luôn set timeout (ví dụ 3s cho Smart Reply, 5s cho Translation). Nếu hết giờ, hủy request và trả về rỗng (không có gợi ý).
    ```go
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    replies, err := ai.GenerateSmartReplies(ctx, ...)
    ```
2.  **Fallback (Reserve):** Nếu gọi OpenAI lỗi (502/Rate Limit), có thể chuyển sang model rẻ hơn (ví dụ: GPT-3.5-Turbo) hoặc trả về danh sách gợi ý tĩnh (static templates: "Ok", "Tôi hiểu", "Nhận được rồi").
3.  **Queueing (Advanced):** Với các task nặng như **Tóm tắt cuộc họp** (Summarization), nên đưa vào **Message Queue (Kafka/RabbitMQ)**.
    *   Flow: User bấm "Tóm tắt" -> Gửi message vào Kafka -> Worker (Go) gọi AI -> Cập nhật vào DB -> WebSocket thông báo cho User khi xong.

---

## 12.10. Bảo mật & Chi phí

1.  **API Key Safety:** Khóa API OpenAI/Anthropic không bao giờ được hardcode trong code. Phải lưu trong **Environment Variables** (`AI_OPENAI_KEY`).
2.  **Content Moderation:** Mặc dù tắt E2EE, AI cũng nên được cấu hình để từ chối xử lý nội dung NSFW (Not Safe For Work) hoặc bạo lực (Content Moderation Policy).
3.  **Cost Monitoring:** Lưu log số token sử dụng của OpenAI hàng ngày vào Prometheus. Cảnh báo nếu chi phí vượt ngân sách.

---

*Liên kết đến tài liệu tiếp theo:* `backend/cassandra-integration-best-practices.md`