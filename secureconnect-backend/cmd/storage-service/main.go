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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"secureconnect-backend/internal/database"
	storageHandler "secureconnect-backend/internal/handler/http/storage"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	storageService "secureconnect-backend/internal/service/storage"
	"secureconnect-backend/pkg/config"
	pkgDatabase "secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/jwt"
	"secureconnect-backend/pkg/logger"
	"secureconnect-backend/pkg/metrics"
)

func main() {
	// Initialize logger first to ensure safe logging throughout initialization
	logger.InitDefault("storage-service")

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
	crdb, err := pkgDatabase.NewCockroachDB(ctx, &pkgDatabase.CockroachConfig{
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
	minioClient, err := minio.New(cfg.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
		Secure: cfg.MinIO.UseSSL,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	minioAdapter := &storageService.MinioAdapter{Client: minioClient}

	storageSvc, err := storageService.NewService(minioAdapter, cfg.MinIO.Bucket, fileRepo)
	if err != nil {
		log.Fatalf("Failed to initialize storage service: %v", err)
	}

	log.Println("✅ Connected to MinIO")

	// Initialize Redis metrics before connecting to Redis
	database.InitRedisMetrics()

	// 4. Initialize Metrics
	appMetrics := metrics.NewMetrics("storage-service")
	prometheusMiddleware := middleware.NewPrometheusMiddleware(appMetrics)

	// 5. Initialize Handlers
	storageHdlr := storageHandler.NewHandler(storageSvc)

	// 6. Connect to Redis
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

	// 7. Setup Gin Router
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
	router.Use(prometheusMiddleware.Handler())
	router.Use(middleware.NewTimeoutMiddleware(nil).Middleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "storage-service",
			"time":    time.Now().UTC(),
		})
	})

	// Metrics endpoint (for Prometheus scraping)
	router.GET("/metrics", middleware.MetricsHandler(appMetrics))

	// Storage routes (all require authentication)
	v1 := router.Group("/v1/storage")
	v1.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
	{
		v1.POST("/upload-url", storageHdlr.GenerateUploadURL)
		v1.GET("/download-url/:file_id", storageHdlr.GenerateDownloadURL)
		v1.DELETE("/files/:file_id", storageHdlr.DeleteFile)

		// Upload complete and quota endpoints
		v1.POST("/upload-complete", storageHdlr.CompleteUpload)
		v1.GET("/quota", storageHdlr.GetQuota)
	}

	// 8. Start server in goroutine
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
