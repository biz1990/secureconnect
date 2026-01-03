package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	
	"github.com/gin-gonic/gin"
	
	authHandler "secureconnect-backend/internal/handler/http/auth"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/internal/repository/redis"
	authService "secureconnect-backend/internal/service/auth"
	"secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/jwt"
)

func main() {
	// Initialize context
	ctx := context.Background()
	
	// 1. Setup JWT Manager
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super-secret-key-change-in-production"
	}
	
	jwtManager := jwt.NewJWTManager(
		jwtSecret,
		15*time.Minute, // Access token: 15 minutes
		30*24*time.Hour, // Refresh token: 30 days
	)
	
	// 2. Connect to CockroachDB
	crdbConfig := &database.CockroachConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     26257,
		User:     getEnv("DB_USER", "root"),
		Password: getEnv("DB_PASSWORD", ""),
		Database: getEnv("DB_NAME", "secureconnect_poc"),
		SSLMode:  "disable", // insecure for dev
	}
	
	crdb, err := database.NewCockroachDB(ctx, crdbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to CockroachDB: %v", err)
	}
	defer crdb.Close()
	
	log.Println("✅ Connected to CockroachDB")
	
	// 3. Connect to Redis
	redisConfig := &database.RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     6379,
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
		PoolSize: 10,
		Timeout:  5 * time.Second,
	}
	
	redisDB, err := database.NewRedisDB(redisConfig)
	if err != nil{
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisDB.Close()
	
	log.Println("✅ Connected to Redis")
	
	// 4. Initialize Repositories
	userRepo := cockroach.NewUserRepository(crdb.Pool)
	directoryRepo := redis.NewDirectoryRepository(redisDB.Client)
	sessionRepo := redis.NewSessionRepository(redisDB.Client)
	
	// 5. Initialize Services
	authSvc := authService.NewService(userRepo, directoryRepo, sessionRepo, jwtManager)
	
	// 6. Initialize Handlers
	authHdlr := authHandler.NewHandler(authSvc)
	
	// 7. Setup Gin Router
	router := gin.Default()
	
	// Apply middleware
	router.Use(middleware.CORSMiddleware())
	
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "auth-service",
			"time":    time.Now().UTC(),
		})
	})
	
	// Auth routes (public, no authentication required)
	v1 := router.Group("/v1/auth")
	{
		v1.POST("/register", authHdlr.Register)
		v1.POST("/login", authHdlr.Login)
		v1.POST("/refresh", authHdlr.RefreshToken)
		
		// Protected routes (require authentication)
		authenticated := v1.Group("")
		authenticated.Use(middleware.AuthMiddleware(jwtManager))
		{
			authenticated.POST("/logout", authHdlr.Logout)
			authenticated.GET("/profile", authHdlr.GetProfile)
		}
	}
	
	// 8. Start server
	port := getEnv("PORT", "8081")
	addr := fmt.Sprintf(":%s", port)
	
	log.Printf("🚀 Auth Service starting on port %s\n", port)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Helper function to get environment variable with default
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}