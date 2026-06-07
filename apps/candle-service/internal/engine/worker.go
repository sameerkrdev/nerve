package engine

import (
	memorystore "github.com/sameerkrdev/nerve/apps/candle-service/internal/memoryStore"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

type Worker struct {
	eventQueue  chan *pb.EngineEvent
	id          int
	candleCache *memorystore.CandleStore
}

func NewWorker(id int, candleCache *memorystore.CandleStore) *Worker {
	eventQueue := make(chan *pb.EngineEvent, 100000)

	return &Worker{
		eventQueue:  eventQueue,
		id:          id,
		candleCache: candleCache,
	}
}

func (w *Worker) Process() {
	for event := range w.eventQueue {
		tradeEvent := &pb.TradeEvent{}

		proto.Unmarshal(event.Data, tradeEvent)

		w.candleCache.AddNewCandle(tradeEvent.Symbol, tradeEvent)
	}
}
