package logger

import (
	"log"
)

// Info ghi log thông thường
func Info(msg string, fields ...string) {
	log.Printf("[INFO] %s %v", msg, fields)
}

// Error ghi log lỗi
func Error(msg string, err string) {
	log.Printf("[ERROR] %s: %s", msg, err)
}
