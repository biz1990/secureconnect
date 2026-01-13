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

	authHandler "secureconnect-backend/internal/handler/http/auth"
	userHandler "secureconnect-backend/internal/handler/http/user"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	"secureconnect-backend/internal/repository/redis"
	authService "secureconnect-backend/internal/service/auth"
	userService "secureconnect-backend/internal/service/user"
	"secureconnect-backend/pkg/config"
	"secureconnect-backend/pkg/constants"
	"secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/email"
	"secureconnect-backend/pkg/jwt"
)

func main() {
	// Initialize context
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate JWT secret in production
	if cfg.Server.Environment == "production" {
		if cfg.JWT.Secret == "" {
			log.Fatal("JWT_SECRET environment variable is required in production")
		}
		if len(cfg.JWT.Secret) < 32 {
			log.Fatal("JWT_SECRET must be at least 32 characters")
		}
	}

	// 1. Setup JWT Manager
	jwtManager := jwt.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenExpiry,
		cfg.JWT.RefreshTokenExpiry,
	)

	// 2. Connect to CockroachDB
	cockroachDB, err := database.NewCockroachDB(ctx, &database.CockroachConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		log.Fatalf("Failed to connect to CockroachDB: %v", err)
	}
	defer cockroachDB.Close()

	log.Println("✅ Connected to CockroachDB")

	// 3. Connect to Redis
	redisDB, err := database.NewRedisDB(&database.RedisConfig{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
		Timeout:  cfg.Redis.Timeout,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisDB.Close()

	log.Println("✅ Connected to Redis")

	// 4. Initialize Repositories
	userRepo := cockroach.NewUserRepository(cockroachDB.Pool)
	blockedUserRepo := cockroach.NewBlockedUserRepository(cockroachDB.Pool)
	emailVerificationRepo := cockroach.NewEmailVerificationRepository(cockroachDB.Pool)
	directoryRepo := redis.NewDirectoryRepository(redisDB.Client)
	sessionRepo := redis.NewSessionRepository(redisDB.Client)
	presenceRepo := redis.NewPresenceRepository(redisDB.Client)

	// 5. Initialize Services
	authSvc := authService.NewService(userRepo, directoryRepo, sessionRepo, presenceRepo, jwtManager)

	// Create email service (using mock sender for development)
	// In production, replace with real email provider (SendGrid, AWS SES, etc.)
	emailSvc := email.NewService(&email.MockSender{})

	userSvc := userService.NewService(userRepo, blockedUserRepo, emailVerificationRepo, emailSvc)

	// 6. Initialize Handlers
	authHdlr := authHandler.NewHandler(authSvc)
	userHdlr := userHandler.NewHandler(userSvc)

	// 7. Setup Gin Router
	router := gin.Default()

	// Apply middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORSMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "auth-service",
			"time":    time.Now().UTC(),
		})
	})

	// API version 1 routes
	v1 := router.Group("/v1")
	{
		// Auth routes (public, no authentication required)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHdlr.Register)
			auth.POST("/login", authHdlr.Login)
			auth.POST("/refresh", authHdlr.RefreshToken)

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
	}

	// 8. Start server in goroutine
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Printf("🚀 Auth Service starting on port %d\n", cfg.Server.Port)
		log.Println("📍 Routes:")
		log.Println("   - Auth: /v1/auth/*")
		log.Println("   - Users: /v1/users/*")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 9. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), constants.GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
