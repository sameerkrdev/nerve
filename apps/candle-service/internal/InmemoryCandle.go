package internal

import (
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	matchingEnigne "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

/*

{
	"BTCUSD": {
		"1m":[
			*pb.Candle,
			*pb.Candle,
			*pb.Candle,
		],
	},
}

*/

type SymbolStore struct {
	current map[string]*pb.Candle
	history map[string][]*pb.Candle
}

type CandleInMemoryCache map[string]*SymbolStore

func NewCandleInMemoryCache() *CandleInMemoryCache {
	cache := make(CandleInMemoryCache)
	return &cache
}

func (m *CandleInMemoryCache) getOrCreateSymbol(symbol string) *SymbolStore {

	cache := *m

	if store, exists := cache[symbol]; exists {
		return store
	}

	store := &SymbolStore{
		current: make(map[string]*pb.Candle),
		history: make(map[string][]*pb.Candle),
	}

	cache[symbol] = store

	return store
}

func (m *CandleInMemoryCache) AddNewCandle(
	symbol string,
	tradeData *matchingEnigne.TradeEvent,
) {
	store := m.getOrCreateSymbol(symbol)

	timestamp := tradeData.Timestamp

	for key, value := range pb.Timeframe_value {
		current, exists := store.current[key]

		bucket := (timestamp.Seconds / int64(value)) * int64(value)

		if !exists {
			store.current[key] = m.newCandle(tradeData, key, value, bucket)

			continue
		}

		if current.OpenTime == bucket {
			m.updateCandle(current, tradeData)
			continue
		}

		//close the current candle if current trade passes the bucket time j
		current.IsClosed = true
		store.history[key] = append(store.history[key], current)

		trim(store.history[key])

		store.current[key] = m.newCandle(tradeData, key, value, bucket)
	}
}

func (m *CandleInMemoryCache) updateCandle(candle *pb.Candle, trade *matchingEnigne.TradeEvent) {
	price := float64(trade.Price / 100)

	if candle.H < price {
		candle.H = price
	}

	if candle.L > price {
		candle.L = price
	}

	candle.C = price
	candle.V += trade.Quantity
}

func (m *CandleInMemoryCache) newCandle(trade *matchingEnigne.TradeEvent, timeframeSting string, timeframeValue int32, bucket int64) *pb.Candle {
	candle := &pb.Candle{
		O:        float64(trade.Price / 100),
		H:        float64(trade.Price / 100),
		L:        float64(trade.Price / 100),
		C:        float64(trade.Price / 100),
		V:        trade.Quantity,
		OpenTime: bucket,
	}

	return candle
}

func (m *CandleInMemoryCache) GetCandles(
	symbol string,
	timeframe pb.Timeframe,
) []*pb.Candle {

	// cache := *m

	// store, exists := cache[symbol]
	// if !exists {
	// 	return []*pb.Candle{}
	// }

	// switch timeframe {

	// case pb.Timeframe_TIMEFRAME_1M:
	// 	return store.Candles1m

	// case pb.Timeframe_TIMEFRAME_5M:
	// 	return store.Candles5m

	// case pb.Timeframe_TIMEFRAME_15M:
	// 	return store.Candles15m

	// case pb.Timeframe_TIMEFRAME_1H:
	// 	return store.Candles1h

	// case pb.Timeframe_TIMEFRAME_4H:
	// 	return store.Candles4h

	// case pb.Timeframe_TIMEFRAME_1D:
	// 	return store.Candles1d
	// }

	return []*pb.Candle{}
}

func trim(candles []*pb.Candle) []*pb.Candle {

	if len(candles) > 500 {
		return candles[1:]
	}

	return candles
}
