package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents application-specific error codes
type ErrorCode string

const (
	// Validation errors
	ErrCodeValidation   ErrorCode = "VALIDATION_ERROR"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeMissingField ErrorCode = "MISSING_FIELD"

	// Authentication errors
	ErrCodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrCodeInvalidToken   ErrorCode = "INVALID_TOKEN"
	ErrCodeExpiredToken   ErrorCode = "EXPIRED_TOKEN"
	ErrCodeInvalidCreds   ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeSessionExpired ErrorCode = "SESSION_EXPIRED"

	// Authorization errors
	ErrCodeForbidden      ErrorCode = "FORBIDDEN"
	ErrCodeAccessDenied   ErrorCode = "ACCESS_DENIED"
	ErrCodeResourceLocked ErrorCode = "RESOURCE_LOCKED"

	// Not found errors
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUserNotFound ErrorCode = "USER_NOT_FOUND"
	ErrCodeFileNotFound ErrorCode = "FILE_NOT_FOUND"
	ErrCodeCallNotFound ErrorCode = "CALL_NOT_FOUND"

	// Conflict errors
	ErrCodeConflict       ErrorCode = "CONFLICT"
	ErrCodeEmailExists    ErrorCode = "EMAIL_EXISTS"
	ErrCodeUsernameExists ErrorCode = "USERNAME_EXISTS"
	ErrCodeResourceInUse  ErrorCode = "RESOURCE_IN_USE"

	// Rate limiting errors
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Quota errors
	ErrCodeQuotaExceeded ErrorCode = "QUOTA_EXCEEDED"

	// Internal errors
	ErrCodeInternal       ErrorCode = "INTERNAL_ERROR"
	ErrCodeDatabase       ErrorCode = "DATABASE_ERROR"
	ErrCodeStorage        ErrorCode = "STORAGE_ERROR"
	ErrCodeServiceUnavail ErrorCode = "SERVICE_UNAVAILABLE"
)

// AppError represents a structured application error with code, message, and HTTP status
type AppError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	StatusCode int       `json:"-"`
	Details    any       `json:"details,omitempty"`
	Err        error     `json:"-"`
}

// Error implements the error interface, returning a formatted error message
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError with the given code and message
// The status code defaults to 500 Internal Server Error
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewWithStatus creates a new AppError with a specific HTTP status code
func NewWithStatus(code ErrorCode, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Wrap wraps an existing error with an AppError, preserving the original error
// The status code defaults to 500 Internal Server Error
func Wrap(code ErrorCode, message string, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
}

// WrapWithStatus wraps an existing error with an AppError and specific status code
func WrapWithStatus(code ErrorCode, message string, statusCode int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
	}
}

// WithDetails adds additional details to an AppError for debugging
func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}

// Validation errors
func ValidationError(message string) *AppError {
	return NewWithStatus(ErrCodeValidation, message, http.StatusBadRequest)
}

func InvalidInputError(message string) *AppError {
	return NewWithStatus(ErrCodeInvalidInput, message, http.StatusBadRequest)
}

func MissingFieldError(field string) *AppError {
	return NewWithStatus(ErrCodeMissingField, fmt.Sprintf("Missing required field: %s", field), http.StatusBadRequest)
}

// Authentication errors
func UnauthorizedError(message string) *AppError {
	return NewWithStatus(ErrCodeUnauthorized, message, http.StatusUnauthorized)
}

func InvalidTokenError(message string) *AppError {
	return NewWithStatus(ErrCodeInvalidToken, message, http.StatusUnauthorized)
}

func ExpiredTokenError() *AppError {
	return NewWithStatus(ErrCodeExpiredToken, "Token has expired", http.StatusUnauthorized)
}

func InvalidCredentialsError() *AppError {
	return NewWithStatus(ErrCodeInvalidCreds, "Invalid email or password", http.StatusUnauthorized)
}

func SessionExpiredError() *AppError {
	return NewWithStatus(ErrCodeSessionExpired, "Session has expired", http.StatusUnauthorized)
}

// Authorization errors
func ForbiddenError(message string) *AppError {
	return NewWithStatus(ErrCodeForbidden, message, http.StatusForbidden)
}

func AccessDeniedError(message string) *AppError {
	return NewWithStatus(ErrCodeAccessDenied, message, http.StatusForbidden)
}

// Not found errors
func NotFoundError(resource string) *AppError {
	return NewWithStatus(ErrCodeNotFound, fmt.Sprintf("%s not found", resource), http.StatusNotFound)
}

func UserNotFoundError() *AppError {
	return NewWithStatus(ErrCodeUserNotFound, "User not found", http.StatusNotFound)
}

func FileNotFoundError() *AppError {
	return NewWithStatus(ErrCodeFileNotFound, "File not found", http.StatusNotFound)
}

func CallNotFoundError() *AppError {
	return NewWithStatus(ErrCodeCallNotFound, "Call not found", http.StatusNotFound)
}

// Conflict errors
func ConflictError(message string) *AppError {
	return NewWithStatus(ErrCodeConflict, message, http.StatusConflict)
}

func EmailExistsError() *AppError {
	return NewWithStatus(ErrCodeEmailExists, "Email already registered", http.StatusConflict)
}

func UsernameExistsError() *AppError {
	return NewWithStatus(ErrCodeUsernameExists, "Username already taken", http.StatusConflict)
}

// Rate limiting errors
func RateLimitExceededError() *AppError {
	return NewWithStatus(ErrCodeRateLimitExceeded, "Rate limit exceeded", http.StatusTooManyRequests)
}

// Quota errors
func QuotaExceededError(message string) *AppError {
	return NewWithStatus(ErrCodeQuotaExceeded, message, http.StatusPaymentRequired)
}

// Internal errors
func InternalError(message string) *AppError {
	return NewWithStatus(ErrCodeInternal, message, http.StatusInternalServerError)
}

func DatabaseError(err error) *AppError {
	return WrapWithStatus(ErrCodeDatabase, "Database error", http.StatusInternalServerError, err)
}

func StorageError(err error) *AppError {
	return WrapWithStatus(ErrCodeStorage, "Storage error", http.StatusInternalServerError, err)
}

func ServiceUnavailableError(message string) *AppError {
	return NewWithStatus(ErrCodeServiceUnavail, message, http.StatusServiceUnavailable)
}

// IsAppError checks if an error is an AppError type
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError extracts AppError from an error, wrapping non-AppErrors as InternalError
func GetAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return InternalError(err.Error())
}
