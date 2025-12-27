package internal

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	pbTypes "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Order struct {
	Symbol            string
	Price             uint64
	Quantity          uint32
	RemainingQuantity uint32
	Side              pbTypes.Side
	Type              pbTypes.OrderType
	UserID            string
	ClientOrderID     string
	Status            pbTypes.OrderStatus
	ClientTimestamp   *timestamppb.Timestamp
	GatewayTimestamp  *timestamppb.Timestamp
	EngineTimestamp   time.Time

	Prev *Order
	Next *Order

	PriceLevel *PriceLevel
}

/*
===================================================================================
========== Price Level (Doubly Linked List --> Order FIFO) Management =============
===================================================================================
*/
type PriceLevel struct {
	Price       uint64
	TotalVolume int
	OrderCount  int
	HeadOrder   *Order
	TailOrder   *Order

	PrevPrice *PriceLevel
	NextPrice *PriceLevel
}

func (pl *PriceLevel) Push(order *Order) {
	order.PriceLevel = pl

	if pl.HeadOrder == nil {
		pl.HeadOrder = order
		pl.TailOrder = order
	} else {
		order.Next = nil
		order.Prev = pl.TailOrder
		pl.TailOrder.Next = order
	}

	pl.TotalVolume += int(order.Quantity)
	pl.OrderCount++
}

func (pl *PriceLevel) Remove(order *Order) {
	if order.Prev != nil {
		order.Prev.Next = order.Next
	} else {
		order.Next.Prev = nil
		pl.HeadOrder = order.Next
	}

	if order.Next != nil {
		order.Next.Prev = order.Prev
	} else {
		order.Prev.Next = nil
		pl.TailOrder = order.Prev
	}

	pl.TotalVolume -= int(order.Quantity)
	pl.OrderCount--

	order.Prev = nil
	order.Next = nil
	order.PriceLevel = nil
}

func (pl *PriceLevel) IsEmpty() bool {
	return pl.HeadOrder == nil
}

/*
==================================================================
================== Order Book Map Management =====================
==================================================================
*/
type OrderBookSide struct {
	Side        pbTypes.Side
	PriceLevels map[int]*PriceLevel

	BestPriceLevel *PriceLevel
}

func NewOrderBookSide(side pbTypes.Side) *OrderBookSide {
	return &OrderBookSide{
		Side:        side,
		PriceLevels: make(map[int]*PriceLevel),
	}
}

func (obs *OrderBookSide) GetOrCreateOrderBookSide(price int) *PriceLevel {
	level, exists := obs.PriceLevels[price]

	if !exists {
		level := &PriceLevel{Price: uint64(price)}
		obs.PriceLevels[price] = level
		obs.LinkPriceLevel(level)
	}

	return level
}

func (obs *OrderBookSide) LinkPriceLevel(newLevel *PriceLevel) {
	if obs.BestPriceLevel == nil {
		obs.BestPriceLevel = newLevel
		return
	}

	currentBestPriceLevel := obs.BestPriceLevel
	if obs.Side == pbTypes.Side_SELL {
		// Bids: descending order (100, 99, 98...)
		if newLevel.Price > currentBestPriceLevel.Price {
			newLevel.NextPrice = currentBestPriceLevel
			currentBestPriceLevel.PrevPrice = newLevel
			obs.BestPriceLevel = newLevel

			return
		}

		for currentBestPriceLevel != nil {
			if currentBestPriceLevel.NextPrice == nil || newLevel.Price > currentBestPriceLevel.Price {
				newLevel.PrevPrice = currentBestPriceLevel
				newLevel.NextPrice = currentBestPriceLevel.NextPrice

				if currentBestPriceLevel.NextPrice.PrevPrice != nil {
					currentBestPriceLevel.NextPrice.PrevPrice = newLevel
				}
				currentBestPriceLevel.NextPrice = newLevel
				return
			}

			currentBestPriceLevel = currentBestPriceLevel.NextPrice
		}
	} else {
		if newLevel.Price < currentBestPriceLevel.Price {
			newLevel.NextPrice = currentBestPriceLevel
			currentBestPriceLevel.PrevPrice = newLevel
			obs.BestPriceLevel = newLevel
			return
		}

		for currentBestPriceLevel != nil {
			if currentBestPriceLevel.NextPrice == nil || newLevel.Price < currentBestPriceLevel.Price {
				newLevel.PrevPrice = currentBestPriceLevel
				newLevel.NextPrice = currentBestPriceLevel.NextPrice

				if currentBestPriceLevel.NextPrice.PrevPrice != nil {
					currentBestPriceLevel.NextPrice.PrevPrice = newLevel
				}
				currentBestPriceLevel.NextPrice = newLevel
				return
			}
			currentBestPriceLevel = currentBestPriceLevel.NextPrice
		}
	}
}

func (obs *OrderBookSide) RemovePriceLevel(level *PriceLevel) error {
	if !level.IsEmpty() {
		return fmt.Errorf("Failed to remove PriceLevel because it contains orders")
	}

	if level.NextPrice != nil {
		level.NextPrice.PrevPrice = level.PrevPrice
	}

	if level.PrevPrice != nil {
		level.PrevPrice.NextPrice = level.NextPrice
	}

	if level == obs.BestPriceLevel {
		obs.BestPriceLevel = level.NextPrice
	}

	level.NextPrice = nil
	level.PrevPrice = nil

	delete(obs.PriceLevels, int(level.Price))

	return nil
}

/*
=================================================================================
================== Single Symbol Matching Engine Management =====================
=================================================================================
*/
type MatchingEngine struct {
	Symbol string
	Bids   *OrderBookSide
	Asks   *OrderBookSide

	AllOrders map[string]*Order

	TotalMatches  uint64
	TotalVolume   uint64
	TradeSequence uint64
	mu            sync.RWMutex
}

