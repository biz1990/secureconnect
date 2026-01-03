package main

import (
	"context"
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
	"secureconnect-backend/pkg/jwt"
)

func main() {
	// Initialize context
	ctx := context.Background()
	
	// 1. Connect to Redis (for rate limiting)
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
	
	log.Println("✅ API Gateway connected to Redis")
	
	// 2. Setup JWT Manager (for optional auth in gateway)
	jwtSecret := getEnv("JWT_SECRET", "super-secret-key-change-in-production")
	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)
	
	// 3. Setup rate limiter
	rateLimiter := middleware.NewRateLimiter(redisDB.Client, 100) // 100 requests/minute
	
	// 4. Setup Gin router
	router := gin.New() // Don't use Default() to have full control
	
	// 5. Apply global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORSMiddleware())
	router.Use(rateLimiter.Middleware())
	
	// 6. Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "api-gateway",
			"timestamp": time.Now().UTC(),
		})
	})
	
	// 7. API version 1 routes
	v1 := router.Group("/v1")
	{
		// Auth Service routes (public)
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", proxyToService("auth-service", 8081))
			authGroup.POST("/login", proxyToService("auth-service", 8081))
			authGroup.POST("/refresh", proxyToService("auth-service", 8081))
			
			// Protected auth routes
			authProtected := authGroup.Group("")
			authProtected.Use(middleware.AuthMiddleware(jwtManager))
			{
				authProtected.POST("/logout", proxyToService("auth-service", 8081))
				authProtected.GET("/profile", proxyToService("auth-service", 8081))
			}
		}
		
		// Keys Service routes (E2EE) - all require authentication
		keysGroup := v1.Group("/keys")
		keysGroup.Use(middleware.AuthMiddleware(jwtManager))
		{
			keysGroup.POST("/upload", proxyToService("auth-service", 8081))
			keysGroup.GET("/:user_id", proxyToService("auth-service", 8081))
			keysGroup.POST("/rotate", proxyToService("auth-service", 8081))
		}
		
		// Chat Service routes - require authentication
		chatGroup := v1.Group("/messages")
		chatGroup.Use(middleware.AuthMiddleware(jwtManager))
		{
			chatGroup.POST("", proxyToService("chat-service", 8082))
			chatGroup.GET("", proxyToService("chat-service", 8082))
		}
		
		// WebSocket chat - will be handled by chat service directly
		v1.GET("/ws/chat", proxyToService("chat-service", 8082))
		
		// Video/Call Service routes - require authentication
		callsGroup := v1.Group("/calls")
		callsGroup.Use(middleware.AuthMiddleware(jwtManager))
		{
			callsGroup.POST("/initiate", proxyToService("video-service", 8083))
			callsGroup.POST("/:id/end", proxyToService("video-service", 8083))
		}
		
		// WebSocket signaling
		v1.GET("/ws/signaling", proxyToService("video-service", 8083))
		
		// Storage Service routes - require authentication
		storageGroup := v1.Group("/storage")
		storageGroup.Use(middleware.AuthMiddleware(jwtManager))
		{
			storageGroup.POST("/upload-url", proxyToService("storage-service", 8084))
			storageGroup.POST("/upload-complete", proxyToService("storage-service", 8084))
			storageGroup.GET("/download-url/:file_id", proxyToService("storage-service", 8084))
			storageGroup.DELETE("/files/:file_id", proxyToService("storage-service", 8084))
			storageGroup.GET("/quota", proxyToService("storage-service", 8084))
		}
	}
	
	// 8. Start server
	port := getEnv("PORT", "8080")
	addr := fmt.Sprintf(":%s", port)
	
	log.Printf("🚀 API Gateway starting on port %s\n", port)
	log.Println("📍 Routes:")
	log.Println("   - Auth: /v1/auth/*")
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
	// In Docker environment, use service name as hostname
	// In local dev, use localhost
	if os.Getenv("ENV") == "production" {
		return serviceName
	}
	return "localhost"
}

// Helper function
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}