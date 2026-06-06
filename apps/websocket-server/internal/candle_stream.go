package internal

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	pbAgg "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	"google.golang.org/protobuf/proto"
)

// handleSubscribeCandles validates and subscribes user to a candle stream.
func (wsg *WSGateway) handleSubscribeCandles(user *User, msg UserMessage) {
	if msg.Symbol == "" || !timeframeValid(msg.Timeframe) {
		slog.Warn("invalid subscribe_candles request", "user", user.ID, "symbol", msg.Symbol, "timeframe", msg.Timeframe)
		return
	}
	wsg.subscribeToCandle(user, msg.Symbol, msg.Timeframe)
}

// handleUnsubscribeCandles removes user from a candle stream.
func (wsg *WSGateway) handleUnsubscribeCandles(user *User, msg UserMessage) {
	if msg.Symbol == "" || msg.Timeframe == "" {
		return
	}
	key := candleKey(msg.Symbol, msg.Timeframe)
	if user.candles[key] {
		wsg.unsubscribeCandle(user, key)
	}
}

// subscribeToCandle adds user to the fan-out set for key, starting the Redis PubSub if needed.
func (wsg *WSGateway) subscribeToCandle(user *User, symbol, timeframe string) {
	key := candleKey(symbol, timeframe)

	if user.candles[key] {
		return // idempotent
	}

	wsg.candleStreamsMu.Lock()

	if _, exists := wsg.candleStreams[key]; exists {
		// goroutine already running — add user to fan-out set
		wsg.candleUsersMu.Lock()
		wsg.candleUsers[key][user] = true
		wsg.candleUsersMu.Unlock()
		wsg.candleStreamsMu.Unlock()

		user.candles[key] = true
		slog.Info("user joined candle stream", "user", user.ID, "key", key)
		return
	}

	// first subscriber — start PubSub goroutine
	pubsub := wsg.redisClient.Subscribe(wsg.ctx, key)
	wsg.candleStreams[key] = pubsub

	wsg.candleUsersMu.Lock()
	wsg.candleUsers[key] = map[*User]bool{user: true}
	wsg.candleUsersMu.Unlock()

	wsg.candleStreamsMu.Unlock()

	user.candles[key] = true
	slog.Info("user started candle stream", "user", user.ID, "key", key)
	go wsg.receiveCandleFromRedis(key, pubsub)
}

// unsubscribeCandle removes user from a candle stream by key.
// Closes the PubSub when no subscribers remain.
func (wsg *WSGateway) unsubscribeCandle(user *User, key string) {
	wsg.candleStreamsMu.Lock()
	defer wsg.candleStreamsMu.Unlock()

	wsg.candleUsersMu.Lock()
	delete(wsg.candleUsers[key], user)
	remaining := len(wsg.candleUsers[key])
	wsg.candleUsersMu.Unlock()

	delete(user.candles, key)

	if remaining == 0 {
		if pubsub, exists := wsg.candleStreams[key]; exists {
			pubsub.Close()
			delete(wsg.candleStreams, key)
		}
		delete(wsg.candleUsers, key)
		slog.Info("closed candle stream (no subscribers)", "key", key)
	}
}

// receiveCandleFromRedis reads from pubsub, JSON-encodes once, fans out to all subscribers.
// Reconnects automatically on Redis drop (only if subscribers still exist).
func (wsg *WSGateway) receiveCandleFromRedis(key string, pubsub *redis.PubSub) {
	defer func() {
		wsg.candleStreamsMu.Lock()
		delete(wsg.candleStreams, key)
		wsg.candleStreamsMu.Unlock()

		wsg.candleUsersMu.RLock()
		hasUsers := len(wsg.candleUsers[key]) > 0
		wsg.candleUsersMu.RUnlock()

		if hasUsers && wsg.ctx.Err() == nil {
			slog.Warn("candle stream dropped, reconnecting", "key", key)
			time.Sleep(reconnectDelay)
			go wsg.reconnectCandleStream(key)
		}
	}()

	symbol, timeframe := parseKeyParts(key)

	for msg := range pubsub.Channel() {
		candle := &pbAgg.Candle{}
		if err := proto.Unmarshal([]byte(msg.Payload), candle); err != nil {
			slog.Error("candle unmarshal failed", "key", key, "err", err)
			continue
		}

		// encode JSON once, reuse across all N subscribers
		payload, err := json.Marshal(&CandleWSPayload{
			EventType: "candle",
			Symbol:    symbol,
			Timeframe: strings.ToUpper(timeframe),
			Data:      candle,
		})
		if err != nil {
			slog.Error("candle json marshal failed", "err", err)
			continue
		}

		event := &Event{EventType: EventTypeCandle, Data: payload}

		wsg.candleUsersMu.RLock()
		snapshot := make([]*User, 0, len(wsg.candleUsers[key]))
		for u := range wsg.candleUsers[key] {
			snapshot = append(snapshot, u)
		}
		wsg.candleUsersMu.RUnlock()

		for _, u := range snapshot {
			u.emit(event)
		}
	}
}

// reconnectCandleStream re-establishes a Redis PubSub for key if subscribers still exist.
func (wsg *WSGateway) reconnectCandleStream(key string) {
	wsg.candleUsersMu.RLock()
	hasUsers := len(wsg.candleUsers[key]) > 0
	wsg.candleUsersMu.RUnlock()

	if !hasUsers || wsg.ctx.Err() != nil {
		return
	}

	pubsub := wsg.redisClient.Subscribe(wsg.ctx, key)

	wsg.candleStreamsMu.Lock()
	wsg.candleStreams[key] = pubsub
	wsg.candleStreamsMu.Unlock()

	go wsg.receiveCandleFromRedis(key, pubsub)
}
