package engine

import (
	"github.com/sameerkrdev/nerve/apps/candle-service/internal"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

type WorkerRouter struct {
	workers []*Worker
	count   int
}

func NewRouterWorker(numOfWorkers int) *WorkerRouter {
	var workers []*Worker

	for i := 0; i < numOfWorkers; i++ {
		worker := NewWorker(i)

		go worker.Process()

		workers = append(workers, worker)
	}

	return &WorkerRouter{
		workers: workers,
		count:   numOfWorkers,
	}
}

func (rw *WorkerRouter) Route(event *pb.EngineEvent) {
	hashNum := int(internal.Hash(event.Symbol) % uint32(rw.count))

	worker := rw.workers[hashNum]

	worker.queue <- event
}
