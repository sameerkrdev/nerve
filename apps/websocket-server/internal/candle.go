package internal

import (
	"fmt"
	"strings"

	pbAgg "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
)

// EventTypeCandle is a synthetic sentinel — not in the proto EventType enum.
// Used to route candle events through the existing user.Channel without proto changes.
const EventTypeCandle = pbType.EventType(99)

// CandleWSPayload is the JSON envelope sent to WS clients for candle events.
// Pre-encoded once at fan-out time, sent as-is to all N subscribers (zero per-user work).
type CandleWSPayload struct {
	EventType string       `json:"eventType"`
	Symbol    string       `json:"symbol"`
	Timeframe string       `json:"timeframe"`
	Data      *pbAgg.Candle `json:"data"`
}

// candleKey produces the Redis pub/sub channel key.
// Must match the format used by candle-service/internal/memoryStore/redis.go.
func candleKey(symbol, timeframe string) string {
	return fmt.Sprintf("candles:%s:%s", strings.ToUpper(symbol), strings.ToLower(timeframe))
}

// parseKeyParts extracts (symbol, timeframe) from a candleKey string.
// e.g. "candles:BTCUSD:timeframe_1m" → ("BTCUSD", "timeframe_1m")
func parseKeyParts(key string) (symbol, timeframe string) {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 {
		return "", ""
	}
	return parts[1], parts[2]
}

// timeframeValid checks the client-supplied timeframe string against the proto enum.
func timeframeValid(tf string) bool {
	if tf == "" || tf == "TIMEFRAME_UNSPECIFIED" {
		return false
	}
	_, ok := pbAgg.Timeframe_value[tf]
	return ok
}
