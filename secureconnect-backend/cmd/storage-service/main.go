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

	storageHandler "secureconnect-backend/internal/handler/http/storage"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	storageService "secureconnect-backend/internal/service/storage"
	"secureconnect-backend/pkg/config"
	"secureconnect-backend/pkg/database"
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
	crdb, err := database.NewCockroachDB(ctx, &database.CockroachConfig{
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
	defer crdb.Close()
	log.Println("✅ Connected to CockroachDB")

	// Initialize Repository
	fileRepo := cockroach.NewFileRepository(crdb.Pool)

	// 3. Setup MinIO Storage Service
	minioClient, err := storageService.NewMinioClient(
		cfg.MinIO.Endpoint,
		cfg.MinIO.AccessKey,
		cfg.MinIO.SecretKey,
	)
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	minioAdapter := &storageService.MinioAdapter{Client: minioClient}

	storageSvc, err := storageService.NewService(minioAdapter, cfg.MinIO.Bucket, fileRepo)
	if err != nil {
		log.Fatalf("Failed to initialize storage service: %v", err)
	}

	log.Println("✅ Connected to MinIO")

	// 4. Initialize Handlers
	storageHdlr := storageHandler.NewHandler(storageSvc)

	// 5. Connect to Redis
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

	// Revocation checker
	revocationChecker := middleware.NewRedisRevocationChecker(redisDB.Client)

	// 4. Setup Gin Router
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

	// Apply global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.CORSMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "storage-service",
			"time":    time.Now().UTC(),
		})
	})

	// Storage routes (all require authentication)
	v1 := router.Group("/v1/storage")
	v1.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
	{
		v1.POST("/upload-url", storageHdlr.GenerateUploadURL)
		v1.GET("/download-url/:file_id", storageHdlr.GenerateDownloadURL)
		v1.DELETE("/files/:file_id", storageHdlr.DeleteFile)

		// Placeholder for upload complete and quota
		v1.POST("/upload-complete", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Not implemented yet"})
		})
		v1.GET("/quota", func(c *gin.Context) {
			c.JSON(501, gin.H{"message": "Not implemented yet"})
		})
	}

	// 6. Start server in goroutine
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Printf("🚀 Storage Service starting on port %d\n", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 7. Graceful shutdown
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
