package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/sameerkrdev/nerve/apps/candle-service/internal"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/engine"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/kafka"
)

//* func: define mux server and start consumer and workers
//* func: start the kafka consumer
//* func: start the workers
//*	 - each worker recieve gets single symbol trade data via channel
//*	 - calculate the candlestick data for multiple timeframe
//*	 - L1: In-memory (last 1000 candles)
//	 - L2: Redis Memory (last 5000 candles)
//	 - L3: store the trades into clickhouse which will eventually generate the candles data
//	 - publish to kafka or redis pub/sub for indicator service
// func: to get the historical data of candles
// func: graceful shutdown

// in main or in server, initialize router workers, then initialize kafka consumer handler then initialize kafka client with consume func call

func main() {
	brokerAddresses := []string{
		"localhost:536",
		"",
	}

	workerRouter := engine.NewWorkerRouter(10)
	mux := internal.NewServer(workerRouter)

	kafkaConsumerClient, err := kafka.NewKafkaConsumerClient(brokerAddresses)
	if err != nil {
		slog.Error("kafka consumer connection failed", "error", err)
		os.Exit(1)
	}

	kafkaConsumerHandler := kafka.NewConsumerHandler(workerRouter)

	topics := []string{
		"trades",
	}

	go kafkaConsumerClient.Consume(topics, kafkaConsumerHandler)

	server := &http.Server{
		Addr:    ":50054",
		Handler: mux,
	}

	fmt.Println("Server is running on PORT: 50054")

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
