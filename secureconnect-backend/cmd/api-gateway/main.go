package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"secureconnect-backend/internal/database"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/pkg/env"
	"secureconnect-backend/pkg/jwt"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/metrics"
)

func main() {
	// Initialize logger with service name
	logger.InitDefault("api-gateway")
	defer logger.Sync()

	// Initialize Redis metrics before connecting to Redis
	database.InitRedisMetrics()

	// 1. Connect to Redis (for rate limiting)
	redisConfig := &database.RedisConfig{
		Host:     env.GetString("REDIS_HOST", "localhost"),
		Port:     6379,
		Password: env.GetStringFromFile("REDIS_PASSWORD", ""),
		DB:       0,
		PoolSize: 10,
		Timeout:  5 * time.Second,
	}

	redisDB, err := database.NewRedisDB(redisConfig)
	if err != nil {
		logger.Fatal("Failed to connect to Redis")
	}
	defer redisDB.Close()

	logger.Info("API Gateway connected to Redis")

	// Start background Redis health check
	go redisDB.StartHealthCheck(context.Background(), 10*time.Second)
	logger.Info("Redis health check started (10s interval)")

	// 2. Setup JWT Manager (for optional auth in gateway)
	jwtSecret := env.GetStringFromFile("JWT_SECRET", "")
	if jwtSecret == "" {
		logger.Fatal("JWT_SECRET environment variable is required")
	}
	if len(jwtSecret) < 32 {
		logger.Fatal("JWT_SECRET must be at least 32 characters")
	}
	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)

	// 3. Setup advanced rate limiter with per-endpoint configuration and degraded mode support
	// DEGRADED MODE: Enable in-memory fallback when Redis is unavailable
	rateLimiter := middleware.NewRateLimiterWithFallback(middleware.RateLimiterConfig{
		RedisClient:            redisDB,
		RequestsPerMin:         100,
		Window:                 time.Minute,
		EnableInMemoryFallback: true, // Enable in-memory rate limiting when Redis is degraded
	})

	// 4. Initialize Metrics
	appMetrics := metrics.NewMetrics("api-gateway")
	prometheusMiddleware := middleware.NewPrometheusMiddleware(appMetrics)

	// 5. Setup Gin router
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

	// 6. Apply global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORSMiddleware())
	router.Use(rateLimiter.Middleware())
	router.Use(prometheusMiddleware.Handler())
	router.Use(middleware.NewTimeoutMiddleware(nil).Middleware())

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

	// 7. Metrics endpoint (for Prometheus scraping - no auth required)
	router.GET("/metrics", middleware.MetricsHandler(appMetrics))

	// 8. Swagger documentation
	router.GET("/swagger", func(c *gin.Context) {
		c.File("./api/swagger/openapi.yaml")
	})

	// 10. API version 1 routes
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

	// 11. Start server
	port := env.GetString("PORT", "8080")
	addr := fmt.Sprintf(":%s", port)

	logger.Info("API Gateway starting",
		zap.String("port", port),
	)
	logger.Info("Routes configured",
		zap.String("auth", "/v1/auth/*"),
		zap.String("users", "/v1/users/*"),
		zap.String("conversations", "/v1/conversations/*"),
		zap.String("keys", "/v1/keys/*"),
		zap.String("chat", "/v1/messages, /v1/ws/chat"),
		zap.String("calls", "/v1/calls/*, /v1/ws/signaling"),
		zap.String("storage", "/v1/storage/*"),
	)

	if err := router.Run(addr); err != nil {
		logger.Fatal("Failed to start API Gateway")
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

		// Create reverse proxy with timeout
		proxy := httputil.NewSingleHostReverseProxy(remote)
		proxy.Transport = &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}

		// Modify request
		proxy.Director = func(req *http.Request) {
			req.Header = c.Request.Header
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = c.Request.URL.Path
			req.URL.RawQuery = c.Request.URL.RawQuery
		}

		// Handle errors - write directly to response writer
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Error("Proxy error",
				zap.String("service", serviceName),
				zap.Error(err),
			)
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"error":"Service unavailable","service":"` + serviceName + `"}`))
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
