package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	
	"github.com/gin-gonic/gin"
	
	storageHandler "secureconnect-backend/internal/handler/http/storage"
	"secureconnect-backend/internal/middleware"
	storageService "secureconnect-backend/internal/service/storage"
	"secureconnect-backend/pkg/jwt"
)

func main() {
	// 1. Setup JWT Manager
	jwtSecret := getEnv("JWT_SECRET", "super-secret-key-change-in-production")
	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)
	
	// 2. Setup MinIO Storage Service
	minioEndpoint := getEnv("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := getEnv("MINIO_ACCESS_KEY", "minioadmin")
	minioSecretKey := getEnv("MINIO_SECRET_KEY", "minioadmin")
	minioBucket := getEnv("MINIO_BUCKET", "secureconnect-files")
	
	storageSvc, err := storageService.NewService(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket)
	if err != nil {
		log.Fatalf("Failed to initialize storage service: %v", err)
	}
	
	log.Println("✅ Connected to MinIO")
	
	// 3. Initialize Handlers
	storageHdlr := storageHandler.NewHandler(storageSvc)
	
	// 4. Setup Gin Router
	router := gin.Default()
	router.Use(middleware.CORSMiddleware())
	
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "storage-service",
			"time":    time.Now().UTC(),
		})
	})
	
	// Storage routes (all require authentication)
	v1 := router.Group("/v1/storage")
	v1.Use(middleware.AuthMiddleware(jwtManager))
	{
		v1.POST("/upload-url", storageHdlr.GenerateUploadURL)
		v1.GET("/download-url/:file_id", storageHdlr.GenerateDownloadURL)
		v1.DELETE("/files/:file_id", storageHdlr.DeleteFile)
		
		// Placeholder for upload complete and quota
		v1.POST("/upload-complete", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Not implemented yet"})
		})
		v1.GET("/quota", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Not implemented yet"})
		})
	}
	
	// 5. Start server
	port := getEnv("PORT", "8084")
	addr := fmt.Sprintf(":%s", port)
	
	log.Printf("🚀 Storage Service starting on port %s\n", port)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
