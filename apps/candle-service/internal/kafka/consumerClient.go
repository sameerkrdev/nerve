package kafka

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	topics []string,
	handler sarama.ConsumerGroupHandler,
) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer k.Close()

	go func() {
		for err := range k.group.Errors() {
			log.Println("kafka error:", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down kafka consumer...")
		cancel()
	}()

	for {
		if err := k.group.Consume(ctx, topics, handler); err != nil {
			log.Println("consumer error:", err)
		}

		if ctx.Err() != nil {
			return
		}
	}
}
