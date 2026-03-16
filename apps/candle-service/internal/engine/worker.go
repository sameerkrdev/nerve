package engine

import (
	"github.com/sameerkrdev/nerve/apps/candle-service/internal"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

type Worker struct {
	queue       chan *pb.EngineEvent
	index       int
	candleCache *internal.CandleInMemoryCache
}

func NewWorker(index int, candleCache *internal.CandleInMemoryCache) *Worker {
	queue := make(chan *pb.EngineEvent, 100000)

	return &Worker{
		queue:       queue,
		index:       index,
		candleCache: candleCache,
	}
}

func (w *Worker) Process() {
	for event := range w.queue {
		//
		tradeEvent := &pb.TradeEvent{}

		proto.Unmarshal(event.Data, tradeEvent)

		w.candleCache.AddNewCandle(tradeEvent.Symbol, tradeEvent)
	}
}
