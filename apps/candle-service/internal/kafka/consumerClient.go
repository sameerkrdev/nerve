package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
)

type KafkaConsumerClient struct {
	group   sarama.ConsumerGroup
	brokers []string
}

func NewKafkaConsumerClient(brokers []string) (*KafkaConsumerClient, error) {
	caCert := os.Getenv("KAFKA_CA")
	if caCert == "" {
		return nil, fmt.Errorf("KAFKA_CA is required in environment variables")
	}

	if os.Getenv("KAFKA_USERNAME") == "" || os.Getenv("KAFKA_PASSWORD") == "" {
		return nil, fmt.Errorf("KAFKA_USERNAME and KAFKA_PASSWORD are required in environment variables")
	}

	caCertBytes := []byte(caCert)

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertBytes)
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	config := sarama.NewConfig()

	// init config, enable errors and notifications
	config.Metadata.Full = true
	config.ClientID = "candle-service"
	config.Producer.Return.Successes = true

	// Kafka SASL configuration
	config.Net.SASL.Enable = true
	config.Net.SASL.User = os.Getenv("KAFKA_USERNAME")
	config.Net.SASL.Password = os.Getenv("KAFKA_PASSWORD")
	config.Net.SASL.Handshake = true
	config.Net.SASL.Mechanism = sarama.SASLTypePlaintext

	// TLS configuration
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig

	config.Consumer.Fetch.Default = 5 * 1024 * 1024
	config.Consumer.MaxProcessingTime = 3 * time.Second
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategySticky()
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 2 * time.Second
	config.Consumer.MaxWaitTime = 500 * time.Millisecond

	consumerGroup, err := sarama.NewConsumerGroup(brokers, "candle-service", config)

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
