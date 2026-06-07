package internal

import (
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
)

func (wsg *WSGateway) subscribeDepth(user *User, symbol string) {
	if user.depthSubs[symbol] {
		return
	}

	wsg.depthStreamsMu.Lock()

	if _, exists := wsg.depthStreams[symbol]; exists {
		wsg.depthUsersMu.Lock()
		wsg.depthUsers[symbol][user] = true
		wsg.depthUsersMu.Unlock()
		wsg.depthStreamsMu.Unlock()

		user.depthSubs[symbol] = true
		slog.Info("user joined depth stream", "user", user.ID, "symbol", symbol)
		return
	}

	pubsub := wsg.redisClient.Subscribe(wsg.ctx, depthKey(symbol))
	wsg.depthStreams[symbol] = pubsub

	wsg.depthUsersMu.Lock()
	wsg.depthUsers[symbol] = map[*User]bool{user: true}
	wsg.depthUsersMu.Unlock()

	wsg.depthStreamsMu.Unlock()

	user.depthSubs[symbol] = true
	slog.Info("user started depth stream", "user", user.ID, "symbol", symbol)
	go wsg.receiveDepthFromRedis(symbol, pubsub)
}

func (wsg *WSGateway) unsubscribeDepth(user *User, symbol string) {
	wsg.depthStreamsMu.Lock()
	defer wsg.depthStreamsMu.Unlock()

	wsg.depthUsersMu.Lock()
	delete(wsg.depthUsers[symbol], user)
	remaining := len(wsg.depthUsers[symbol])
	wsg.depthUsersMu.Unlock()

	delete(user.depthSubs, symbol)

	if remaining == 0 {
		if pubsub, exists := wsg.depthStreams[symbol]; exists {
			pubsub.Close()
			delete(wsg.depthStreams, symbol)
		}
		delete(wsg.depthUsers, symbol)
		slog.Info("closed depth stream (no subscribers)", "symbol", symbol)
	}
}

func (wsg *WSGateway) receiveDepthFromRedis(symbol string, pubsub *redis.PubSub) {
	defer func() {
		wsg.depthStreamsMu.Lock()
		delete(wsg.depthStreams, symbol)
		wsg.depthStreamsMu.Unlock()

		wsg.depthUsersMu.RLock()
		hasUsers := len(wsg.depthUsers[symbol]) > 0
		wsg.depthUsersMu.RUnlock()

		if hasUsers && wsg.ctx.Err() == nil {
			slog.Warn("depth stream dropped, reconnecting", "symbol", symbol)
			time.Sleep(reconnectDelay)
			go wsg.reconnectDepthStream(symbol)
		}
	}()

	for msg := range pubsub.Channel() {
		event := &Event{EventType: pbType.EventType_DEPTH, Data: []byte(msg.Payload)}

		wsg.depthUsersMu.RLock()
		snapshot := make([]*User, 0, len(wsg.depthUsers[symbol]))
		for u := range wsg.depthUsers[symbol] {
			snapshot = append(snapshot, u)
		}
		wsg.depthUsersMu.RUnlock()

		for _, u := range snapshot {
			u.emit(event)
		}
	}
}

func (wsg *WSGateway) reconnectDepthStream(symbol string) {
	wsg.depthUsersMu.RLock()
	hasUsers := len(wsg.depthUsers[symbol]) > 0
	wsg.depthUsersMu.RUnlock()

	if !hasUsers || wsg.ctx.Err() != nil {
		return
	}

	pubsub := wsg.redisClient.Subscribe(wsg.ctx, depthKey(symbol))

	wsg.depthStreamsMu.Lock()
	wsg.depthStreams[symbol] = pubsub
	wsg.depthStreamsMu.Unlock()

	go wsg.receiveDepthFromRedis(symbol, pubsub)
}
