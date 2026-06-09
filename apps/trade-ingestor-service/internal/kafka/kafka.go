package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
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
		caCert := os.Getenv("KAFKA_CA")
		if caCert == "" {
			initErr = fmt.Errorf("KAFKA_CA is required in environment variables")
			return
		}

		if os.Getenv("KAFKA_USERNAME") == "" || os.Getenv("KAFKA_PASSWORD") == "" {
			initErr = fmt.Errorf("KAFKA_USERNAME and KAFKA_PASSWORD are required in environment variables")
			return
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
		config.ClientID = "trade-ingestor-service"
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
		config.Consumer.Return.Errors = true

		config.Net.TLS.Config = tlsConfig

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
