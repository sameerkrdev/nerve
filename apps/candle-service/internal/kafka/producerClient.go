package kafka

import (
	"fmt"
	"log"
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

		config := sarama.NewConfig()
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Retry.Max = 5
		config.Producer.Return.Successes = true
		config.Producer.Idempotent = true
		config.Net.MaxOpenRequests = 1

		config.Producer.Flush.Frequency = 10 * time.Millisecond
		config.Producer.Flush.Bytes = 1024 * 1024

		KafkaProducerClient, err = sarama.NewAsyncProducer(brokers, config)
		if err != nil {
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
		Topic: "candles",
		Key:   sarama.StringEncoder(fmt.Sprintf("%s:%s", strings.ToUpper(symbol), strings.ToLower(timeframe))),
		Value: sarama.ByteEncoder(value),
	}

	KafkaProducerClient.Input() <- msg
}
