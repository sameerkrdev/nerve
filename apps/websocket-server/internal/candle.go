package internal

import (
	"encoding/json"
	"fmt"
	"strings"

	pbAgg "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	"google.golang.org/protobuf/proto"
)

// Synthetic EventType sentinels — not in the proto EventType enum.
const EventTypeCandle = pbType.EventType(99)
const EventTypeError = pbType.EventType(98)

type CandleWSPayload struct {
	EventType string        `json:"eventType"`
	Symbol    string        `json:"symbol"`
	Timeframe string        `json:"timeframe"`
	Data      *pbAgg.Candle `json:"data"`
}

func makeCandleEvent(key string, data []byte) (*Event, error) {
	symbol, timeframe := parseKeyParts(key)
	candle := &pbAgg.Candle{}
	if err := proto.Unmarshal(data, candle); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(&CandleWSPayload{
		EventType: "candle",
		Symbol:    symbol,
		Timeframe: strings.ToUpper(timeframe),
		Data:      candle,
	})
	if err != nil {
		return nil, err
	}
	return &Event{EventType: EventTypeCandle, Data: payload}, nil
}

func candleKey(symbol, timeframe string) string {
	return fmt.Sprintf("candles:%s:%s", strings.ToUpper(symbol), strings.ToLower(timeframe))
}

func depthKey(symbol string) string  { return "depth:" + strings.ToUpper(symbol) }
func tickerKey(symbol string) string { return "ticker:" + strings.ToUpper(symbol) }
func orderKey(userID string) string  { return "order:" + userID }

func parseKeyParts(key string) (symbol, timeframe string) {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 {
		return "", ""
	}
	return parts[1], parts[2]
}

func timeframeValid(tf string) bool {
	if tf == "" || tf == "TIMEFRAME_UNSPECIFIED" {
		return false
	}
	_, ok := pbAgg.Timeframe_value[tf]
	return ok
}
