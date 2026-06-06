package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	internal "github.com/sameerkrdev/nerve/apps/websocket-server/internal"
)

func main() {
	godotenv.Load()

	redisClient, err := internal.InitRedis()
	if err != nil {
		log.Fatalf("redis init failed: %v", err)
	}

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	engineURL := os.Getenv("MATCHING_ENGINE_GRPC_URL")
	if engineURL == "" {
		engineURL = "localhost:50052"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "50053"
	}

	wsg := internal.NewWSGateway(redisClient)

	if err := wsg.ConnectToEngine(engineURL); err != nil {
		log.Fatalf("engine connect failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ws", wsg.HandleWebSocket)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		slog.Info("websocket server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")

	wsg.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}

	slog.Info("shutdown complete")
}
