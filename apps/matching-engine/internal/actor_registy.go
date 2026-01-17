package internal

import (
	"fmt"
	"log"
	"log/slog"

	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

type Symbol struct {
	Name            string
	StartingPrice   int64
	MaxWalFileSize  int
	WalDir          string
	WalSyncInterval int
	WalShouldFsync  bool
	KafkaBatchSize  int
	KafkaEmitMM     int
}

var actors = map[string]*SymbolActor{}

func StartActors(symbols []Symbol) {
	for _, sym := range symbols {
		actor, err := NewSymbolActor(sym, 8192)
		if err != nil {
			log.Fatalln("Failed to start actor", symbols)
		}

		// 1. Load snapshot (if exists)
		// 2. Replay WAL (blocking)
		actor.ReplyWal(0)

		// 3. Start other workers owned by actor
		go actor.wal.keepSyncing()
		go actor.kafkaEmitter.Run()
		// go actor.snapshotWorker()

		// 4. Start actor loop LAST
		go actor.Run()

		actors[sym.Name] = actor
	}
}

func PlaceOrder(order *Order) (*AddOrderInternalResponse, error) {
	actor, ok := actors[order.Symbol]
	if !ok {
		return nil, fmt.Errorf("unknown symbol %s", order.Symbol)
	}

	replyCh := make(chan *AddOrderInternalResponse, 1)
	errCh := make(chan error, 1)
	actor.inbox <- PlaceOrderMsg{
		Order: order,
		Reply: replyCh,
		Err:   errCh,
	}

	select {
	case res := <-replyCh:
		return res, nil
	case err := <-errCh:
		return nil, fmt.Errorf("failed to process the order. Error: %v", err)
	}
}

func CancelOrder(id string, userID string, symbol string) (*CancelOrderInternalResponse, error) {
	actor, ok := actors[symbol]
	if !ok {
		return nil, fmt.Errorf("unknown symbol %s", symbol)
	}

	replyCh := make(chan *CancelOrderInternalResponse, 1)
	errCh := make(chan error, 1)

	actor.inbox <- CancelOrderMsg{
		ID:     id,
		UserID: userID,
		Symbol: symbol,
		Reply:  replyCh,
		Err:    errCh,
	}

	select {
	case res := <-replyCh:
		return res, nil
	case err := <-errCh:
		return nil, err
	}
}

func ModifyOrder(
	symbol string,
	orderID string,
	userID string,
	clientModifyID string,
	newPrice *int64,
	newQuantity *int64,
) (*ModifyOrderInternalResponse, error) {
	actor, ok := actors[symbol]
	if !ok {
		return nil, fmt.Errorf("unknown symbol %s", symbol)
	}

	replyCh := make(chan *ModifyOrderInternalResponse, 1)
	errCh := make(chan error, 1)

	actor.inbox <- ModifyOrderMsg{
		Symbol:         symbol,
		OrderID:        orderID,
		UserID:         userID,
		ClientModifyID: clientModifyID,
		NewPrice:       newPrice,
		NewQuantity:    newQuantity,
		Reply:          replyCh,
		Err:            errCh,
	}

	select {
	case res := <-replyCh:
		return res, nil
	case err := <-errCh:
		return nil, err
	}
}

func SubscribeSymbol(symbol string, gatewayId string, stream pb.MatchingEngine_SubscribeSymbolServer) error {
	actor, ok := actors[symbol]
	if !ok {
		return fmt.Errorf("unknown symbol %s", symbol)
	}

	actor.mu.Lock()
	actor.grpcStreams = append(actor.grpcStreams, stream)
	actor.mu.Unlock()
	slog.Info(fmt.Sprintf("Gateway %s subscribed to %s via dedicated stream", gatewayId, symbol))

	<-stream.Context().Done()

	for i, s := range actor.grpcStreams {
		if s == stream {
			actor.mu.Lock()
			actor.grpcStreams = append(actor.grpcStreams[:i], actor.grpcStreams[i+1:]...)
			actor.mu.Unlock()
			break
		}
	}

	slog.Info(fmt.Sprintf("Gateway %s unsubscribed to %s", gatewayId, symbol))

	return nil
}
