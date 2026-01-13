package password

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// ValidationError represents a password validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (ve *ValidationError) Error() string {
	return ve.Message
}

// Errors returns a slice of all validation errors
func (ve *ValidationError) Errors() []error {
	return []error{ve}
}

// ComplexityLevel represents password complexity level
type ComplexityLevel int

const (
	// ComplexityWeak indicates weak password
	ComplexityWeak ComplexityLevel = iota
	// ComplexityMedium indicates medium password
	ComplexityMedium
	// ComplexityStrong indicates strong password
	ComplexityStrong
	// ComplexityVeryStrong indicates very strong password
	ComplexityVeryStrong
)

// ComplexityRequirements defines password complexity requirements
type ComplexityRequirements struct {
	MinLength          int
	RequireUppercase   bool
	RequireLowercase   bool
	RequireNumber      bool
	RequireSpecialChar bool
}

// DefaultRequirements returns default complexity requirements
func DefaultRequirements() *ComplexityRequirements {
	return &ComplexityRequirements{
		MinLength:          8,
		RequireUppercase:   true,
		RequireLowercase:   true,
		RequireNumber:      true,
		RequireSpecialChar: true,
	}
}

// Validate validates password against complexity requirements
func Validate(password string, requirements *ComplexityRequirements) ([]*ValidationError, error) {
	if requirements == nil {
		requirements = DefaultRequirements()
	}

	var validationErrors []*ValidationError

	// Check minimum length
	if len(password) < requirements.MinLength {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "password",
			Message: fmt.Sprintf("Password must be at least %d characters", requirements.MinLength),
		})
	}

	// Check for uppercase letters
	if requirements.RequireUppercase {
		hasUppercase := regexp.MustCompile(`[A-Z]`).MatchString(password)
		if !hasUppercase {
			validationErrors = append(validationErrors, &ValidationError{
				Field:   "password",
				Message: "Password must contain at least one uppercase letter",
			})
		}
	}

	// Check for lowercase letters
	if requirements.RequireLowercase {
		hasLowercase := regexp.MustCompile(`[a-z]`).MatchString(password)
		if !hasLowercase {
			validationErrors = append(validationErrors, &ValidationError{
				Field:   "password",
				Message: "Password must contain at least one lowercase letter",
			})
		}
	}

	// Check for numbers
	if requirements.RequireNumber {
		hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
		if !hasNumber {
			validationErrors = append(validationErrors, &ValidationError{
				Field:   "password",
				Message: "Password must contain at least one number",
			})
		}
	}

	// Check for special characters
	if requirements.RequireSpecialChar {
		hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>?/~\\|]`).MatchString(password)
		if !hasSpecial {
			validationErrors = append(validationErrors, &ValidationError{
				Field:   "password",
				Message: "Password must contain at least one special character",
			})
		}
	}

	// Check for common patterns
	if isCommonPattern(password) {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "password",
			Message: "Password contains common patterns and is not secure",
		})
	}

	// Check for sequential characters
	if hasSequentialChars(password) {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "password",
			Message: "Password contains sequential characters (e.g., '123', 'abc')",
		})
	}

	// Check for repeated characters
	if hasRepeatedChars(password) {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "password",
			Message: "Password contains repeated characters (e.g., 'aaa', '111')",
		})
	}

	if len(validationErrors) > 0 {
		return validationErrors, errors.New("password validation failed")
	}

	return nil, nil
}

// CalculateComplexity calculates password complexity level
func CalculateComplexity(password string) ComplexityLevel {
	score := 0

	// Length score
	if len(password) >= 8 {
		score++
	}
	if len(password) >= 12 {
		score++
	}
	if len(password) >= 16 {
		score++
	}

	// Character variety score
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>?/~\\|]`).MatchString(password)

	varietyScore := 0
	if hasLower {
		varietyScore++
	}
	if hasUpper {
		varietyScore++
	}
	if hasNumber {
		varietyScore++
	}
	if hasSpecial {
		varietyScore++
	}

	score += varietyScore

	// Determine complexity level
	if score <= 2 {
		return ComplexityWeak
	} else if score <= 4 {
		return ComplexityMedium
	} else if score <= 5 {
		return ComplexityStrong
	}
	return ComplexityVeryStrong
}

