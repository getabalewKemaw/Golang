package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"task-manager/configs"
	"task-manager/internal/handlers"
	"task-manager/internal/repositories"
	"task-manager/internal/routes"
	"task-manager/internal/services"
	"task-manager/internal/workers"
	"task-manager/pkg/database"
)

func main() {
	appConfig := configs.Load()
	databasePool, err := database.ConnectPostgres(appConfig.DB.DSN())
	if err != nil {
		log.Printf("Warning: Database connection failed: %v\n", err)
	} else {
		defer databasePool.Close()
	}

	// Explicit constructor-based dependency injection
	userRepository := repositories.NewPostgresUserRepository(databasePool)
	tokenRepository := repositories.NewPostgresTokenRepository(databasePool)
	authService := services.NewAuthService(
		userRepository,
		tokenRepository,
		appConfig.JWTAccessSecret,
		appConfig.JWTRefreshSecret,
	)
	authHandler := handlers.NewAuthHandler(authService)

	taskRepository := repositories.NewPostgresTaskRepository(databasePool)
	taskService := services.NewTaskService(taskRepository)

	rootContext, cancelRootContext := context.WithCancel(context.Background())
	defer cancelRootContext()

	// Background Workers:
	// 1. Event-driven worker pool for asynchronous task processing
	taskEventPool := workers.NewTaskEventWorkerPool(100, 3)
	taskEventPool.Start(rootContext)
	defer taskEventPool.Stop()

	// 2. Periodic worker for expired revoked token cleanup (Ticker + Select pattern)
	tokenCleanupWorker := workers.NewTokenCleanupWorker(tokenRepository, 1*time.Hour)
	tokenCleanupWorker.Start(rootContext)

	taskHandler := handlers.NewTaskHandler(taskService, taskEventPool)

	router := routes.SetupRouter(
		taskHandler,
		authHandler,
		appConfig.JWTAccessSecret,
		tokenRepository,
		databasePool,
	)

	serverAddr := ":" + appConfig.Port
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	fmt.Printf("Server starting in [%s] mode on http://localhost%s\n", appConfig.AppEnv, serverAddr)
	fmt.Printf("API Base URL: http://localhost%s/api/v1\n", serverAddr)
	fmt.Printf("Health check: http://localhost%s/api/v1/health\n", serverAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v\n", err)
	}
}
