package internal

import (
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

func (wsg *WSGateway) startOrderStream(user *User) {
	if user.orderSub != nil {
		return
	}
	pubsub := wsg.redis.Subscribe(wsg.ctx, orderKey(user.ID))
	user.orderSub = pubsub
	slog.Info("started order stream", "user", user.ID)
	go wsg.receiveOrderEvents(user, pubsub)
}

func (wsg *WSGateway) stopOrderStream(user *User) {
	if user.orderSub != nil {
		user.orderSub.Close()
		user.orderSub = nil
	}
}

func (wsg *WSGateway) receiveOrderEvents(user *User, pubsub *redis.PubSub) {
	defer func() {
		wsg.connectedUsersMu.RLock()
		_, stillConnected := wsg.connectedUsers[user.ID]
		wsg.connectedUsersMu.RUnlock()

		if stillConnected && wsg.ctx.Err() == nil {
			slog.Warn("order stream dropped, reconnecting", "user", user.ID)
			time.Sleep(reconnectDelay)
			wsg.startOrderStream(user)
		}
	}()

	for msg := range pubsub.Channel() {
		var engineEvent pb.EngineEvent
		if err := proto.Unmarshal([]byte(msg.Payload), &engineEvent); err != nil {
			slog.Error("order event unmarshal failed", "user", user.ID, "err", err)
			continue
		}
		user.emit(&Event{EventType: engineEvent.EventType, Data: engineEvent.Data})
	}
}
