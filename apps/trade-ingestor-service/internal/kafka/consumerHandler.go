package kafka

import (
	"log/slog"

	"github.com/IBM/sarama"
	"github.com/sameerkrdev/nerve/apps/trade-ingestor-service/internal/clickhouse"
	"github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pbEngine "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

type ConsumerHandler struct {
	batcher *clickhouse.TradeBatcher
}

func NewConsumerHandler(batcher *clickhouse.TradeBatcher) *ConsumerHandler {
	return &ConsumerHandler{batcher: batcher}
}

func (h *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.batcher.SetSession(session)
	slog.Info("consumer group session started")
	return nil
}

func (h *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	slog.Info("consumer group session ended")
	return nil
}

func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		event := &pbEngine.EngineEvent{}
		if err := proto.Unmarshal(msg.Value, event); err != nil {
			slog.Error("failed to unmarshal engine event", "error", err)
			session.MarkMessage(msg, "")
			continue
		}

		if event.EventType != common.EventType_TRADE_EXECUTED {
			session.MarkMessage(msg, "")
			continue
		}

		trade := &pbEngine.TradeEvent{}
		if err := proto.Unmarshal(event.Data, trade); err != nil {
			slog.Error("failed to unmarshal trade event", "error", err)
			session.MarkMessage(msg, "")
			continue
		}

		h.batcher.Insert(msg, trade)
		// batcher marks msg after successful ClickHouse flush
	}
	return nil
}
