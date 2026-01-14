package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/env"
	"secureconnect-backend/pkg/jwt"
)

func main() {
	// Initialize context

	// 1. Connect to Redis (for rate limiting)
	redisConfig := &database.RedisConfig{
		Host:     env.GetString("REDIS_HOST", "localhost"),
		Port:     6379,
		Password: env.GetString("REDIS_PASSWORD", ""),
		DB:       0,
		PoolSize: 10,
		Timeout:  5 * time.Second,
	}

	redisDB, err := database.NewRedisDB(redisConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisDB.Close()

	log.Println("✅ API Gateway connected to Redis")

	// 2. Setup JWT Manager (for optional auth in gateway)
	jwtSecret := env.GetString("JWT_SECRET", "")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}
	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)

	// 3. Setup rate limiter
	rateLimiter := middleware.NewRateLimiter(redisDB.Client, 100, time.Minute) // 100 requests/minute

	// 4. Setup Gin router
	router := gin.New() // Don't use Default() to have full control

	// 5. Apply global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORSMiddleware())
	router.Use(rateLimiter.Middleware())

	// Revocation checker
	revocationChecker := middleware.NewRedisRevocationChecker(redisDB.Client)

	// 6. Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "api-gateway",
			"timestamp": time.Now().UTC(),
		})
	})

	// 7. Swagger documentation
	router.GET("/swagger", func(c *gin.Context) {
		c.File("./api/swagger/openapi.yaml")
	})

	// 7. API version 1 routes
	v1 := router.Group("/v1")
	{
		// Auth Service routes (public)
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", proxyToService("auth-service", 8080))
			authGroup.POST("/login", proxyToService("auth-service", 8080))
			authGroup.POST("/refresh", proxyToService("auth-service", 8080))

			// Protected auth routes
			authProtected := authGroup.Group("")
			authProtected.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
			{
				authProtected.POST("/logout", proxyToService("auth-service", 8080))
				authProtected.GET("/profile", proxyToService("auth-service", 8080))
			}
		}

		// User Management routes (all require authentication)
		usersGroup := v1.Group("/users")
		usersGroup.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
			// Current user profile
			usersGroup.GET("/me", proxyToService("auth-service", 8080))
			usersGroup.PATCH("/me", proxyToService("auth-service", 8080))
			usersGroup.POST("/me/password", proxyToService("auth-service", 8080))
			usersGroup.POST("/me/email", proxyToService("auth-service", 8080))
			usersGroup.POST("/me/email/verify", proxyToService("auth-service", 8080))
			usersGroup.DELETE("/me", proxyToService("auth-service", 8080))

			// Blocked users
			usersGroup.GET("/me/blocked", proxyToService("auth-service", 8080))
			usersGroup.POST("/:id/block", proxyToService("auth-service", 8080))
			usersGroup.DELETE("/:id/block", proxyToService("auth-service", 8080))

			// Friends
			usersGroup.GET("/me/friends", proxyToService("auth-service", 8080))
			usersGroup.POST("/:id/friend", proxyToService("auth-service", 8080))
			usersGroup.POST("/me/friends/:id/accept", proxyToService("auth-service", 8080))
			usersGroup.DELETE("/me/friends/:id/reject", proxyToService("auth-service", 8080))
			usersGroup.DELETE("/me/friends/:id", proxyToService("auth-service", 8080))
		}

		// Conversation Management routes - all require authentication
		conversationsGroup := v1.Group("/conversations")
		conversationsGroup.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
			conversationsGroup.POST("", proxyToService("auth-service", 8080))
			conversationsGroup.GET("", proxyToService("auth-service", 8080))
			conversationsGroup.GET("/:id", proxyToService("auth-service", 8080))
			conversationsGroup.PATCH("/:id", proxyToService("auth-service", 8080))
			conversationsGroup.DELETE("/:id", proxyToService("auth-service", 8080))
			conversationsGroup.PUT("/:id/settings", proxyToService("auth-service", 8080))
			conversationsGroup.POST("/:id/participants", proxyToService("auth-service", 8080))
			conversationsGroup.GET("/:id/participants", proxyToService("auth-service", 8080))
			conversationsGroup.DELETE("/:id/participants/:userId", proxyToService("auth-service", 8080))
		}

		// Keys Service routes (E2EE) - all require authentication
		keysGroup := v1.Group("/keys")
		keysGroup.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
			keysGroup.POST("/upload", proxyToService("auth-service", 8080))
			keysGroup.GET("/:user_id", proxyToService("auth-service", 8080))
			keysGroup.POST("/rotate", proxyToService("auth-service", 8080))
		}

		// Chat Service routes - require authentication
		chatGroup := v1.Group("/messages")
		chatGroup.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
			chatGroup.POST("", proxyToService("chat-service", 8082))
			chatGroup.GET("", proxyToService("chat-service", 8082))
		}

		// Presence endpoint - require authentication
		presenceGroup := v1.Group("/presence")
		presenceGroup.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
			presenceGroup.POST("", proxyToService("chat-service", 8082))
		}

		// WebSocket chat - will be handled by chat service directly
		v1.GET("/ws/chat", proxyToService("chat-service", 8082))

		// Video/Call Service routes - require authentication
		callsGroup := v1.Group("/calls")
		callsGroup.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
			callsGroup.POST("/initiate", proxyToService("video-service", 8083))
			callsGroup.POST("/:id/end", proxyToService("video-service", 8083))
		}

		// WebSocket signaling
		v1.GET("/ws/signaling", proxyToService("video-service", 8083))

		// Storage Service routes - require authentication
		storageGroup := v1.Group("/storage")
		storageGroup.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
		{
			storageGroup.POST("/upload-url", proxyToService("storage-service", 8080))
			storageGroup.POST("/upload-complete", proxyToService("storage-service", 8080))
			storageGroup.GET("/download-url/:file_id", proxyToService("storage-service", 8080))
			storageGroup.DELETE("/files/:file_id", proxyToService("storage-service", 8080))
			storageGroup.GET("/quota", proxyToService("storage-service", 8080))
		}
	}

	// 8. Start server
	port := env.GetString("PORT", "8080")
	addr := fmt.Sprintf(":%s", port)

	log.Printf("🚀 API Gateway starting on port %s\n", port)
	log.Println("📍 Routes:")
	log.Println("   - Auth: /v1/auth/*")
	log.Println("   - Users: /v1/users/*")
	log.Println("   - Conversations: /v1/conversations/*")
	log.Println("   - Keys: /v1/keys/*")
	log.Println("   - Chat: /v1/messages, /v1/ws/chat")
	log.Println("   - Calls: /v1/calls/*, /v1/ws/signaling")
	log.Println("   - Storage: /v1/storage/*")

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start API Gateway: %v", err)
	}
}

// proxyToService creates a reverse proxy handler for a microservice
func proxyToService(serviceName string, port int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build target URL
		targetURL := fmt.Sprintf("http://%s:%d", getServiceHost(serviceName), port)

		// Parse URL
		remote, err := url.Parse(targetURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Service unavailable"})
			return
		}

		// Create reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(remote)

		// Modify request
		proxy.Director = func(req *http.Request) {
			req.Header = c.Request.Header
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = c.Request.URL.Path
			req.URL.RawQuery = c.Request.URL.RawQuery
		}

		// Handle errors
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error for %s: %v", serviceName, err)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "Service unavailable",
				"service": serviceName,
			})
		}

		// Serve
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// getServiceHost returns service hostname (Docker DNS or localhost)
func getServiceHost(serviceName string) string {
	// In Docker environment (production, local, staging), use service name as hostname
	// Only use localhost for direct local development outside Docker
	env := os.Getenv("ENV")
	if env == "production" || env == "local" || env == "staging" {
		return serviceName
	}
	return "localhost"
}
