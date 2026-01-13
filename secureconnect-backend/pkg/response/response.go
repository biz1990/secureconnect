package response

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Response represents standard API response envelope
// Per spec: docs/05-api-design.md
type Response struct {
	Success bool         `json:"success"`
	Data    interface{}  `json:"data,omitempty"`
	Error   *ErrorDetail `json:"error,omitempty"`
	Meta    Meta         `json:"meta"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code    string `json:"code"`    // Error code (e.g., "INVALID_CREDENTIALS")
	Message string `json:"message"` // Human-readable error message
}

// Meta contains response metadata
type Meta struct {
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id,omitempty"`
}

// Success sends a successful response
func Success(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, Response{
		Success: true,
		Data:    data,
		Meta: Meta{
			Timestamp: time.Now().UTC(),
			RequestID: getRequestID(c),
		},
	})
}

// Error sends an error response
func Error(c *gin.Context, statusCode int, errorCode, errorMessage string) {
	c.JSON(statusCode, Response{
		Success: false,
		Error: &ErrorDetail{
			Code:    errorCode,
			Message: errorMessage,
		},
		Meta: Meta{
			Timestamp: time.Now().UTC(),
			RequestID: getRequestID(c),
		},
	})
}

// ValidationError sends a validation error response (400)
func ValidationError(c *gin.Context, message string) {
	Error(c, 400, "VALIDATION_ERROR", message)
}

// Unauthorized sends unauthorized error (401)
func Unauthorized(c *gin.Context, message string) {
	Error(c, 401, "UNAUTHORIZED", message)
}

// Forbidden sends forbidden error (403)
func Forbidden(c *gin.Context, message string) {
	Error(c, 403, "FORBIDDEN", message)
}

// NotFound sends not found error (404)
func NotFound(c *gin.Context, message string) {
	Error(c, 404, "NOT_FOUND", message)
}

// Conflict sends conflict error (409)
func Conflict(c *gin.Context, message string) {
	Error(c, 409, "CONFLICT", message)
}

// InternalError sends internal server error (500)
func InternalError(c *gin.Context, message string) {
	Error(c, 500, "INTERNAL_ERROR", message)
}

// getRequestID extracts request ID from context
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
