package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sameerkrdev/nerve/apps/trade-ingestor-service/internal/clickhouse"
	"github.com/sameerkrdev/nerve/apps/trade-ingestor-service/internal/kafka"
)

func main() {
	godotenv.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chConn, err := clickhouse.NewClickhouseClient(ctx)
	if err != nil {
		slog.Error("clickhouse init failed", "error", err)
		os.Exit(1)
	}

	if err := clickhouse.EnsureSchema(ctx, chConn); err != nil {
		slog.Error("clickhouse schema setup failed", "error", err)
		os.Exit(1)
	}

	brokersEnv := os.Getenv("KAFKA_BROKERS")
	if brokersEnv == "" {
		brokersEnv = "localhost:19092,localhost:19093,localhost:19094"
	}
	brokerAddresses := strings.Split(brokersEnv, ",")

	_, err = kafka.InitKafkaConsumerClient(brokerAddresses)
	if err != nil {
		slog.Error("kafka init failed", "error", err)
		os.Exit(1)
	}

	batcher := clickhouse.NewTradeBatcher(chConn, 500, 50*time.Millisecond)
	go batcher.Start(ctx)

	consumerHandler := kafka.NewConsumerHandler(batcher)

	topics := []string{"trades"}

	go kafka.Consume(ctx, topics, consumerHandler)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	slog.Info("shutting down...")
	cancel()
}
