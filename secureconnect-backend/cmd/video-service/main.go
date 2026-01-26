package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	intDatabase "secureconnect-backend/internal/database"
	videoHandler "secureconnect-backend/internal/handler/http/video"
	wsHandler "secureconnect-backend/internal/handler/ws"
	"secureconnect-backend/internal/middleware"
	"secureconnect-backend/internal/repository/cockroach"
	redisRepo "secureconnect-backend/internal/repository/redis"
	videoService "secureconnect-backend/internal/service/video"
	pkgDatabase "secureconnect-backend/pkg/database"
	"secureconnect-backend/pkg/env"
	"secureconnect-backend/pkg/jwt"
	"secureconnect-backend/pkg/metrics"
	"secureconnect-backend/pkg/push"
)

func main() {
	// Create context for database operations
	ctx := context.Background()

	// 1. Setup JWT Manager
	jwtSecret := env.GetStringFromFile("JWT_SECRET", "")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}

	jwtManager := jwt.NewJWTManager(jwtSecret, 15*time.Minute, 30*24*time.Hour)

	// Validate production mode
	productionMode := os.Getenv("ENV") == "production"

	// 2. Connect to CockroachDB for call logs with retry logic
	dbConfig := &pkgDatabase.CockroachConfig{
		Host:     env.GetString("DB_HOST", "localhost"),
		Port:     26257,
		User:     env.GetString("DB_USER", "root"),
		Password: env.GetStringFromFile("DB_PASSWORD", ""),
		Database: env.GetString("DB_NAME", "secureconnect"),
		SSLMode:  "disable",
	}

	// Connect to CockroachDB with exponential backoff retry
	var db *pkgDatabase.CockroachDB
	var err error

	maxRetries := 5
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second

	// Execute first connection attempt
	db, err = pkgDatabase.NewCockroachDB(ctx, dbConfig)
	if err == nil {
		log.Println("✅ Connected to CockroachDB")
	} else {
		// Retry with exponential backoff
		for attempt := 2; attempt <= maxRetries; attempt++ {
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > maxDelay {
				delay = maxDelay
			}
			log.Printf("⚠️  CockroachDB connection attempt %d failed: %v. Retrying in %v...", attempt, err, delay)
			time.Sleep(delay)

			// Retry connection
			db, err = pkgDatabase.NewCockroachDB(ctx, dbConfig)
			if err == nil {
				log.Printf("✅ Connected to CockroachDB (attempt %d/%d)", attempt, maxRetries)
				break
			}
		}
	}

	if err != nil {
		log.Printf("Warning: Failed to connect to CockroachDB after %d attempts: %v", maxRetries, err)
		log.Println("Running in limited mode without call logs persistence")
	}

	var callRepo *cockroach.CallRepository
	var conversationRepo *cockroach.ConversationRepository
	var userRepo *cockroach.UserRepository
	if db != nil {
		defer db.Close()
		callRepo = cockroach.NewCallRepository(db.Pool)
		conversationRepo = cockroach.NewConversationRepository(db.Pool)
		userRepo = cockroach.NewUserRepository(db.Pool)
		log.Println("✅ Connected to CockroachDB")
	}

	// 3. Initialize Redis with degraded mode support
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
		log.Printf("Warning: Failed to connect to Redis: %v", err)
	} else {
		log.Println("✅ Connected to Redis")
	}
	defer redisDB.Close()

	// Start background Redis health check
	go redisDB.StartHealthCheck(ctx, 10*time.Second)
	log.Println("✅ Redis health check started (10s interval)")

	// 4. Initialize Push Service
	pushTokenRepo := redisRepo.NewPushTokenRepository(redisDB.Client)

	// Select push provider based on environment
	var pushProvider push.Provider
	pushProviderType := env.GetString("PUSH_PROVIDER", "mock")

	switch pushProviderType {
	case "firebase":
		// Firebase Cloud Messaging (supports Android, iOS via APNs bridge, Web)
		// Firebase provider handles credential loading from Docker secrets internally
		firebaseProjectID := env.GetStringFromFile("FIREBASE_PROJECT_ID", "")
		if firebaseProjectID == "" {
			if productionMode {
				log.Println("❌ FIREBASE_PROJECT_ID not set. Required in production mode.")
				log.Println("❌ Please create Docker secret: echo 'your-project-id' | docker secret create firebase_project_id -")
				log.Fatal("❌ Fatal: Firebase project ID required in production mode")
			}
			log.Println("Warning: FIREBASE_PROJECT_ID not set, falling back to mock provider")
			pushProvider = &push.MockProvider{}
		} else {
			pushProvider = push.NewFirebaseProvider(firebaseProjectID)
			log.Printf("✅ Using Firebase Provider for project: %s", firebaseProjectID)

			// Perform startup validation check
			if fbProvider, ok := pushProvider.(*push.FirebaseProvider); ok {
				if err := push.StartupCheck(fbProvider); err != nil {
					if productionMode {
						log.Fatal("❌ Fatal: Firebase startup check failed")
					}
				}
			}
		}
	case "mock", "":
		// Mock provider for development/testing
		if productionMode {
			log.Println("❌ ERROR: PUSH_PROVIDER=mock is not allowed in production mode!")
			log.Println("❌ Please set PUSH_PROVIDER=firebase and configure Firebase credentials")
			log.Fatal("❌ Fatal: Mock push provider not allowed in production")
		}
		pushProvider = &push.MockProvider{}
		log.Println("ℹ️  Using MockProvider for push notifications (development mode)")
	default:
		log.Printf("Warning: Unknown PUSH_PROVIDER '%s', falling back to mock", pushProviderType)
		pushProvider = &push.MockProvider{}
	}

	pushSvc := push.NewService(pushProvider, pushTokenRepo)

	// 5. Initialize Video Service
	videoSvc := videoService.NewService(callRepo, conversationRepo, userRepo, pushSvc)

	// 6. Initialize Metrics
	appMetrics := metrics.NewMetrics("video-service")
	prometheusMiddleware := middleware.NewPrometheusMiddleware(appMetrics)

	// 7. Initialize Handlers
	videoHdlr := videoHandler.NewHandler(videoSvc)

	// 8. Initialize WebRTC Signaling Hub
	signalingHub := wsHandler.NewSignalingHub(redisDB)

	// 9. Setup Gin Router
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

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "video-service",
			"time":    time.Now().UTC(),
		})
	})

	// Metrics endpoint (for Prometheus scraping)
	router.GET("/metrics", middleware.MetricsHandler(appMetrics))

	// Revocation checker
	revocationChecker := middleware.NewRedisRevocationChecker(redisDB.Client)

	// Video routes (all require authentication)
	v1 := router.Group("/v1/calls")
	v1.Use(middleware.AuthMiddleware(jwtManager, revocationChecker))
	{
		// Call management endpoints
		v1.POST("/initiate", videoHdlr.InitiateCall)
		v1.POST("/:id/end", videoHdlr.EndCall)
		v1.POST("/:id/join", videoHdlr.JoinCall)
		v1.GET("/:id", videoHdlr.GetCallStatus)

		// WebSocket endpoint for WebRTC signaling
		v1.GET("/ws/signaling", signalingHub.ServeWS)
	}

	// 10. Start server
	port := env.GetString("PORT", "8083")
	addr := fmt.Sprintf(":%s", port)

	log.Printf("🚀 Video Service starting on port %s\n", port)
	log.Println("📡 WebRTC Signaling: /v1/calls/ws/signaling")
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
