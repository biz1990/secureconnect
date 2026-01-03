package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	
	"github.com/gin-gonic/gin"
	
	chatHandler "secureconnect-backend/internal/handler/http/chat"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cassandra"
	"secureconnect-backend/internal/repository/redis"
	chatService "secureconnect-backend/internal/service/chat"
	"secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/jwt"
)

func main() {
	ctx := context.Background()
	
	// 1. Setup JWT Manager
	jwtSecret := getEnv("JWT_SECRET", "super-secret-key-change-in-production")
	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)
	
	// 2. Connect to Cassandra
	cassandraConfig := &database.CassandraConfig{
		Hosts:    []string{getEnv("CASSANDRA_HOST", "localhost")},
		Keyspace: "secureconnect_ks",
		Timeout:  10 * time.Second,
	}
	
	cassandraDB, err := database.NewCassandraDB(cassandraConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Cassandra: %v", err)
	}
	defer cassandraDB.Close()
	
	log.Println("✅ Connected to Cassandra")
	
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
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisDB.Close()
	
	log.Println("✅ Connected to Redis")
	
	// 4. Initialize Repositories
	messageRepo := cassandra.NewMessageRepository(cassandraDB.Session)
	presenceRepo := redis.NewPresenceRepository(redisDB.Client)
	
	// 5. Initialize Services
	chatSvc := chatService.NewService(messageRepo, presenceRepo)
	
	// 6. Initialize Handlers
	chatHdlr := chatHandler.NewHandler(chatSvc)
	
	// 7. Setup Gin Router
	router := gin.Default()
	router.Use(middleware.CORSMiddleware())
	
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "chat-service",
			"time":    time.Now().UTC(),
		})
	})
	
	// Chat routes (all require authentication)
	v1 := router.Group("/v1")
	v1.Use(middleware.AuthMiddleware(jwtManager))
	{
		// Message endpoints
		v1.POST("/messages", chatHdlr.SendMessage)
		v1.GET("/messages", chatHdlr.GetMessages)
		
		// Presence endpoint
		v1.POST("/presence", chatHdlr.UpdatePresence)
		
		// WebSocket endpoint (placeholder)
		v1.GET("/ws/chat", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "WebSocket not implemented yet"})
		})
	}
	
	// 8. Start server
	port := getEnv("PORT", "8082")
	addr := fmt.Sprintf(":%s", port)
	
	log.Printf("🚀 Chat Service starting on port %s\n", port)
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
