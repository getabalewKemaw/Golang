package main

import (
	"fmt"
	"log"
	"net/http"
	"task-manager/configs"
	"task-manager/pkg/database"
)

func main() {
	cfg := configs.Load()

	db, err := database.ConnectPostgres(cfg.DB.DSN())
	if err != nil {
		log.Printf("Warning: Database connection failed: %v\n", err)
	} else {
		defer db.Close()
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbStatus := "connected"
		if db == nil || db.Ping() != nil {
			dbStatus = "disconnected"
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","database":"%s"}`, dbStatus)
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world this the main route "))
	})

	serverAddr := ":" + cfg.Port
	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	fmt.Printf("Server starting in [%s] mode on http://localhost%s\n", cfg.AppEnv, serverAddr)
	fmt.Printf("Test health check at: http://localhost%s/health\n", serverAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v\n", err)
	}
}
