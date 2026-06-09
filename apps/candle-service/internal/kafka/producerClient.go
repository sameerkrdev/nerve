package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	pbAggegration "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	"google.golang.org/protobuf/proto"
)

var (
	KafkaProducerClient sarama.AsyncProducer
	once                sync.Once
)

func InitKafkaProducer(brokers []string) error {
	var err error
	once.Do(func() {
		caCert := os.Getenv("KAFKA_CA")
		if caCert == "" {
			err = fmt.Errorf("KAFKA_CA is required in environment variables")
			return
		}

		if os.Getenv("KAFKA_USERNAME") == "" || os.Getenv("KAFKA_PASSWORD") == "" {
			err = fmt.Errorf("KAFKA_USERNAME and KAFKA_PASSWORD are required in environment variables")
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

		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Retry.Max = 5
		config.Producer.Return.Successes = true
		config.Producer.Idempotent = true
		config.Net.MaxOpenRequests = 1
		config.Net.TLS.Config = tlsConfig

		config.Producer.Flush.Frequency = 10 * time.Millisecond
		config.Producer.Flush.Bytes = 1024 * 1024

		KafkaProducerClient, err = sarama.NewAsyncProducer(brokers, config)
		if err != nil {
			err = fmt.Errorf("failed to create Kafka producer: %w", err)
			return
		}

		go func() {
			for msg := range KafkaProducerClient.Successes() {
				log.Printf("sent: topic=%s partition=%d offset=%d\n",
					msg.Topic, msg.Partition, msg.Offset)
			}
		}()

		go func() {
			for err := range KafkaProducerClient.Errors() {
				log.Println("kafka error:", err.Err)
			}
		}()
	})

	return err
}

func PublishCandleEventToKafka(symbol string, timeframe string, candle *pbAggegration.Candle) {

	value, err := proto.Marshal(candle)
	if err != nil {
		log.Println("marshal error:", err)
		return
	}

	msg := &sarama.ProducerMessage{
		Topic: "candle-service.candles",
		Key:   sarama.StringEncoder(fmt.Sprintf("%s:%s", strings.ToUpper(symbol), strings.ToLower(timeframe))),
		Value: sarama.ByteEncoder(value),
	}

	KafkaProducerClient.Input() <- msg
}
