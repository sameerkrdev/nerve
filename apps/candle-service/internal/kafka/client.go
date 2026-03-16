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

type KafkaClient struct {
	group   sarama.ConsumerGroup
	brokers []string
}

func NewKafkaClient(brokers []string) *KafkaClient {
	config := sarama.NewConfig()

	config.Consumer.Fetch.Default = 5 * 1024 * 1024
	config.Consumer.MaxProcessingTime = 3 * time.Second
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategySticky()
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 2 * time.Second
	config.Consumer.MaxWaitTime = 500 * time.Millisecond

	client, err := sarama.NewConsumerGroup(brokers, "candle_service_group", config)

	if err != nil {
		panic(err)
	}

	return &KafkaClient{
		group:   client,
		brokers: brokers,
	}
}

func (k *KafkaClient) Close() error {
	return k.group.Close()
}

func (k *KafkaClient) Consume(
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

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigchan
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
