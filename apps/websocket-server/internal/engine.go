package internal

import (
	"log/slog"
	"time"

	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

// subscribeEngineEvents adds user to symbol fan-out set and starts a gRPC stream if needed.
func (wsg *WSGateway) subscribeEngineEvents(user *User, symbol string) {
	wsg.startEngineEventsStream(symbol)

	wsg.symbolUsersMu.Lock()
	if wsg.symbolUsers[symbol] == nil {
		wsg.symbolUsers[symbol] = make(map[*User]bool)
	}
	wsg.symbolUsers[symbol][user] = true
	wsg.symbolUsersMu.Unlock()

	user.symbols[symbol] = true
	slog.Info("user subscribed to symbol", "user", user.ID, "symbol", symbol)
}

// unsubscribeEngineEvents removes user from symbol fan-out. Closes stream when last user leaves.
func (wsg *WSGateway) unsubscribeEngineEvents(user *User, symbol string) {
	wsg.symbolUsersMu.Lock()

	users := wsg.symbolUsers[symbol]
	delete(users, user)

	if len(users) == 0 {
		delete(wsg.symbolUsers, symbol)

		wsg.symbolStreamsMu.Lock()
		delete(wsg.symbolStreams, symbol)
		wsg.symbolStreamsMu.Unlock()

		slog.Info("closed symbol stream (no subscribers)", "symbol", symbol)
	}

	wsg.symbolUsersMu.Unlock()
	delete(user.symbols, symbol)
}

// startEngineEventsStream creates a gRPC stream for engine events if one doesn't exist.
func (wsg *WSGateway) startEngineEventsStream(symbol string) {
	wsg.symbolStreamsMu.Lock()
	defer wsg.symbolStreamsMu.Unlock()

	if _, exists := wsg.symbolStreams[symbol]; exists {
		return
	}

	client := pb.NewMatchingEngineClient(wsg.engineConn)
	stream, err := client.SubscribeSymbol(wsg.ctx, &pb.SubscribeRequest{
		Symbol:    symbol,
		GatewayId: "ws-gateway",
		Timestamp: time.Now().UnixNano(),
	})
	if err != nil {
		slog.Error("failed to subscribe to engine events stream", "symbol", symbol, "err", err)
		return
	}

	wsg.symbolStreams[symbol] = stream
	slog.Info("started engine events stream", "symbol", symbol)
	go wsg.receiveFromEngineEventsStream(symbol, stream)
}

// receiveFromEngineEventsStream reads events from the gRPC stream and fans out to users.
// Reconnects on error if subscribers still exist.
func (wsg *WSGateway) receiveFromEngineEventsStream(symbol string, stream pb.MatchingEngine_SubscribeSymbolClient) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			slog.Warn("engine events stream error", "symbol", symbol, "err", err)

			wsg.symbolStreamsMu.Lock()
			delete(wsg.symbolStreams, symbol)
			wsg.symbolStreamsMu.Unlock()

			// only reconnect if users still subscribed and gateway not shutting down
			wsg.symbolUsersMu.RLock()
			hasUsers := len(wsg.symbolUsers[symbol]) > 0
			wsg.symbolUsersMu.RUnlock()

			if hasUsers && wsg.ctx.Err() == nil {
				time.Sleep(reconnectDelay)
				wsg.startEngineEventsStream(symbol)
			}
			return
		}

		wsg.broadcastEngineEvents(symbol, msg)
	}
}

// broadcastEngineEvents routes an engine event to the appropriate users.
func (wsg *WSGateway) broadcastEngineEvents(symbol string, msg *pb.EngineEvent) {
	event := &Event{EventType: msg.EventType, Data: msg.Data}

	switch msg.EventType {
	// order events → specific user by ID
	case pbType.EventType_ORDER_ACCEPTED,
		pbType.EventType_ORDER_CANCELLED,
		pbType.EventType_ORDER_FILLED,
		pbType.EventType_ORDER_REJECTED,
		pbType.EventType_ORDER_REDUCED:

		wsg.connectedUsersMu.RLock()
		user, exists := wsg.connectedUsers[msg.UserId]
		wsg.connectedUsersMu.RUnlock()

		if exists {
			user.emit(event)
		}

	// trade events → buyer and seller
	case pbType.EventType_TRADE_EXECUTED:
		var trade pb.TradeEvent
		if err := proto.Unmarshal(msg.Data, &trade); err != nil {
			slog.Error("failed to unmarshal trade event", "symbol", symbol, "err", err)
			return
		}

		wsg.connectedUsersMu.RLock()
		buyer, hasBuyer := wsg.connectedUsers[trade.BuyerId]
		seller, hasSeller := wsg.connectedUsers[trade.SellerId]
		wsg.connectedUsersMu.RUnlock()

		if hasBuyer {
			buyer.emit(event)
		}
		if hasSeller {
			seller.emit(event)
		}

	// depth / ticker → all symbol subscribers
	case pbType.EventType_DEPTH, pbType.EventType_TICKER:
		wsg.symbolUsersMu.RLock()
		snapshot := make([]*User, 0, len(wsg.symbolUsers[symbol]))
		for u := range wsg.symbolUsers[symbol] {
			snapshot = append(snapshot, u)
		}
		wsg.symbolUsersMu.RUnlock()

		for _, u := range snapshot {
			u.emit(event)
		}
	}
}
