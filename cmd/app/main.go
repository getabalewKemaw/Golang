package main

import (
	"fmt"
	"log"
	"net/http"

	"task-manager/configs"
	"task-manager/internal/handlers"
	"task-manager/internal/repositories"
	"task-manager/internal/routes"
	"task-manager/internal/services"
	"task-manager/pkg/database"
)

func main() {
	cfg := configs.Load()

	databasePool, err := database.ConnectPostgres(cfg.DB.DSN())
	if err != nil {
		log.Printf("Warning: Database connection failed: %v\n", err)
	} else {
		defer databasePool.Close()
	}

	//  dependency Injection: Database -> Repository -> Service -> Handler
	taskRepository := repositories.NewPostgresTaskRepository(databasePool)
	taskService := services.NewTaskService(taskRepository)
	taskHandler := handlers.NewTaskHandler(taskService)

	router := routes.SetupRouter(taskHandler, databasePool)

	serverAddr := ":" + cfg.Port
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	fmt.Printf("Server starting in [%s] mode on http://localhost%s\n", cfg.AppEnv, serverAddr)
	fmt.Printf("API Base URL: http://localhost%s/api/v1\n", serverAddr)
	fmt.Printf("Health check: http://localhost%s/api/v1/health\n", serverAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v\n", err)
	}
}
