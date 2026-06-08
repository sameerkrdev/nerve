package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
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

func parseRSAPublicKey(b64pem string) (*rsa.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64pem)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	block, _ := pem.Decode(decoded)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaKey, nil
}


func main() {
	godotenv.Load()

	redisClient, err := internal.InitRedis()
	if err != nil {
		log.Fatalf("redis init failed: %v", err)
	}

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	jwtPublicKeyB64 := os.Getenv("JWT_PUBLIC_KEY")
	if jwtPublicKeyB64 == "" {
		log.Fatal("JWT_PUBLIC_KEY env var required")
	}
	jwtPublicKey, err := parseRSAPublicKey(jwtPublicKeyB64)
	if err != nil {
		log.Fatalf("parse JWT_PUBLIC_KEY: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "50053"
	}

	wsg := internal.NewWSGateway(redisClient, jwtPublicKey)

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
