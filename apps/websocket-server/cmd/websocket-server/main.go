package main

import (
	"context"
	"log"
	"net/http"
	"os"

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
	wsg.ConnectToEngine(engineURL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ws", wsg.HandelWebsocket)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("websocket server running on port %s", port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
