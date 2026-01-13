package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	videoHandler "secureconnect-backend/internal/handler/http/video"
	wsHandler "secureconnect-backend/internal/handler/ws"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	redisRepo "secureconnect-backend/internal/repository/redis"
	videoService "secureconnect-backend/internal/service/video"
	"secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/env"
	"secureconnect-backend/pkg/jwt"
	"secureconnect-backend/pkg/push"
)

func main() {
	// Create context for database operations
	ctx := context.Background()

	// 1. Setup JWT Manager
	jwtSecret := env.GetString("JWT_SECRET", "")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}

	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)

	// 2. Connect to CockroachDB for call logs
	dbConfig := &database.CockroachConfig{
		Host:     env.GetString("DB_HOST", "localhost"),
		Port:     26257,
		User:     env.GetString("DB_USER", "root"),
		Password: env.GetString("DB_PASSWORD", ""),
		Database: env.GetString("DB_NAME", "secureconnect"),
		SSLMode:  "disable",
	}

	db, err := database.NewCockroachDB(ctx, dbConfig)
	if err != nil {
		log.Printf("Warning: Failed to connect to CockroachDB: %v", err)
		log.Println("Running in limited mode without call logs persistence")
	}

	var callRepo *cockroach.CallRepository
	var conversationRepo *cockroach.ConversationRepository
	var userRepo *cockroach.UserRepository
	if db != nil {
		defer db.Close()
		callRepo = cockroach.NewCallRepository(db.Pool)
		conversationRepo = cockroach.NewConversationRepository(db.Pool)
		userRepo = cockroach.NewUserRepository(db.Pool)
		log.Println("✅ Connected to CockroachDB")
	}

	// 3. Initialize Redis
	redisHost := env.GetString("REDIS_HOST", "localhost")
	redisPort := env.GetString("REDIS_PORT", "6379")
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: env.GetString("REDIS_PASSWORD", ""),
		DB:       0,
	})
	defer redisClient.Close()

	// Check Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
	} else {
		log.Println("✅ Connected to Redis")
	}

	// 4. Initialize Push Service
	pushTokenRepo := redisRepo.NewPushTokenRepository(redisClient)
	pushProvider := &push.MockProvider{} // Use mock for development
	pushSvc := push.NewService(pushProvider, pushTokenRepo)

	// 5. Initialize Video Service
	videoSvc := videoService.NewService(callRepo, conversationRepo, userRepo, pushSvc)

	// 6. Initialize Handlers
	videoHdlr := videoHandler.NewHandler(videoSvc)

	// 7. Initialize WebRTC Signaling Hub
	signalingHub := wsHandler.NewSignalingHub(redisClient)

	// 8. Setup Gin Router
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

	// Revocation checker
	revocationChecker := middleware.NewRedisRevocationChecker(redisClient)

	// Video routes (all require authentication)
	v1 := router.Group("/v1/calls")
	v1.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
	{
		// Call management endpoints
		v1.POST("/initiate", videoHdlr.InitiateCall)
		v1.POST("/:id/end", videoHdlr.EndCall)
		v1.POST("/:id/join", videoHdlr.JoinCall)
		v1.GET("/:id", videoHdlr.GetCallStatus)

		// WebSocket endpoint for WebRTC signaling
		v1.GET("/ws/signaling", signalingHub.ServeWS)
	}

	// 8. Start server
	port := env.GetString("PORT", "8083")
	addr := fmt.Sprintf(":%s", port)

	log.Printf("🚀 Video Service starting on port %s\n", port)
	log.Println("📡 WebRTC Signaling: /v1/calls/ws/signaling")
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
