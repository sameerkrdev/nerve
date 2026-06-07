package engine

import (
	memorystore "github.com/sameerkrdev/nerve/apps/candle-service/internal/memoryStore"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/utils"
	pbAggeration "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

type WorkerRouter struct {
	workers []*Worker
	count   int
}

func NewWorkerRouter(workerCount int, onCandleClosed memorystore.OnCandleClosedFn) *WorkerRouter {
	if workerCount <= 0 {
		panic("workerCount must be > 0")
	}

	var workers []*Worker

	for i := range workerCount {
		candleCache := memorystore.NewCandleStore(onCandleClosed)
		worker := NewWorker(i, candleCache)

		go worker.Process()

		workers = append(workers, worker)
	}

	return &WorkerRouter{
		workers: workers,
		count:   workerCount,
	}
}

func (wr *WorkerRouter) Route(event *pb.EngineEvent) {
	workerIndex := int(utils.Hash(event.Symbol) % uint32(wr.count))

	worker := wr.workers[workerIndex]

	worker.eventQueue <- event
}

func (wr *WorkerRouter) GetCandles(symbol string, timeframe pbAggeration.Timeframe) ([]*pbAggeration.Candle, error) {
	workerIndex := int(utils.Hash(symbol) % uint32(wr.count))
	return wr.workers[workerIndex].candleCache.GetCandles(symbol, timeframe)
}
