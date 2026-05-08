package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	internal "github.com/sameerkrdev/nerve/apps/websocket-server/internal"
)

func main() {
	godotenv.Load()

	engineURL := os.Getenv("MATCHING_ENGINE_GRPC_URL")
	if engineURL == "" {
		engineURL = "localhost:50052"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "50053"
	}

	wsg := internal.NewWSGateway()
	wsg.ConnectToEngine(engineURL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ws", wsg.HandelWebsocket)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Server is running on PORT: %s", port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
