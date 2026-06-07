package kafka

import (
	"log"

	"github.com/IBM/sarama"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/engine"
	"github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

type ConsumerHandler struct {
	router *engine.WorkerRouter
}

func NewConsumerHandler(router *engine.WorkerRouter) *ConsumerHandler {
	return &ConsumerHandler{router: router}
}

func (ch *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	log.Println("consumer group session started")

	return nil
}

func (ch *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	log.Println("consumer group session ended")

	return nil
}

func (ch *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {

		event := &pb.EngineEvent{}
		if err := proto.Unmarshal(msg.Value, event); err != nil {
			log.Println("failed to unmarshal engine event", "error", err)
			session.MarkMessage(msg, "") // TODO: maybe retry
			continue
		}

		if event.EventType == common.EventType_TRADE_EXECUTED {
			ch.router.Route(event)
		}

		// mark message as processed
		session.MarkMessage(msg, "")
	}
	return nil
}
