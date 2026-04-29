package kafka

import (
	"log/slog"

	"github.com/IBM/sarama"
)

type ConsumerHandler struct{}

func NewConsumerHandler() *ConsumerHandler {
	return &ConsumerHandler{}
}

func (h *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	slog.Info("Consumer group session started")
	return nil
}

func (h *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	slog.Info("Consumer group session ended")
	return nil
}

func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		slog.Info("message consumed", "topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset, "value", string(msg.Value))

		// mark message as processed
		session.MarkMessage(msg, "")
	}
	return nil
}
