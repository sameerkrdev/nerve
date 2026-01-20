package main

import (
	"fmt"
	"log"
	"net/http"

	internal "github.com/sameerkrdev/nerve/apps/websocket-server/internal"
)

func main() {
	wsg := internal.NewWSGateway()
	wsg.ConnectToEngine("localhost:50052")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ws", wsg.HandelWebsocket)

	server := &http.Server{
		Addr:    ":50053",
		Handler: mux,
	}

	fmt.Println("Server is running on PORT: 50053")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