// GetComplexityDescription returns a human-readable description of complexity level
func GetComplexityDescription(level ComplexityLevel) string {
	switch level {
	case ComplexityWeak:
		return "Weak - Your password is vulnerable to attacks"
	case ComplexityMedium:
		return "Medium - Your password could be stronger"
	case ComplexityStrong:
		return "Strong - Good password"
	case ComplexityVeryStrong:
		return "Very Strong - Excellent password"
	default:
		return "Unknown"
	}
}

// isCommonPattern checks if password contains common patterns
func isCommonPattern(password string) bool {
	// Convert to lowercase for comparison
	lower := strings.ToLower(password)

	// Common passwords list
	commonPasswords := []string{
		"password", "123456", "12345678", "qwerty",
		"abc123", "monkey", "password1", "letmein",
		"dragon", "111111", "baseball", "iloveyou",
		"trustno1", "sunshine", "master", "hello",
		"freedom", "whatever", "qazwsx", "admin",
		"welcome", "shadow", "ashley", "football",
		"jesus", "michael", "ninja", "mustang",
	}

	for _, common := range commonPasswords {
		if strings.Contains(lower, common) {
			return true
		}
	}

	// Check for keyboard patterns
	keyboardPatterns := []string{
		"qwerty", "asdfgh", "zxcvbn", "123456",
		"qwertyuiop", "asdfghjkl", "zxcvbnm",
	}

	for _, pattern := range keyboardPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// hasSequentialChars checks for sequential characters
func hasSequentialChars(password string) bool {
	if len(password) < 3 {
		return false
	}

	lower := strings.ToLower(password)

	// Check for sequential letters
	sequential := "abcdefghijklmnopqrstuvwxyz"
	for i := 0; i < len(lower)-2; i++ {
		if strings.Contains(sequential, lower[i:i+3]) {
			return true
		}
	}

	// Check for sequential numbers
	numSequential := "01234567890"
	for i := 0; i < len(lower)-2; i++ {
		if strings.Contains(numSequential, lower[i:i+3]) {
			return true
		}
	}

	return false
}

// hasRepeatedChars checks for repeated characters
func hasRepeatedChars(password string) bool {
	if len(password) < 3 {
		return false
	}

	// Check for 3 or more consecutive identical characters
	for i := 0; i < len(password)-2; i++ {
		if password[i] == password[i+1] && password[i] == password[i+2] {
			return true
		}
	}

	return false
}

// Entropy calculates password entropy (measure of randomness)
func Entropy(password string) float64 {
	if len(password) == 0 {
		return 0
	}

	// Count character types
	hasLower := false
	hasUpper := false
	hasNumber := false
	hasSpecial := false

	for _, r := range password {
		if unicode.IsLower(r) {
			hasLower = true
		} else if unicode.IsUpper(r) {
			hasUpper = true
		} else if unicode.IsDigit(r) {
			hasNumber = true
		} else {
			hasSpecial = true
		}
	}

	poolSize := 0
	if hasLower {
		poolSize += 26
	}
	if hasUpper {
		poolSize += 26
	}
	if hasNumber {
		poolSize += 10
	}
	if hasSpecial {
		poolSize += 32
	}

	// Calculate entropy: log2(poolSize) * length
	entropy := math.Log2(float64(poolSize)) * float64(len(password))
	return entropy
}

// StrengthScore returns a numerical score (0-100) for password strength
func StrengthScore(password string) int {
	entropy := Entropy(password)

	// Normalize to 0-100 scale
	// Maximum entropy for typical passwords is around 80 bits
	score := int((entropy / 80.0) * 100)
	if score > 100 {
		score = 100
	}

	return score
}
