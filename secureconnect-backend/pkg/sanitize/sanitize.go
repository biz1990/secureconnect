package sanitize

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// SanitizeString removes potentially dangerous characters and HTML tags
func SanitizeString(input string) string {
	// Remove HTML tags
	input = html.EscapeString(input)

	// Remove potential SQL injection patterns
	input = strings.ReplaceAll(input, "'", "''")
	input = strings.ReplaceAll(input, "\"", "\"\"")
	input = strings.ReplaceAll(input, ";", "")
	input = strings.ReplaceAll(input, "--", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	return input
}

// SanitizeEmail sanitizes email input
func SanitizeEmail(email string) string {
	// Trim whitespace
	email = strings.TrimSpace(email)
	// Convert to lowercase
	email = strings.ToLower(email)
	// Remove potentially dangerous characters
	email = regexp.MustCompile(`[<>;\\]`).ReplaceAllString(email, "")
	return email
}

// SanitizeUsername sanitizes username input
func SanitizeUsername(username string) string {
	// Trim whitespace
	username = strings.TrimSpace(username)
	// Remove special characters except alphanumeric, underscore, hyphen, and dot
	reg := regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	username = reg.ReplaceAllString(username, "")
	return username
}

// SanitizeFilename sanitizes filename input
func SanitizeFilename(filename string) string {
	// Trim whitespace
	filename = strings.TrimSpace(filename)
	// Remove path traversal attempts
	filename = strings.ReplaceAll(filename, "../", "")
	filename = strings.ReplaceAll(filename, "./", "")
	filename = strings.ReplaceAll(filename, "..\\", "")
	filename = strings.ReplaceAll(filename, ".\\", "")
	// Remove null bytes and control characters
	reg := regexp.MustCompile(`[\x00-\x1f\x7f]`)
	filename = reg.ReplaceAllString(filename, "")
	return filename
}

// SanitizePhoneNumber sanitizes phone number input
func SanitizePhoneNumber(phone string) string {
	// Remove all non-digit characters
	reg := regexp.MustCompile(`[^\d]`)
	phone = reg.ReplaceAllString(phone, "")
	return phone
}

// SanitizeURL sanitizes URL input
func SanitizeURL(url string) string {
	// Trim whitespace
	url = strings.TrimSpace(url)
	// Remove potentially dangerous characters
	url = regexp.MustCompile(`[<>;\\]`).ReplaceAllString(url, "")
	return url
}

// ValidateStringLength checks if string length is within bounds
func ValidateStringLength(input string, minLen, maxLen int) bool {
	if len(input) < minLen {
		return false
	}
	if len(input) > maxLen {
		return false
	}
	return true
}

// ValidateEmailFormat checks if email format is valid
func ValidateEmailFormat(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// ValidateUsernameFormat checks if username format is valid
func ValidateUsernameFormat(username string) bool {
	// Username should be 3-30 characters, alphanumeric, underscore, hyphen only
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)
	return usernameRegex.MatchString(username)
}

// SanitizeHTML removes all HTML tags
func SanitizeHTML(input string) string {
	// Remove script tags
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	input = scriptRegex.ReplaceAllString(input, "")

	// Remove style tags
	styleRegex := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	input = styleRegex.ReplaceAllString(input, "")

	// Remove other HTML tags
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	input = htmlRegex.ReplaceAllString(input, "")

	return input
}

// StripControlCharacters removes control characters from string
func StripControlCharacters(input string) string {
	var result strings.Builder
	for _, r := range input {
		if !unicode.IsControl(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// SanitizeSQLInput removes SQL injection patterns
func SanitizeSQLInput(input string) string {
	// Common SQL injection patterns
	patterns := []string{
		"'",
		"\"",
		";",
		"--",
		"/*",
		"*/",
		"xp_",
		"union",
		"select",
		"insert",
		"update",
		"delete",
		"drop",
		"exec",
		"eval",
	}

	lowerInput := strings.ToLower(input)
	for _, pattern := range patterns {
		if strings.Contains(lowerInput, pattern) {
			// Replace with safe character
			input = strings.ReplaceAll(input, strings.ToUpper(pattern), "")
		}
	}

	return input
}
