package internal

import (
	"fmt"
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

	OrderSequence uint64

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
	TotalVolume uint64
	OrderCount  uint64
	HeadOrder   *Order
	TailOrder   *Order

	PrevPrice *PriceLevel
	NextPrice *PriceLevel
}

func (pl *PriceLevel) Push(order *Order) {
	order.PriceLevel = pl

	if pl.TailOrder == nil {
		pl.HeadOrder = order
		pl.TailOrder = order
	} else {
		pl.TailOrder.Next = order
		order.Prev = pl.TailOrder
		pl.TailOrder = order
	}

	pl.TotalVolume += uint64(order.Quantity)
	pl.OrderCount++
}

func (pl *PriceLevel) Remove(order *Order) {
	if order.Prev != nil {
		order.Prev.Next = order.Next
	} else {
		pl.HeadOrder = order.Next
	}

	if order.Next != nil {
		order.Next.Prev = order.Prev
	} else {
		pl.TailOrder = order.Prev
	}

	pl.TotalVolume -= uint64(order.Quantity)
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
	PriceLevels map[uint64]*PriceLevel

	BestPriceLevel *PriceLevel
}

func NewOrderBookSide(side pbTypes.Side) *OrderBookSide {
	return &OrderBookSide{
		Side:        side,
		PriceLevels: make(map[uint64]*PriceLevel),
	}
}

func (obs *OrderBookSide) IsEmpty() bool {
	return obs.BestPriceLevel == nil
}

