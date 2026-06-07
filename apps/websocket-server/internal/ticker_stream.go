package internal

import (
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
)

func (wsg *WSGateway) subscribeTicker(user *User, symbol string) {
	if user.tickerSubs[symbol] {
		return
	}

	wsg.tickerStreamsMu.Lock()

	if _, exists := wsg.tickerStreams[symbol]; exists {
		wsg.tickerUsersMu.Lock()
		wsg.tickerUsers[symbol][user] = true
		wsg.tickerUsersMu.Unlock()
		wsg.tickerStreamsMu.Unlock()

		user.tickerSubs[symbol] = true
		slog.Info("user joined ticker stream", "user", user.ID, "symbol", symbol)
		return
	}

	pubsub := wsg.redisClient.Subscribe(wsg.ctx, tickerKey(symbol))
	wsg.tickerStreams[symbol] = pubsub

	wsg.tickerUsersMu.Lock()
	wsg.tickerUsers[symbol] = map[*User]bool{user: true}
	wsg.tickerUsersMu.Unlock()

	wsg.tickerStreamsMu.Unlock()

	user.tickerSubs[symbol] = true
	slog.Info("user started ticker stream", "user", user.ID, "symbol", symbol)
	go wsg.receiveTickerFromRedis(symbol, pubsub)
}

func (wsg *WSGateway) unsubscribeTicker(user *User, symbol string) {
	wsg.tickerStreamsMu.Lock()
	defer wsg.tickerStreamsMu.Unlock()

	wsg.tickerUsersMu.Lock()
	delete(wsg.tickerUsers[symbol], user)
	remaining := len(wsg.tickerUsers[symbol])
	wsg.tickerUsersMu.Unlock()

	delete(user.tickerSubs, symbol)

	if remaining == 0 {
		if pubsub, exists := wsg.tickerStreams[symbol]; exists {
			pubsub.Close()
			delete(wsg.tickerStreams, symbol)
		}
		delete(wsg.tickerUsers, symbol)
		slog.Info("closed ticker stream (no subscribers)", "symbol", symbol)
	}
}

func (wsg *WSGateway) receiveTickerFromRedis(symbol string, pubsub *redis.PubSub) {
	defer func() {
		wsg.tickerStreamsMu.Lock()
		delete(wsg.tickerStreams, symbol)
		wsg.tickerStreamsMu.Unlock()

		wsg.tickerUsersMu.RLock()
		hasUsers := len(wsg.tickerUsers[symbol]) > 0
		wsg.tickerUsersMu.RUnlock()

		if hasUsers && wsg.ctx.Err() == nil {
			slog.Warn("ticker stream dropped, reconnecting", "symbol", symbol)
			time.Sleep(reconnectDelay)
			go wsg.reconnectTickerStream(symbol)
		}
	}()

	for msg := range pubsub.Channel() {
		event := &Event{EventType: pbType.EventType_TICKER, Data: []byte(msg.Payload)}

		wsg.tickerUsersMu.RLock()
		snapshot := make([]*User, 0, len(wsg.tickerUsers[symbol]))
		for u := range wsg.tickerUsers[symbol] {
			snapshot = append(snapshot, u)
		}
		wsg.tickerUsersMu.RUnlock()

		for _, u := range snapshot {
			u.emit(event)
		}
	}
}

func (wsg *WSGateway) reconnectTickerStream(symbol string) {
	wsg.tickerUsersMu.RLock()
	hasUsers := len(wsg.tickerUsers[symbol]) > 0
	wsg.tickerUsersMu.RUnlock()

	if !hasUsers || wsg.ctx.Err() != nil {
		return
	}

	pubsub := wsg.redisClient.Subscribe(wsg.ctx, tickerKey(symbol))

	wsg.tickerStreamsMu.Lock()
	wsg.tickerStreams[symbol] = pubsub
	wsg.tickerStreamsMu.Unlock()

	go wsg.receiveTickerFromRedis(symbol, pubsub)
}
