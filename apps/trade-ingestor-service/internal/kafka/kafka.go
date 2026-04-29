package kafka

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

var (
	KafkaConsumerClient sarama.ConsumerGroup
	initErr             error
	once                sync.Once
)

func InitKafkaConsumerClient(brokers []string) (*sarama.ConsumerGroup, error) {
	once.Do(func() {
		config := sarama.NewConfig()

		config.Consumer.Fetch.Default = 5 * 1024 * 1024
		config.Consumer.MaxProcessingTime = 3 * time.Second
		config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategySticky()
		config.Consumer.Offsets.AutoCommit.Enable = true
		config.Consumer.Offsets.AutoCommit.Interval = 2 * time.Second
		config.Consumer.MaxWaitTime = 500 * time.Millisecond

		conn, err := sarama.NewConsumerGroup(brokers, "trade-ingestor-service", config)

		if err != nil {
			initErr = err
			return
		}

		KafkaConsumerClient = conn
	})
	return &KafkaConsumerClient, initErr
}

func Close() error {
	return KafkaConsumerClient.Close()
}

func Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) {
	defer Close()

	go func() {
		for err := range KafkaConsumerClient.Errors() {
			slog.Error("kafka consume error", "error", err)
		}
	}()

	for {
		if err := KafkaConsumerClient.Consume(ctx, topics, handler); err != nil {
			slog.Error("consumer error:", "error", err)
		}

		// exit cleanly when context is cancelled
		if ctx.Err() != nil {
			slog.Info("kafka consumer stopped")
			return
		}
	}
}
