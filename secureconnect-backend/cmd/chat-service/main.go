package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	intDatabase "secureconnect-backend/internal/database"
	chatHandler "secureconnect-backend/internal/handler/http/chat"
	wsHandler "secureconnect-backend/internal/handler/ws"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cassandra"
	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/internal/repository/redis"
	chatService "secureconnect-backend/internal/service/chat"
	notificationService "secureconnect-backend/internal/service/notification"
	pkgDatabase "secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/env"
	"secureconnect-backend/pkg/jwt"
	"secureconnect-backend/pkg/metrics"
)

func main() {
	// 1. Setup JWT Manager
	jwtSecret := env.GetStringFromFile("JWT_SECRET", "")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}

	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)

	// 2. Connect to Cassandra with authentication
	cassandraConfig := &intDatabase.CassandraConfig{
		Hosts:    []string{env.GetString("CASSANDRA_HOST", "localhost")},
		Keyspace: "secureconnect_ks",
		Username: env.GetStringFromFile("CASSANDRA_USER", ""),
		Password: env.GetStringFromFile("CASSANDRA_PASSWORD", ""),
		Timeout:  10 * time.Second,
	}
	cassandraDB, err := intDatabase.NewCassandraDBWithConfig(cassandraConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Cassandra: %v", err)
	}
	defer cassandraDB.Close()

	log.Println("✅ Connected to Cassandra")

	// Initialize Redis metrics before connecting to Redis
	intDatabase.InitRedisMetrics()

	// 3. Connect to Redis with degraded mode support
	redisConfig := &intDatabase.RedisConfig{
		Host:     env.GetString("REDIS_HOST", "localhost"),
		Port:     6379,
		Password: env.GetStringFromFile("REDIS_PASSWORD", ""),
		DB:       0,
		PoolSize: 10,
		Timeout:  5 * time.Second,
	}

	redisDB, err := intDatabase.NewRedisDB(redisConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisDB.Close()

	log.Println("✅ Connected to Redis")

	// Start background Redis health check
	go redisDB.StartHealthCheck(context.Background(), 10*time.Second)
	log.Println("✅ Redis health check started (10s interval)")

	// 4. Connect to CockroachDB
	cockroachConfig := &pkgDatabase.CockroachConfig{
		Host:     env.GetString("COCKROACH_HOST", "localhost"),
		Port:     env.GetInt("COCKROACH_PORT", 26257),
		User:     env.GetString("COCKROACH_USER", "root"),
		Password: env.GetStringFromFile("COCKROACH_PASSWORD", ""),
		Database: env.GetString("COCKROACH_DATABASE", "secureconnect_db"),
		SSLMode:  env.GetString("COCKROACH_SSLMODE", "disable"),
	}

	cockroachDB, err := pkgDatabase.NewCockroachDB(context.Background(), cockroachConfig)
	if err != nil {
		log.Fatalf("Failed to connect to CockroachDB: %v", err)
	}
	defer cockroachDB.Close()

	log.Println("✅ Connected to CockroachDB")

	// 5. Initialize Repositories
	messageRepo := cassandra.NewMessageRepository(cassandraDB)
	presenceRepo := redis.NewPresenceRepository(redisDB)
	userRepo := cockroach.NewUserRepository(cockroachDB.Pool)
	conversationRepo := cockroach.NewConversationRepository(cockroachDB.Pool)
	notificationRepo := cockroach.NewNotificationRepository(cockroachDB.Pool)
	// 6. Initialize Services
	redisPublisher := &chatService.RedisAdapter{Client: redisDB.Client}
	notificationSvc := notificationService.NewService(notificationRepo)
	chatSvc := chatService.NewService(messageRepo, presenceRepo, redisPublisher, notificationSvc, conversationRepo, userRepo)

	// 7. Initialize Metrics
	appMetrics := metrics.NewMetrics("chat-service")
	prometheusMiddleware := middleware.NewPrometheusMiddleware(appMetrics)

	// 8. Initialize Handlers
	chatHdlr := chatHandler.NewHandler(chatSvc)

	// 9. Initialize WebSocket Hub
	chatHub := wsHandler.NewChatHub(redisDB.Client)

	// 10. Setup Gin Router
	router := gin.New() // Don't use Default() to have full control

	// Configure trusted proxies for production
	trustedProxies := []string{}
	if env := os.Getenv("ENV"); env == "production" {
		// Production: Only trust specific domains
		trustedProxies = []string{
			"https://api.secureconnect.com",
			"https://*.secureconnect.com",
		}
	} else {
		// Development: Allow localhost and private IPs
		trustedProxies = []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
	}
	router.SetTrustedProxies(trustedProxies)

	// Apply global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORSMiddleware())
	router.Use(prometheusMiddleware.Handler())
	router.Use(middleware.NewTimeoutMiddleware(nil).Middleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "chat-service",
			"time":    time.Now().UTC(),
		})
	})

	// Metrics endpoint (for Prometheus scraping)
	router.GET("/metrics", middleware.MetricsHandler(appMetrics))

	// Revocation checker
	revocationChecker := middleware.NewRedisRevocationChecker(redisDB.Client)

	// Chat routes (all require authentication)
	v1 := router.Group("/v1")
	v1.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
	{
		// Message endpoints
		v1.POST("/messages", chatHdlr.SendMessage)
		v1.GET("/messages", chatHdlr.GetMessages)

		// Presence endpoint
		v1.POST("/presence", chatHdlr.UpdatePresence)

		// Typing indicator endpoint
		v1.POST("/typing", chatHdlr.HandleTypingIndicator)

		// WebSocket endpoint (real-time chat)
		v1.GET("/ws/chat", func(c *gin.Context) {
			chatHub.ServeWS(c, conversationRepo)
		})
	}

	// 11. Start server
	port := env.GetString("PORT", "8082")
	addr := fmt.Sprintf(":%s", port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Printf("🚀 Chat Service starting on port %s\n", port)
		log.Println("📡 WebSocket endpoint: /v1/ws/chat")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 11. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
