package kafka

import (
	"context"
	"log"
	"time"

	"github.com/IBM/sarama"
)

type KafkaConsumerClient struct {
	group   sarama.ConsumerGroup
	brokers []string
}

func NewKafkaConsumerClient(brokers []string) (*KafkaConsumerClient, error) {
	config := sarama.NewConfig()

	config.Consumer.Fetch.Default = 5 * 1024 * 1024
	config.Consumer.MaxProcessingTime = 3 * time.Second
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategySticky()
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 2 * time.Second
	config.Consumer.MaxWaitTime = 500 * time.Millisecond

	consumerGroup, err := sarama.NewConsumerGroup(brokers, "candle_service_group", config)

	if err != nil {
		return nil, err
	}

	return &KafkaConsumerClient{
		group:   consumerGroup,
		brokers: brokers,
	}, nil
}

func (k *KafkaConsumerClient) Close() error {
	return k.group.Close()
}

func (k *KafkaConsumerClient) Consume(
	ctx context.Context,
	topics []string,
	handler sarama.ConsumerGroupHandler,
) {
	defer k.Close()

	// log errors
	go func() {
		for err := range k.group.Errors() {
			log.Println("kafka error:", err)
		}
	}()

	for {
		if err := k.group.Consume(ctx, topics, handler); err != nil {
			log.Println("consumer error:", err)
		}

		// exit cleanly when context is cancelled
		if ctx.Err() != nil {
			log.Println("kafka consumer stopped")
			return
		}
	}
}
