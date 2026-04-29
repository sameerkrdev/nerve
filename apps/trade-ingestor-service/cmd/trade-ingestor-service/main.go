package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sameerkrdev/nerve/apps/trade-ingestor-service/internal/clickhouse"
	"github.com/sameerkrdev/nerve/apps/trade-ingestor-service/internal/kafka"
)

// Kafka and clickhouse connection is now completed
// TODO: make a fucntion to insert the trade batch to clichouse --> InsertTrades([]*Trade)
// TODO: make a trade batching system which flush the buffer data to clichouse after some interval 50ms and mark the kafka mark msg
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := clickhouse.NewClickhouseClient(ctx)
	if err != nil {
		slog.Error("Clickhouse init failed", "error", err)
		os.Exit(1)
	}

	brokerAddresses := []string{"localhost:19092", "localhost:19093", "localhost:19094"}
	_, err = kafka.InitKafkaConsumerClient(brokerAddresses)
	if err != nil {
		slog.Error("Kafka init failed", "error", err)
		os.Exit(1)
	}

	consumerHandler := kafka.NewConsumerHandler()

	topics := []string{
		"trades",
	}

	go kafka.Consume(ctx, topics, consumerHandler)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	slog.Info("shutting down...")
}