func (obs *OrderBookSide) GetOrCreatePriceLevel(price uint64) *PriceLevel {
	level, exists := obs.PriceLevels[price]

	if !exists {
		level = &PriceLevel{Price: uint64(price)}
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
	if obs.Side == pbTypes.Side_BUY {
		// Bid: descending order (100, 99, 98...)
		if newLevel.Price > currentBestPriceLevel.Price {
			newLevel.NextPrice = currentBestPriceLevel
			currentBestPriceLevel.PrevPrice = newLevel
			obs.BestPriceLevel = newLevel

			return
		}

		for currentBestPriceLevel != nil {
			if currentBestPriceLevel.NextPrice == nil || newLevel.Price > currentBestPriceLevel.NextPrice.Price {
				newLevel.PrevPrice = currentBestPriceLevel
				newLevel.NextPrice = currentBestPriceLevel.NextPrice

				if currentBestPriceLevel.NextPrice != nil {
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
			if currentBestPriceLevel.NextPrice == nil || newLevel.Price < currentBestPriceLevel.NextPrice.Price {
				newLevel.PrevPrice = currentBestPriceLevel
				newLevel.NextPrice = currentBestPriceLevel.NextPrice

				if currentBestPriceLevel.NextPrice != nil {
					currentBestPriceLevel.NextPrice.PrevPrice = newLevel
				}
				currentBestPriceLevel.NextPrice = newLevel
				return
			}
			currentBestPriceLevel = currentBestPriceLevel.NextPrice
		}
	}
}

func (obs *OrderBookSide) RemovePriceLevel(level *PriceLevel) {

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

	delete(obs.PriceLevels, level.Price)
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
	OrderSequence uint64
}

func NewMatchingEngine(symbol string) *MatchingEngine {
	return &MatchingEngine{
		Symbol:        symbol,
		Bids:          NewOrderBookSide(pbTypes.Side_BUY),
		Asks:          NewOrderBookSide(pbTypes.Side_SELL),
		AllOrders:     make(map[string]*Order),
		TotalMatches:  0,
		TotalVolume:   0,
		TradeSequence: 0,
		OrderSequence: 0,
	}
}

type AddOrderInternalResponse struct {
	Order  *Order
	Trades []Trade
}

type EngineEventType uint8

const (
	EngineEventType_ORDER_ACCEPTED EngineEventType = iota
	EngineEventType_ORDER_PARTIALLY_FILLED
	EngineEventType_ORDER_FILLED
	EngineEventType_ORDER_CANCELLED
	EngineEventType_ORDER_MODIFY
	EngineEventType_ORDER_REJECTED
	EngineEventType_TRADE_EXECUTED
)

type EngineEvent struct {
	Type uint8
	Data any
}

type OrderAcceptedEvent struct {
	Order *Order
}

type OrderRejectedEvent struct {
	Order *Order
}

type TradeExecutedEvent struct {
	Trade Trade
}

type OrderFilledEvent struct {
	Order  *Order
	Trades []Trade
}

type OrderPartiallyFilledEvent struct {
	Order  *Order
	Trades []Trade
}

type OrderCancelledEvent struct {
	Order  *Order
	Trades []Trade
}

func (me *MatchingEngine) AddOrderInternal(order *Order) (*AddOrderInternalResponse, []EngineEvent, error) {
	if _, exists := me.AllOrders[order.ClientOrderID]; exists {
		return nil, nil, fmt.Errorf("Duplicate Order ID: %s", order.ClientOrderID)
	}

	trades := me.MatchOrder(order)

	if order.Type == pbTypes.OrderType_MARKET {
		// MARKET + no liquidity → already REJECTED inside MatchOrder
		if order.Status == pbTypes.OrderStatus_REJECTED {
			events := me.buildEvents(order, trades)
			return &AddOrderInternalResponse{Order: order, Trades: trades}, events, nil
		}

		// MARKET + partial fill → cancel remainder
		if order.RemainingQuantity > 0 {
			order.Status = pbTypes.OrderStatus_CANCELLED
			order.RemainingQuantity = 0
		}
	}

	if order.RemainingQuantity > 0 && order.Type == pbTypes.OrderType_LIMIT {
		var obs *OrderBookSide

		if order.Side == pbTypes.Side_BUY {
			obs = me.Bids
		} else {
			obs = me.Asks
		}

		level := obs.GetOrCreatePriceLevel(order.Price)

		level.Push(order)
		me.AllOrders[order.ClientOrderID] = order
	}

	events := me.buildEvents(order, trades)
	return &AddOrderInternalResponse{Order: order, Trades: trades}, events, nil
}

func (me *MatchingEngine) MatchOrder(incoming *Order) []Trade {
	var oppositeBook *OrderBookSide

	if incoming.Side == pbTypes.Side_BUY {
		oppositeBook = me.Asks
	} else {
		oppositeBook = me.Bids
	}

	// MARKET + no liquidity → reject immediately
	if incoming.Type == pbTypes.OrderType_MARKET && oppositeBook.IsEmpty() {
		incoming.Status = pbTypes.OrderStatus_REJECTED
		return nil
	}

	if incoming.Type == pbTypes.OrderType_LIMIT && incoming.RemainingQuantity == incoming.Quantity {
		incoming.Status = pbTypes.OrderStatus_OPEN
	}

	trades := []Trade{}

	for incoming.RemainingQuantity > 0 && me.CanMatch(oppositeBook, incoming) {
		bestPriceLevel := oppositeBook.BestPriceLevel
		restingOrder := bestPriceLevel.HeadOrder

		matchQuantity := min(incoming.RemainingQuantity, restingOrder.RemainingQuantity)
		matchPrice := restingOrder.Price

		trade := me.ExecuteTrade(incoming, restingOrder, matchQuantity, matchPrice)

		trades = append(trades, trade)

		incoming.RemainingQuantity -= matchQuantity
		restingOrder.RemainingQuantity -= matchQuantity

		me.TotalMatches++
		me.TotalVolume += uint64(matchQuantity)

		if restingOrder.RemainingQuantity == 0 {
			restingOrder.Status = pbTypes.OrderStatus_FILLED
			bestPriceLevel.Remove(restingOrder)

			delete(me.AllOrders, restingOrder.ClientOrderID)

			if bestPriceLevel.IsEmpty() {

				oppositeBook.RemovePriceLevel(bestPriceLevel)
			}
		}

		if incoming.RemainingQuantity == 0 {
			incoming.Status = pbTypes.OrderStatus_FILLED
			break
		} else {
			incoming.Status = pbTypes.OrderStatus_PARTIAL_FILLED
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

	if incoming.Side == pbTypes.Side_BUY {
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
	me.TradeSequence = +1

	tradeID := me.GenerateTradeID(me.TradeSequence)

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
		TradeSequence: me.TradeSequence,
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

func (me *MatchingEngine) buildEvents(
	order *Order,
	trades []Trade,
) []EngineEvent {

	events := []EngineEvent{}

	// ---------- REJECT ----------
	if order.Status == pbTypes.OrderStatus_REJECTED {
		return []EngineEvent{
			{
				Type: uint8(EngineEventType_ORDER_REJECTED),
				Data: OrderRejectedEvent{Order: order},
			},
		}
	}

	// ---------- ACCEPT ----------
	events = append(events, EngineEvent{
		Type: uint8(EngineEventType_ORDER_ACCEPTED),
		Data: OrderAcceptedEvent{Order: order},
	})

	// ---------- TRADES ----------
	for _, trade := range trades {
		events = append(events, EngineEvent{
			Type: uint8(EngineEventType_TRADE_EXECUTED),
			Data: TradeExecutedEvent{Trade: trade},
		})
	}

	// ---------- FINAL STATE ----------
	switch order.Status {
	case pbTypes.OrderStatus_FILLED:
		events = append(events, EngineEvent{
			Type: uint8(EngineEventType_ORDER_FILLED),
			Data: OrderFilledEvent{Order: order, Trades: trades},
		})

	case pbTypes.OrderStatus_PARTIAL_FILLED:
		events = append(events, EngineEvent{
			Type: uint8(EngineEventType_ORDER_PARTIALLY_FILLED),
			Data: OrderPartiallyFilledEvent{Order: order, Trades: trades},
		})

	case pbTypes.OrderStatus_CANCELLED:
		events = append(events, EngineEvent{
			Type: uint8(EngineEventType_ORDER_CANCELLED),
			Data: OrderCancelledEvent{Order: order, Trades: trades},
		})
	}

	return events
}

/*
=================================================================================
========== Multiple Symbol Matching Engine Management via Actor Model ===========
=================================================================================
*/
type EngineMsg interface{}

type PlaceOrderMsg struct {
	Order *Order
	Reply chan *AddOrderInternalResponse
	Err   chan error
}

type CancelOrderMsg struct {
	OrderID string
	Reply   chan error
}

type SymbolActor struct {
	symbol string
	inbox  chan EngineMsg
	engine *MatchingEngine
}

func NewSymbolActor(symbol string, buffer int) *SymbolActor {
	return &SymbolActor{
		symbol: symbol,
		inbox:  make(chan EngineMsg, buffer),
		engine: NewMatchingEngine(symbol),
	}
}

func (a *SymbolActor) Run() {
	for msg := range a.inbox {
		switch m := msg.(type) {
		case PlaceOrderMsg:
			response, events, err := a.engine.AddOrderInternal(m.Order)
			if err != nil {
				m.Err <- err
				continue
			}
			m.Reply <- response

			a.PublishKafkaEvents(events)

		case CancelOrderMsg:
			// err := a.engine.AddOrder(m.OrderID)
			// m.Reply <- err
			fmt.Println("cancel order")

		default:
			panic("unknown actor message")
		}

	}
}

func (a *SymbolActor) PublishKafkaEvents(events []EngineEvent) {

}
