package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	
	"github.com/gin-gonic/gin"
	
	videoHandler "secureconnect-backend/internal/handler/http/video"
	wsHandler "secureconnect-backend/internal/handler/ws"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	videoService "secureconnect-backend/internal/service/video"
	"secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/jwt"
)

func main() {
	ctx := context.Background()
	_ = ctx // Mark as used
	
	// 1. Setup JWT Manager
	jwtSecret := getEnv("JWT_SECRET", "super-secret-key-change-in-production")
	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)
	
	// 2. Connect to CockroachDB for call logs
	dbConfig := &database.CockroachDBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     26257,
		User:     getEnv("DB_USER", "root"),
		Password: getEnv("DB_PASSWORD", ""),
		Database: getEnv("DB_NAME", "secureconnect"),
		SSLMode:  "disable",
	}
	
	db, err := database.NewCockroachDB(dbConfig)
	if err != nil {
		log.Printf("Warning: Failed to connect to CockroachDB: %v", err)
		log.Println("Running in limited mode without call logs persistence")
	}
	
	var callRepo *cockroach.CallRepository
	if db != nil {
		defer db.Close()
		callRepo = cockroach.NewCallRepository(db.Pool)
		log.Println("✅ Connected to CockroachDB")
	}
	
	// 3. Initialize Video Service
	videoSvc := videoService.NewService(callRepo)
	
	// 4. Initialize Handlers
	videoHdlr := videoHandler.NewHandler(videoSvc)
	
	// 5. Initialize WebRTC Signaling Hub
	signalingHub := wsHandler.NewSignalingHub()
	
	// 6. Setup Gin Router
	router := gin.Default()
	router.Use(middleware.CORSMiddleware())
	
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "video-service",
			"time":    time.Now().UTC(),
		})
	})
	
	// Video routes (all require authentication)
	v1 := router.Group("/v1/calls")
	v1.Use(middleware.AuthMiddleware(jwtManager))
	{
		// Call management endpoints
		v1.POST("/initiate", videoHdlr.InitiateCall)
		v1.POST("/:id/end", videoHdlr.EndCall)
		v1.POST("/:id/join", videoHdlr.JoinCall)
		v1.GET("/:id", videoHdlr.GetCallStatus)
		
		// WebSocket endpoint for WebRTC signaling
		v1.GET("/ws/signaling", signalingHub.ServeWS)
	}
	
	// 7. Start server
	port := getEnv("PORT", "8083")
	addr := fmt.Sprintf(":%s", port)
	
	log.Printf("🚀 Video Service starting on port %s\n", port)
	log.Println("📡 WebRTC Signaling: /v1/calls/ws/signaling")
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
