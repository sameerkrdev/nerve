package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal"
	clickhousepkg "github.com/sameerkrdev/nerve/apps/candle-service/internal/clickhouse"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/engine"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/kafka"
	memorystore "github.com/sameerkrdev/nerve/apps/candle-service/internal/memoryStore"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
)

func main() {
	godotenv.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := memorystore.InitRedis(); err != nil {
		slog.Error("redis init failed", "error", err)
		os.Exit(1)
	}

	chConn, err := clickhousepkg.NewClickhouseClient(ctx)
	if err != nil {
		slog.Warn("clickhouse init failed — L3 disabled", "error", err)
		chConn = nil
	}

	brokerAddresses := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")

	if err := kafka.InitKafkaProducer(brokerAddresses); err != nil {
		slog.Error("kafka producer init failed", "error", err)
		os.Exit(1)
	}

	onCandleClosed := func(symbol, timeframe string, candle *pb.Candle) {
		if err := memorystore.PushCandle(symbol, timeframe, candle); err != nil {
			slog.Error("redis L2 push failed", "symbol", symbol, "timeframe", timeframe, "error", err)
		}
		kafka.PublishCandleEventToKafka(symbol, timeframe, candle)
	}

	workerRouter := engine.NewWorkerRouter(10, onCandleClosed)

	kafkaConsumerClient, err := kafka.NewKafkaConsumerClient(brokerAddresses)
	if err != nil {
		slog.Error("kafka consumer connection failed", "error", err)
		os.Exit(1)
	}

	kafkaConsumerHandler := kafka.NewConsumerHandler(workerRouter)

	topics := []string{"trades"}

	go kafkaConsumerClient.Consume(ctx, topics, kafkaConsumerHandler)

	PORT := os.Getenv("PORT")

	listener, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		slog.Error("net server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Net server listening", "port", PORT)

	grpcServer := internal.NewGrpcServer(workerRouter, chConn, listener)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	slog.Info("shutting down...")

	cancel()
	grpcServer.GracefulStop()
}
