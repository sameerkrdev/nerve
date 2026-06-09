package memorystore

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	"google.golang.org/protobuf/proto"
)

var (
	RedisClient *redis.Client
	once        sync.Once
	ctx         = context.Background()
	initErr     error
)

func InitRedis() error {
	once.Do(func() {
		REDIS_URL := os.Getenv("REDIS_URL")
		if REDIS_URL == "" {
			initErr = fmt.Errorf("REDIS_URL is required")
			return
		}

		opt, err := redis.ParseURL(REDIS_URL)
		if err != nil {
			initErr = fmt.Errorf("parse error %w", err)
			return
		}

		client := redis.NewClient(opt)

		if e := client.Ping(ctx).Err(); e != nil {
			initErr = fmt.Errorf("connection error: %w", e)
			return
		}

		slog.Info("redis connected", "addr", opt.Addr)

		RedisClient = client
	})
	return initErr
}

func candleKey(symbol, timeframe string) string {
	return fmt.Sprintf("candles:%s:%s",
		strings.ToUpper(symbol),
		strings.ToLower(timeframe),
	)
}

func PushCandle(symbol, timeframe string, candle *pb.Candle) error {
	data, err := proto.Marshal(candle)
	if err != nil {
		return fmt.Errorf("marshal candle: %w", err)
	}

	key := candleKey(symbol, timeframe)
	// err = RedisClient.LPush(ctx, key, data).Err()
	// if err != nil {
	// 	return err
	// }

	// return RedisClient.LTrim(ctx, key, 0, 4999).Err()

	pipe := RedisClient.TxPipeline()

	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, 4999)

	_, err = pipe.Exec(ctx)
	return err
}

func GetCandlesFromRedis(symbol, timeframe string, count int64) ([]*pb.Candle, error) {
	results, err := RedisClient.LRange(ctx, candleKey(symbol, timeframe), 0, count-1).Result()
	if err != nil {
		return nil, fmt.Errorf("redis LRange: %w", err)
	}

	candles := make([]*pb.Candle, 0, len(results))
	for _, r := range results {
		c := &pb.Candle{}
		if err := proto.Unmarshal([]byte(r), c); err != nil {
			return nil, fmt.Errorf("unmarshal candle: %w", err)
		}
		candles = append(candles, c)
	}
	return candles, nil
}

func PublishCandleEventToRedis(symbol string, timeframe string, candle *pb.Candle) {
	data, err := proto.Marshal(candle)
	if err != nil {
		slog.Error("failed to marshal candle for publish", "error", err)
		return
	}
	if err := RedisClient.Publish(ctx, candleKey(symbol, timeframe), data).Err(); err != nil {
		slog.Error("failed to publish candle event", "error", err)
	}
}
