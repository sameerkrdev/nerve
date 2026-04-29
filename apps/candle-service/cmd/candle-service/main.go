package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/sameerkrdev/nerve/apps/candle-service/internal"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/engine"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/kafka"
	memorystore "github.com/sameerkrdev/nerve/apps/candle-service/internal/memoryStore"
)

//* func: define grpc server and start consumer and workers
//* func: start the kafka consumer
//* func: start the workers
//*	 - each worker recieve gets single symbol trade data via channel
//*	 - calculate the candlestick data for multiple timeframe
//*	 - L1: In-memory (last 1000 candles)
//*	 - L2: Redis Memory (last 5000 candles)
//	 - L3: store the trades into clickhouse which will eventually generate the candles data
//*	 - Fanout:
//* 		- publish to kafka for other services
//* 		- redis pub/sub for websockets servers
// func: to get the historical data of candles
// func: graceful shutdown

// in main or in server, initialize router workers, then initialize kafka consumer handler then initialize kafka client with consume func call

func main() {
	if err := memorystore.InitRedis(); err != nil {
		slog.Error("redis init failed", "error", err)
		os.Exit(1)
	}

	brokerAddresses := []string{"localhost:19092", "localhost:19093", "localhost:19094"}

	if err := kafka.InitKafkaProducer(brokerAddresses); err != nil {
		slog.Error("kafka producer init failed", "error", err)
		os.Exit(1)
	}

	workerRouter := engine.NewWorkerRouter(10, kafka.PublishCandleEventToKafka)
	// mux := internal.NewServer(workerRouter)

	kafkaConsumerClient, err := kafka.NewKafkaConsumerClient(brokerAddresses)
	if err != nil {
		slog.Error("kafka consumer connection failed", "error", err)
		os.Exit(1)
	}

	kafkaConsumerHandler := kafka.NewConsumerHandler(workerRouter)

	topics := []string{
		"trades",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go kafkaConsumerClient.Consume(ctx, topics, kafkaConsumerHandler)

	PORT := "50054"

	listener, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		slog.Error("net server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Net server listening", "port", PORT)

	grpcServer := internal.NewGrpcServer(workerRouter, listener)

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	slog.Info("shutting down...")

	cancel()
	grpcServer.GracefulStop()
}
