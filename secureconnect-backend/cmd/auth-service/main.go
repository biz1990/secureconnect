package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"secureconnect-backend/internal/database"
	authHandler "secureconnect-backend/internal/handler/http/auth"
	"secureconnect-backend/internal/handler/http/conversation"
	userHandler "secureconnect-backend/internal/handler/http/user"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/internal/repository/redis"
	authService "secureconnect-backend/internal/service/auth"
	conversationService "secureconnect-backend/internal/service/conversation"
	userService "secureconnect-backend/internal/service/user"
	"secureconnect-backend/pkg/config"
	"secureconnect-backend/pkg/constants"
	pkgDatabase "secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/email"
	"secureconnect-backend/pkg/jwt"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/metrics"
)

func main() {
	// Initialize logger with service name
	logger.InitDefault("auth-service")
	defer logger.Sync()

	// Initialize context
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config")
	}

	// Validate JWT secret in production
	if cfg.Server.Environment == "production" {
		if cfg.JWT.Secret == "" {
			logger.Fatal("JWT_SECRET environment variable is required in production")
		}
		if len(cfg.JWT.Secret) < 32 {
			logger.Fatal("JWT_SECRET must be at least 32 characters")
		}
		// Validate SMTP configuration in production
		if cfg.SMTP.Username == "" || cfg.SMTP.Password == "" {
			logger.Fatal("SMTP_USERNAME and SMTP_PASSWORD environment variables are required in production")
		}
	}

	// 1. Setup JWT Manager
	jwtManager := jwt.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenExpiry,
		cfg.JWT.RefreshTokenExpiry,
	)

	// 2. Connect to CockroachDB
	cockroachDB, err := pkgDatabase.NewCockroachDB(ctx, &pkgDatabase.CockroachConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		logger.Fatal("Failed to connect to CockroachDB")
	}
	defer cockroachDB.Close()

	logger.Info("Connected to CockroachDB")

	// Initialize Redis metrics before connecting to Redis
	database.InitRedisMetrics()

	// 3. Connect to Redis with degraded mode support
	redisDB, err := database.NewRedisDB(&database.RedisConfig{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
		Timeout:  cfg.Redis.Timeout,
	})
	if err != nil {
		logger.Fatal("Failed to connect to Redis")
	}
	defer redisDB.Close()

	logger.Info("Connected to Redis")

	// Start background Redis health check
	go redisDB.StartHealthCheck(ctx, 10*time.Second)
	logger.Info("Redis health check started (10s interval)")

	// 4. Initialize Repositories
	userRepo := cockroach.NewUserRepository(cockroachDB.Pool)
	blockedUserRepo := cockroach.NewBlockedUserRepository(cockroachDB.Pool)
	emailVerificationRepo := cockroach.NewEmailVerificationRepository(cockroachDB.Pool)
	conversationRepo := cockroach.NewConversationRepository(cockroachDB.Pool)
	directoryRepo := redis.NewDirectoryRepository(redisDB.Client)
	sessionRepo := redis.NewSessionRepository(redisDB)
	presenceRepo := redis.NewPresenceRepository(redisDB)

	// 5. Initialize Services
	// Create email service (using SMTP in production, MockSender in development)
	var emailSender email.Sender

	// Check if SMTP credentials are configured
	smtpConfigured := cfg.SMTP.Username != "" && cfg.SMTP.Password != ""

	if smtpConfigured {
		// Use real SMTP sender
		emailSender = email.NewSMTPSender(&email.SMTPConfig{
			Host:     cfg.SMTP.Host,
			Port:     cfg.SMTP.Port,
			Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password,
			From:     cfg.SMTP.From,
		})
		logger.Info("Using SMTP email provider")
	} else {
		// Development: Use mock sender
		if cfg.Server.Environment == "production" {
			logger.Fatal("SMTP credentials are required in production mode")
		}
		emailSender = &email.MockSender{}
		logger.Info("Using Mock email sender (development)")
	}
	emailSvc := email.NewService(emailSender)

	authSvc := authService.NewService(userRepo, directoryRepo, sessionRepo, presenceRepo, emailVerificationRepo, emailSvc, jwtManager)

	// Note: emailSvc now initialized above before authSvc

	userSvc := userService.NewService(userRepo, blockedUserRepo, emailVerificationRepo, emailSvc)
	conversationSvc := conversationService.NewService(conversationRepo, userRepo)

	// 6. Initialize Metrics
	appMetrics := metrics.NewMetrics("auth-service")
	prometheusMiddleware := middleware.NewPrometheusMiddleware(appMetrics)

	// 7. Initialize Handlers
	authHdlr := authHandler.NewHandler(authSvc)
	userHdlr := userHandler.NewHandler(userSvc)
	conversationHdlr := conversation.NewHandler(conversationSvc)

	// 8. Setup Gin Router
	router := gin.New() // Don't use Default() to have full control

	// Configure trusted proxies for production
	trustedProxies := []string{}
	if cfg.Server.Environment == "production" {
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

	// Apply middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORSMiddleware())
	router.Use(prometheusMiddleware.Handler())
	router.Use(middleware.NewTimeoutMiddleware(nil).Middleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "auth-service",
			"time":    time.Now().UTC(),
		})
	})

	// Metrics endpoint (for Prometheus scraping - no auth required)
	router.GET("/metrics", middleware.MetricsHandler(appMetrics))

	// API version 1 routes
	v1 := router.Group("/v1")
	{
		// Auth routes (public, no authentication required)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHdlr.Register)
			auth.POST("/login", authHdlr.Login)
			auth.POST("/refresh", authHdlr.RefreshToken)
			auth.POST("/password-reset/request", authHdlr.RequestPasswordReset)
			auth.POST("/password-reset/confirm", authHdlr.ResetPassword)

			// Protected routes (require authentication)
			authenticated := auth.Group("")
			authenticated.Use(middleware.AuthMiddleware(jwtManager, authSvc))
			{
				authenticated.POST("/logout", authHdlr.Logout)
				authenticated.GET("/profile", authHdlr.GetProfile)
			}
		}

		// User Management routes (all require authentication)
		users := v1.Group("/users")
		users.Use(middleware.AuthMiddleware(jwtManager, authSvc))
		{
			// Current user profile
			users.GET("/me", userHdlr.GetProfile)
			users.PATCH("/me", userHdlr.UpdateProfile)
			users.POST("/me/password", userHdlr.ChangePassword)
			users.POST("/me/email", userHdlr.ChangeEmail)
			users.POST("/me/email/verify", userHdlr.VerifyEmail)
			users.DELETE("/me", userHdlr.DeleteAccount)

			// Blocked users
			users.GET("/me/blocked", userHdlr.GetBlockedUsers)
			users.POST("/:id/block", userHdlr.BlockUser)
			users.DELETE("/:id/block", userHdlr.UnblockUser)

			// Friends
			users.GET("/me/friends", userHdlr.GetFriends)
			users.POST("/:id/friend", userHdlr.SendFriendRequest)
			users.POST("/me/friends/:id/accept", userHdlr.AcceptFriendRequest)
			users.DELETE("/me/friends/:id/reject", userHdlr.RejectFriendRequest)
			users.DELETE("/me/friends/:id", userHdlr.Unfriend)
		}

		// Conversation Management routes (all require authentication)
		conversations := v1.Group("/conversations")
		conversations.Use(middleware.AuthMiddleware(jwtManager, authSvc))
		{
			conversations.POST("", conversationHdlr.CreateConversation)
			conversations.GET("", conversationHdlr.GetConversations)
			conversations.GET("/:id", conversationHdlr.GetConversation)
			conversations.PATCH("/:id", conversationHdlr.UpdateConversation)
			conversations.DELETE("/:id", conversationHdlr.DeleteConversation)
			conversations.PUT("/:id/settings", conversationHdlr.UpdateSettings)
			conversations.POST("/:id/participants", conversationHdlr.AddParticipants)
			conversations.GET("/:id/participants", conversationHdlr.GetParticipants)
			conversations.DELETE("/:id/participants/:userId", conversationHdlr.RemoveParticipant)
		}
	}

	// 9. Start server in goroutine
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		logger.Info("Auth Service starting",
			zap.Int("port", cfg.Server.Port),
		)
		logger.Info("Routes configured",
			zap.String("auth", "/v1/auth/*"),
			zap.String("password_reset", "/v1/auth/password-reset/*"),
			zap.String("users", "/v1/users/*"),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server")
		}
	}()

	// 9. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), constants.GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown")
	}

	logger.Info("Server exited")
}
