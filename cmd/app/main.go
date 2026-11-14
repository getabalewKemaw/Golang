package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"task-manager/configs"
	"task-manager/internal/handlers"
	"task-manager/internal/middleware"
	"task-manager/internal/repositories"
	"task-manager/internal/routes"
	"task-manager/internal/services"
	"task-manager/internal/workers"
	"task-manager/pkg/cache"
	"task-manager/pkg/database"
	"task-manager/pkg/telemetry"
)

func main() {
	appConfig := configs.Load()

	// Initialize structured logger (JSON in production, readable text in development)
	var logHandler slog.Handler
	if appConfig.AppEnv == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(logHandler))

	databasePool, err := database.ConnectPostgres(appConfig.DB.DSN())
	if err != nil {
		slog.Warn("database connection failed on startup", slog.Any("error", err))
	} else {
		defer databasePool.Close()
		telemetry.RegisterDatabaseMetrics(databasePool)
	}

	// Initialize Redis Cache with graceful fallback to NoOp cache if offline
	var cacheService cache.CacheService
	redisClient := redis.NewClient(&redis.Options{
		Addr:     appConfig.Redis.Address(),
		Password: appConfig.Redis.Password,
		DB:       0,
	})

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelPing()

	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		slog.Warn("redis offline; falling back to in-memory bypass cache", slog.Any("error", err))
		cacheService = cache.NewNoOpCache()
	} else {
		slog.Info("connected to Redis cache successfully", slog.String("addr", appConfig.Redis.Address()))
		cacheService = cache.NewRedisCache(redisClient)
		defer redisClient.Close()
	}

	// Explicit constructor-based dependency injection
	transactor := database.NewPostgresTransactor(databasePool)
	userRepository := repositories.NewPostgresUserRepository(databasePool)
	tokenRepository := repositories.NewPostgresTokenRepository(databasePool)
	taskRepository := repositories.NewPostgresTaskRepository(databasePool)

	authService := services.NewAuthService(
		userRepository,
		tokenRepository,
		taskRepository,
		transactor,
		appConfig.JWTAccessSecret,
		appConfig.JWTRefreshSecret,
	)
	authHandler := handlers.NewAuthHandler(authService)
	taskService := services.NewTaskService(taskRepository, cacheService)

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// Background Workers:
	// 1. Event-driven pool stays alive during HTTP drain and exits on explicit Stop()
	taskEventPool := workers.NewTaskEventWorkerPool(100, 3)
	taskEventPool.Start(context.Background())
	defer taskEventPool.Stop()

	// 2. Periodic cleanup stops immediately when OS shutdown signal arrives
	tokenCleanupWorker := workers.NewTokenCleanupWorker(tokenRepository, 1*time.Hour)
	tokenCleanupWorker.Start(signalContext)

	taskHandler := handlers.NewTaskHandler(taskService, taskEventPool)

	router := routes.SetupRouter(
		taskHandler,
		authHandler,
		appConfig.JWTAccessSecret,
		tokenRepository,
		databasePool,
	)

	// Initialize global IP rate limiter: 10 requests/second sustained, 20 burst
	ipRateLimiter := middleware.NewIPRateLimiter(10.0, 20.0)

	// Global HTTP middleware pipeline:
	// RequestID -> Recovery -> Logging -> Metrics -> RateLimiting -> Router
	var rootHandler http.Handler = router
	rootHandler = middleware.RateLimiterMiddleware(ipRateLimiter)(rootHandler)
	rootHandler = middleware.MetricsMiddleware(rootHandler)
	rootHandler = middleware.LoggingMiddleware(rootHandler)
	rootHandler = middleware.RecoveryMiddleware(rootHandler)
	rootHandler = middleware.RequestIDMiddleware(rootHandler)

	serverAddr := ":" + appConfig.Port
	server := &http.Server{
		Addr:    serverAddr,
		Handler: rootHandler,
	}

	slog.Info("server starting",
		slog.String("env", appConfig.AppEnv),
		slog.String("addr", serverAddr),
		slog.String("api_base", fmt.Sprintf("http://localhost%s/api/v1", serverAddr)),
	)

	serverErrChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrChan <- err
		}
	}()

	select {
	case err := <-serverErrChan:
		slog.Error("fatal server startup error", slog.Any("error", err))
		os.Exit(1)
	case <-signalContext.Done():
		slog.Info("shutdown signal received; commencing graceful drain")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful server shutdown failed", slog.Any("error", err))
		_ = server.Close()
	} else {
		slog.Info("HTTP listeners closed gracefully; in-flight requests drained")
	}

	taskEventPool.Stop()
	slog.Info("task event worker pool stopped and flushed")
	slog.Info("graceful shutdown completed successfully")
}
