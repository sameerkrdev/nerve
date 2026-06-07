package internal

import (
	"context"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"
	pbTypes "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

var redisClient *redis.Client

func InitRedis(url string) error {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return err
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		return err
	}
	redisClient = c
	return nil
}

// Non-fatal: logs warn on error so matching loop is never blocked.
func PublishEngineEvent(event *pb.EngineEvent) {
	if redisClient == nil {
		return
	}
	ctx := context.Background()
	sym := strings.ToUpper(event.Symbol)

	switch event.EventType {
	case pbTypes.EventType_DEPTH:
		if err := redisClient.Publish(ctx, "depth:"+sym, event.Data).Err(); err != nil {
			slog.Warn("redis publish depth failed", "symbol", sym, "err", err)
		}

	case pbTypes.EventType_TICKER:
		if err := redisClient.Publish(ctx, "ticker:"+sym, event.Data).Err(); err != nil {
			slog.Warn("redis publish ticker failed", "symbol", sym, "err", err)
		}

	case pbTypes.EventType_TRADE_EXECUTED:
		var trade pb.TradeEvent
		if err := proto.Unmarshal(event.Data, &trade); err != nil {
			slog.Error("redis: unmarshal trade event failed", "err", err)
			return
		}
		data, err := proto.Marshal(event)
		if err != nil {
			slog.Error("redis: marshal engine event failed", "err", err)
			return
		}
		if err := redisClient.Publish(ctx, "order:"+trade.BuyerId, data).Err(); err != nil {
			slog.Warn("redis publish trade→buyer failed", "buyer", trade.BuyerId, "err", err)
		}
		if err := redisClient.Publish(ctx, "order:"+trade.SellerId, data).Err(); err != nil {
			slog.Warn("redis publish trade→seller failed", "seller", trade.SellerId, "err", err)
		}

	default:
		if event.UserId == "" {
			return
		}
		data, err := proto.Marshal(event)
		if err != nil {
			slog.Error("redis: marshal engine event failed", "err", err)
			return
		}
		if err := redisClient.Publish(ctx, "order:"+event.UserId, data).Err(); err != nil {
			slog.Warn("redis publish order event failed", "user", event.UserId, "err", err)
		}
	}
}
