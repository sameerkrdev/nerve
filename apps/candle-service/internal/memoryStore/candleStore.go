package memorystore

import (
	"fmt"

	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	matchingEnigne "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

type OnCandleClosedFn func(symbol, timeframe string, candle *pb.Candle)

/*

{
	"BTCUSD": {
		"current":{
			"1m": *pb.Candle,
		},
		history:{
			"1m":[
				*pb.Candle,
				*pb.Candle,
				*pb.Candle,
				.....
			],
		}
	},
	.....
}

*/

type SymbolStore struct {
	current map[string]*pb.Candle
	history map[string][]*pb.Candle
}

type CandleStore struct {
	store          map[string]*SymbolStore
	onCandleClosed OnCandleClosedFn
}

func NewCandleStore(onCandleClosed OnCandleClosedFn) *CandleStore {
	return &CandleStore{
		store:          make(map[string]*SymbolStore),
		onCandleClosed: onCandleClosed,
	}
}

func (cache *CandleStore) getOrCreateSymbol(symbol string) *SymbolStore {
	if store, exists := cache.store[symbol]; exists {
		return store
	}

	store := &SymbolStore{
		current: make(map[string]*pb.Candle),
		history: make(map[string][]*pb.Candle),
	}

	cache.store[symbol] = store

	return store
}

func (cache *CandleStore) AddNewCandle(
	symbol string,
	tradeData *matchingEnigne.TradeEvent,
) {
	store := cache.getOrCreateSymbol(symbol)

	timestamp := tradeData.Timestamp

	for tfName, tfSeconds := range pb.Timeframe_value {
		activeCandle, exists := store.current[tfName]

		openTimeBucket := (timestamp.Seconds / int64(tfSeconds)) * int64(tfSeconds)

		if !exists {
			store.current[tfName] = cache.newCandle(tradeData, tfName, tfSeconds, openTimeBucket)

			continue
		}

		if activeCandle.OpenTime == openTimeBucket {
			cache.updateCandle(activeCandle, tradeData)
			PublishCandleEventToRedis(symbol, tfName, store.current[tfName])
			continue
		}

		//close the current candle if current trade passes the bucket time
		activeCandle.IsClosed = true
		store.history[tfName] = append(store.history[tfName], activeCandle)

		store.current[tfName] = cache.newCandle(tradeData, tfName, tfSeconds, openTimeBucket)
		store.history[tfName] = trim(store.history[tfName])

		if cache.onCandleClosed != nil {
			cache.onCandleClosed(symbol, tfName, activeCandle)
		}
	}
}

func (cache *CandleStore) updateCandle(candle *pb.Candle, trade *matchingEnigne.TradeEvent) {
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

func (cache *CandleStore) newCandle(trade *matchingEnigne.TradeEvent, _ string, _ int32, openTimeBucket int64) *pb.Candle {
	candle := &pb.Candle{
		O:        float64(trade.Price / 100),
		H:        float64(trade.Price / 100),
		L:        float64(trade.Price / 100),
		C:        float64(trade.Price / 100),
		V:        trade.Quantity,
		OpenTime: openTimeBucket,
	}

	return candle
}

func trim(candles []*pb.Candle) []*pb.Candle {

	if len(candles) > 1000 {
		return candles[1:]
	}

	return candles
}

func (cache *CandleStore) GetCandles(
	symbol string,
	timeframe pb.Timeframe,
) ([]*pb.Candle, error) {

	store := cache.getOrCreateSymbol(symbol)

	tfName, ok := pb.Timeframe_name[int32(timeframe)]
	if !ok {
		return nil, fmt.Errorf("invalid interval")
	}

	// TODO: Implement the from, to feature :- inmemory, redis, clickhouse
	candles := store.history[tfName]

	return candles, nil
}