func NewMatchingEnigne(symbol string) *MatchingEngine {
	return &MatchingEngine{
		Symbol:        symbol,
		Bids:          NewOrderBookSide(pbTypes.Side_SELL),
		Asks:          NewOrderBookSide(pbTypes.Side_BUY),
		AllOrders:     make(map[string]*Order),
		TotalMatches:  0,
		TotalVolume:   0,
		TradeSequence: 0,
	}
}

func (me *MatchingEngine) AddOrder(order *Order) error {
	me.mu.Lock()
	defer me.mu.Unlock()

	if _, exists := me.AllOrders[order.ClientOrderID]; exists {
		slog.Error("Duplicate Order ID",
			"clientOrderID", order.ClientOrderID,
			"symbol", order.Symbol,
			"price", order.Price,
			"side", order.Side,
		)
		return fmt.Errorf("Duplicate Order ID: %s", order.ClientOrderID)
	}

	me.MatchOrder(order)

	if order.RemainingQuantity > 0 && order.Type != pbTypes.OrderType_MARKET {
		var obs *OrderBookSide

		if order.Side == pbTypes.Side_BUY {
			obs = me.Asks
		} else {
			obs = me.Bids
		}

		level := obs.GetOrCreateOrderBookSide(int(order.Price))

		order.Status = pbTypes.OrderStatus_PARTIAL_FILLED

		level.Push(order)
		me.AllOrders[order.ClientOrderID] = order
	}

	return nil
}

func (me *MatchingEngine) MatchOrder(incoming *Order) []Trade {
	var oppositeBook *OrderBookSide

	if incoming.Side == pbTypes.Side_BUY {
		oppositeBook = me.Asks
	} else {
		oppositeBook = me.Bids
	}

	trades := []Trade{}

	for incoming.RemainingQuantity > 0 && me.CanMatch(oppositeBook, incoming) {
		bestPriceLevel := oppositeBook.BestPriceLevel
		restingOrder := bestPriceLevel.HeadOrder

		matchQuantity := min(incoming.RemainingQuantity, restingOrder.Quantity)
		matchPrice := restingOrder.Price

		trade := me.ExecuteTrade(incoming, restingOrder, matchQuantity, matchPrice)

		trades = append(trades, trade)

		incoming.RemainingQuantity -= matchQuantity
		restingOrder.RemainingQuantity -= matchQuantity

		atomic.AddUint64(&me.TotalMatches, 1)
		atomic.AddUint64(&me.TotalVolume, uint64(matchQuantity))

		if restingOrder.RemainingQuantity == 0 {
			bestPriceLevel.Remove(restingOrder)

			delete(me.AllOrders, restingOrder.ClientOrderID)

			if bestPriceLevel.IsEmpty() {
				oppositeBook.RemovePriceLevel(bestPriceLevel) // err handle
			}
		}
	}
	return trades
}

func (me *MatchingEngine) CanMatch(oppositeBook *OrderBookSide, incoming *Order) bool {
	if oppositeBook.BestPriceLevel == nil {
		return false
	}
	if incoming.Type == pbTypes.OrderType_MARKET {
		return true
	}

	bestPrice := oppositeBook.BestPriceLevel.Price
	side := oppositeBook.Side

	if side == pbTypes.Side_BUY {
		return bestPrice <= incoming.Price
	}

	return bestPrice >= incoming.Price
}

type Trade struct {
	TradeID       string
	Symbol        string
	TradeSequence uint64
	Price         uint64
	Quantity      uint32
	Timeline      time.Time

	RestingOrderID   string
	AggressorOrderID string

	AggressorOrder *Order
	RestingOrder   *Order

	BuyerID     string
	SellerID    string
	BuyOrderID  string
	SellOrderID string
}

func (me *MatchingEngine) ExecuteTrade(aggressor *Order, restingOrder *Order, matchQuantity uint32, matchPrice uint64) Trade {
	seq := atomic.AddUint64(&me.TradeSequence, 1)

	tradeID := me.GenerateTradeID(seq)

	var (
		BuyerID     string
		SellerID    string
		BuyOrderID  string
		SellOrderID string
	)

	if aggressor.Side == pbTypes.Side_SELL {
		SellOrderID = aggressor.ClientOrderID
		SellerID = aggressor.UserID
		BuyOrderID = restingOrder.ClientOrderID
		BuyerID = restingOrder.UserID
	} else {
		BuyOrderID = aggressor.ClientOrderID
		BuyerID = aggressor.UserID
		SellOrderID = restingOrder.ClientOrderID
		SellerID = restingOrder.UserID
	}

	return Trade{
		TradeID:       tradeID,
		Symbol:        aggressor.Symbol,
		TradeSequence: seq,
		Price:         matchPrice,
		Quantity:      matchQuantity,
		Timeline:      time.Now(),

		RestingOrderID:   restingOrder.ClientOrderID,
		AggressorOrderID: aggressor.ClientOrderID,
		RestingOrder:     restingOrder,
		AggressorOrder:   aggressor,

		BuyOrderID:  BuyOrderID,
		BuyerID:     BuyerID,
		SellOrderID: SellOrderID,
		SellerID:    SellerID,
	}
}

// generateTradeID creates a unique trade ID
func (me *MatchingEngine) GenerateTradeID(seq uint64) string {
	return fmt.Sprintf("%s-T%d-%d", me.Symbol, time.Now().UnixNano(), seq)
}

/*
=================================================================================
========== Multiple Symbol Matching Engine Management via Actor Model ===========
=================================================================================
*/
