package internal

import (
	"fmt"
	"log"
	"log/slog"
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
			log.Fatalln("Failed to start actor", symbols, err)
		}

		// 1. Load snapshot (if exists) --> TODO
		// 2. Replay WAL (blocking)
		slog.Info(fmt.Sprintf("replaying the %s orderbook Starting...", sym.Name))
		err = actor.replayWal(0)

		if err != nil {
			slog.Info(fmt.Sprintf("Replaying the %s orderbook Failed. Error: %s", sym.Name, err.Error()))
			continue
		}
		slog.Info(fmt.Sprintf("Replaying the %s orderbook Completed and the order count is %v", sym.Name, len(actor.engine.AllOrders)))

		// 3. Start other workers owned by actor
		go actor.wal.keepSyncing()
		go actor.kafkaEmitter.Run()
		// go actor.snapshotWorker() --> TODO

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

	replayCh := make(chan *AddOrderInternalResponse, 1)
	errCh := make(chan error, 1)
	actor.inbox <- PlaceOrderMsg{
		Order:  order,
		replay: replayCh,
		Err:    errCh,
	}

	select {
	case res := <-replayCh:
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

	replayCh := make(chan *CancelOrderInternalResponse, 1)
	errCh := make(chan error, 1)

	actor.inbox <- CancelOrderMsg{
		ID:     id,
		UserID: userID,
		Symbol: symbol,
		replay: replayCh,
		Err:    errCh,
	}

	select {
	case res := <-replayCh:
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

	replayCh := make(chan *ModifyOrderInternalResponse, 1)
	errCh := make(chan error, 1)

	actor.inbox <- ModifyOrderMsg{
		Symbol:         symbol,
		OrderID:        orderID,
		UserID:         userID,
		ClientModifyID: clientModifyID,
		NewPrice:       newPrice,
		NewQuantity:    newQuantity,
		replay:         replayCh,
		Err:            errCh,
	}

	select {
	case res := <-replayCh:
		return res, nil
	case err := <-errCh:
		return nil, err
	}
}

