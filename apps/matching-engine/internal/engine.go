package internal

import (
	"fmt"
	"time"

	pbTypes "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Order struct {
	Symbol        string
	Price         int64
	AveragePrice  int64
	ExecutedValue int64

	Quantity          int64
	FilledQuantity    int64
	RemainingQuantity int64
	CancelledQuantity int64
	Side              pbTypes.Side
	Type              pbTypes.OrderType
	UserID            string
	ClientOrderID     string
	Status            pbTypes.OrderStatus
	StatusMessage     string
	ClientTimestamp   *timestamppb.Timestamp
	GatewayTimestamp  *timestamppb.Timestamp
	EngineTimestamp   *timestamppb.Timestamp

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
	Price       int64
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

	pl.TotalVolume -= uint64(order.RemainingQuantity)
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
	PriceLevels map[int64]*PriceLevel

	BestPriceLevel *PriceLevel
}

func NewOrderBookSide(side pbTypes.Side) *OrderBookSide {
	return &OrderBookSide{
		Side:        side,
		PriceLevels: make(map[int64]*PriceLevel),
	}
}

func (obs *OrderBookSide) IsEmpty() bool {
	return obs.BestPriceLevel == nil
}

func (obs *OrderBookSide) GetOrCreatePriceLevel(price int64) *PriceLevel {
	level, exists := obs.PriceLevels[price]

	if !exists {
		level = &PriceLevel{Price: int64(price)}
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

	wal *SymbolWAL
}

func NewMatchingEngine(symbol string, wal *SymbolWAL) *MatchingEngine {
	return &MatchingEngine{
		Symbol:        symbol,
		Bids:          NewOrderBookSide(pbTypes.Side_BUY),
		Asks:          NewOrderBookSide(pbTypes.Side_SELL),
		AllOrders:     make(map[string]*Order),
		TotalMatches:  0,
		TotalVolume:   0,
		TradeSequence: 0,
		OrderSequence: 0,
		wal:           wal,
	}
}

type AddOrderInternalResponse struct {
	Order  *Order
	Trades []Trade
}

func (me *MatchingEngine) AddOrderInternal(order *Order) (*AddOrderInternalResponse, []*pb.EngineEvent, error) {
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
		if order.FilledQuantity > 0 {
			order.Status = pbTypes.OrderStatus_CANCELLED

			order.StatusMessage = "Market order partially filled; remaining quantity cancelled"
			order.CancelledQuantity = order.RemainingQuantity
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
		incoming.StatusMessage = "Market order rejected: no liquidity on opposite side"
		return nil
	}

	if incoming.Type == pbTypes.OrderType_LIMIT && incoming.RemainingQuantity == incoming.Quantity {
		incoming.Status = pbTypes.OrderStatus_OPEN
	}

	trades := []Trade{}

	for incoming.RemainingQuantity > 0 && me.CanMatch(oppositeBook, incoming) {
		bestPriceLevel := oppositeBook.BestPriceLevel
		restingOrder := bestPriceLevel.HeadOrder

		// Self-trade prevention
		if incoming.UserID == restingOrder.UserID {
			break // skip resting order
		}

		matchQuantity := min(incoming.RemainingQuantity, restingOrder.RemainingQuantity)
		matchPrice := restingOrder.Price

		trade := me.ExecuteTrade(incoming, restingOrder, matchQuantity, matchPrice)

		trades = append(trades, trade)

		incoming.RemainingQuantity -= matchQuantity
		restingOrder.RemainingQuantity -= matchQuantity

		incoming.FilledQuantity += matchQuantity
		restingOrder.FilledQuantity += matchQuantity

		incoming.ExecutedValue += int64(matchPrice) * int64(matchQuantity)
		restingOrder.ExecutedValue += int64(matchPrice) * int64(matchQuantity)

		incoming.AveragePrice = incoming.ExecutedValue / int64(incoming.FilledQuantity)
		restingOrder.AveragePrice = restingOrder.ExecutedValue / int64(restingOrder.FilledQuantity)

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

	}

	if incoming.RemainingQuantity == 0 {
		incoming.Status = pbTypes.OrderStatus_FILLED
	} else {
		incoming.Status = pbTypes.OrderStatus_PARTIAL_FILLED
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
	Price         int64
	Quantity      int64
	Timeline      *timestamppb.Timestamp

	BuyerID     string
	SellerID    string
	BuyOrderID  string
	SellOrderID string

	IsBuyerMaker bool
}

func (me *MatchingEngine) ExecuteTrade(aggressor *Order, restingOrder *Order, matchQuantity int64, matchPrice int64) Trade {
	me.TradeSequence++

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
		Timeline:      timestamppb.Now(),

		BuyOrderID:  BuyOrderID,
		BuyerID:     BuyerID,
		SellOrderID: SellOrderID,
		SellerID:    SellerID,

		IsBuyerMaker: restingOrder.Side == pbTypes.Side_BUY,
	}
}

// generateTradeID creates a unique trade ID
func (me *MatchingEngine) GenerateTradeID(seq uint64) string {
	return fmt.Sprintf("%s-T%d-%d", me.Symbol, time.Now().UnixNano(), seq)
}

func (me *MatchingEngine) buildEvents(
	order *Order,
	trades []Trade,
) []*pb.EngineEvent {

	events := []*pb.EngineEvent{}
	data, _ := EncodeOrderStatusEvent(order, StrPtr(""))

	// ---------- REJECT ----------
	if order.Status == pbTypes.OrderStatus_REJECTED {

		return []*pb.EngineEvent{
			{
				EventType: pbTypes.EventType_ORDER_REJECTED,
				Data:      data,
			},
		}
	}

	// ---------- ACCEPT ----------
	events = append(events, &pb.EngineEvent{
		EventType: pbTypes.EventType_ORDER_ACCEPTED,
		Data:      data,
	})

	// ---------- TRADES ----------
	for _, trade := range trades {
		tradeData, _ := EncodeTradeEvent(&trade)
		events = append(events, &pb.EngineEvent{
			EventType: pbTypes.EventType_TRADE_EXECUTED,
			Data:      tradeData,
		})
	}

	// ---------- FINAL STATE ----------
	switch order.Status {
	case pbTypes.OrderStatus_FILLED:
		events = append(events, &pb.EngineEvent{
			EventType: pbTypes.EventType_ORDER_FILLED,
			Data:      data,
		})

	case pbTypes.OrderStatus_CANCELLED:
		events = append(events, &pb.EngineEvent{
			EventType: pbTypes.EventType_ORDER_CANCELLED,
			Data:      data,
		})
	}

	return events
}

type CancelOrderInternalResponse struct {
	ID            string
	Status        string
	StatusMessage string
}

func (me *MatchingEngine) CancelOrderInternal(
	id string,
	userID string,
	symbol string,
) (*CancelOrderInternalResponse, []*pb.EngineEvent, error) {

	events := []*pb.EngineEvent{}

	order, ok := me.AllOrders[id]
	if !ok {
		return nil, nil, fmt.Errorf("order not found")
	}

	// ownership check
	if order.UserID != userID {
		return nil, nil, fmt.Errorf("unauthorized cancel")
	}

	// already filled or already cancelled
	if order.RemainingQuantity == 0 {
		return nil, nil, fmt.Errorf("order already completed")
	}

	// determine book side
	obs := me.Asks
	if order.Side == pbTypes.Side_BUY {
		obs = me.Bids
	}

	level, ok := obs.PriceLevels[order.Price]
	if !ok {
		return nil, nil, fmt.Errorf("price level not found")
	}

	// cancel remaining quantity
	order.CancelledQuantity = order.RemainingQuantity
	order.RemainingQuantity = 0
	order.Status = pbTypes.OrderStatus_CANCELLED

	// remove from book
	level.Remove(order)
	delete(me.AllOrders, order.ClientOrderID)

	if level.IsEmpty() {
		obs.RemovePriceLevel(level)
	}

	data, _ := EncodeOrderStatusEvent(order, StrPtr(""))
	events = append(events, &pb.EngineEvent{
		EventType: pbTypes.EventType_ORDER_CANCELLED,
		Data:      data,
	})

	return &CancelOrderInternalResponse{
		ID:     order.ClientOrderID,
		Status: "ORDER_CANCELLED",
	}, events, nil
}

type ModifyOrderInternalResponse struct {
	OrderID       string
	OldOrderId    string
	NewOrderId    string
	Status        string
	StatusMessage string
}

func (me *MatchingEngine) ModifyOrderInternal(
	symbol string,
	oldOrderID string,
	userID string,
	newOrderID string, // required ONLY for replace
	newPrice *int64,
	newQuantity *int64,
) (*ModifyOrderInternalResponse, []*pb.EngineEvent, error) {

	order, ok := me.AllOrders[oldOrderID]
	if !ok {
		return nil, nil, fmt.Errorf("order not found")
	}
	if order.UserID != userID {
		return nil, nil, fmt.Errorf("unauthorized")
	}
	if order.Symbol != symbol {
		return nil, nil, fmt.Errorf("symbol mismatch")
	}
	if order.Status == pbTypes.OrderStatus_FILLED || order.Status == pbTypes.OrderStatus_CANCELLED {
		return nil, nil, fmt.Errorf("order not modifiable")
	}

	executed := order.Quantity - order.RemainingQuantity

	if newQuantity != nil && *newQuantity < executed {
		return nil, nil, fmt.Errorf("new quantity < executed quantity")
	}

	newRemaining := order.RemainingQuantity
	if newQuantity != nil {
		newRemaining = *newQuantity - executed
	}

	priceChanged := newPrice != nil && *newPrice != order.Price
	qtyReduced := newRemaining < order.RemainingQuantity
	qtyIncreased := newRemaining > order.RemainingQuantity

	switch {
	case priceChanged || qtyIncreased:
		if _, exists := me.AllOrders[newOrderID]; exists {
			return nil, nil, fmt.Errorf("new_order_id already exists")
		}
		events, err := me.replaceOrder(order, newOrderID, newPrice, newQuantity)
		if err != nil {
			return nil, nil, err
		}

		response := &ModifyOrderInternalResponse{OrderID: order.ClientOrderID, OldOrderId: order.ClientOrderID, NewOrderId: newOrderID, Status: "Success"}
		return response, events, nil

	case qtyReduced:

		events, err := me.reduceOrder(order, newQuantity)
		if err != nil {
			return nil, nil, err
		}

		response := &ModifyOrderInternalResponse{OrderID: order.ClientOrderID, OldOrderId: "", NewOrderId: "", Status: "Success"}
		return response, events, nil

	default:
		return nil, nil, nil
	}
}

func (me *MatchingEngine) reduceOrder(
	order *Order,
	newQuantity *int64,
) ([]*pb.EngineEvent, error) {

	events := []*pb.EngineEvent{}

	if newQuantity == nil {
		return nil, fmt.Errorf("quantity required")
	}

	oldQuantity := order.Quantity
	oldRemaining := order.RemainingQuantity

	executed := oldQuantity - oldRemaining
	newRemaining := *newQuantity - executed

	if newRemaining < 0 {
		return nil, fmt.Errorf("invalid reduce")
	}

	// apply mutation
	order.Quantity = *newQuantity
	order.RemainingQuantity = newRemaining

	// update price level volume
	if order.PriceLevel != nil {
		volumeDelta := oldRemaining - newRemaining
		order.PriceLevel.TotalVolume -= uint64(volumeDelta)
	}

	// emit correct event
	if order.RemainingQuantity == 0 {
		event, err := EncodeOrderStatusEvent(order, StrPtr("remaining quantity become 0"))
		if err != nil {
			return nil, err
		}

		events = append(events, &pb.EngineEvent{
			EventType: pbTypes.EventType_ORDER_CANCELLED,
			Data:      event,
		})
	} else {
		event, err := EncodeOrderReducedEvent(order, oldQuantity, oldRemaining)
		if err != nil {
			return nil, err
		}
		events = append(events, &pb.EngineEvent{
			EventType: pbTypes.EventType_ORDER_REDUCED,
			Data:      event,
		})
	}

	return events, nil
}

func (me *MatchingEngine) replaceOrder(
	order *Order,
	newOrderID string,
	newPrice *int64,
	newQuantity *int64,
) ([]*pb.EngineEvent, error) {

	var events []*pb.EngineEvent
	executed := order.Quantity - order.RemainingQuantity

	// ---------- cancel old ----------
	_, cancelEvents, err := me.CancelOrderInternal(
		order.ClientOrderID,
		order.UserID,
		order.Symbol,
	)
	if err != nil {
		return nil, err
	}
	events = append(events, cancelEvents...)

	price := order.Price
	if newPrice != nil {
		price = *newPrice
	}

	totalQty := order.Quantity
	if newQuantity != nil {
		totalQty = *newQuantity
	}

	newRemaining := totalQty - executed

	// ---------- create new ----------
	newOrder := &Order{
		Symbol:            order.Symbol,
		Price:             price,
		Quantity:          totalQty,
		RemainingQuantity: newRemaining,
		Side:              order.Side,
		Type:              order.Type,
		ClientOrderID:     newOrderID,
		UserID:            order.UserID,
		EngineTimestamp:   timestamppb.New(time.Now()), // priority reset
	}

	_, addEvents, err := me.AddOrderInternal(newOrder)
	if err != nil {
		return nil, err
	}
	events = append(events, addEvents...)

	return events, nil
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
	ID     string
	UserID string
	Symbol string
	Reply  chan *CancelOrderInternalResponse
	Err    chan error
}

type ModifyOrderMsg struct {
	OrderID        string
	UserID         string
	ClientModifyID string
	Symbol         string
	NewPrice       *int64
	NewQuantity    *int64
	Reply          chan *ModifyOrderInternalResponse
	Err            chan error
}

type SymbolActor struct {
	symbol string
	inbox  chan EngineMsg
	engine *MatchingEngine

	wal          *SymbolWAL
	kafkaEmitter *KafkaProducerWorker
	grpcStreams  []pb.MatchingEngine_SubscribeSymbolServer
}

func NewSymbolActor(symbol Symbol, buffer int) (*SymbolActor, error) {
	wal, err := OpenWAL(symbol.WalDir, symbol.Name, int64(symbol.MaxWalFileSize), symbol.WalShouldFsync, symbol.WalSyncInterval)
	if err != nil {
		return nil, err
	}

	kakfaWoker, err := NewKafkaProducerWorker(
		symbol.Name,
		symbol.WalDir,
		wal,
		symbol.KafkaBatchSize,
		symbol.KafkaEmitMM,
	)
	if err != nil {
		return nil, err
	}

	return &SymbolActor{
		symbol:       symbol.Name,
		inbox:        make(chan EngineMsg, buffer),
		engine:       NewMatchingEngine(symbol.Name, wal),
		wal:          wal,
		kafkaEmitter: kakfaWoker,
		grpcStreams:  make([]pb.MatchingEngine_SubscribeSymbolServer, 0),
	}, nil
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

			for _, event := range events {
				data, err := proto.Marshal(event)
				if err != nil {
					m.Err <- err
					continue
				}
				for _, stream := range a.grpcStreams {
					stream.Send(event)
				}

				if err := a.wal.writeEntry(data); err != nil {
					m.Err <- err
					continue
				}
			}

		case CancelOrderMsg:
			response, events, err := a.engine.CancelOrderInternal(m.ID, m.UserID, m.Symbol)

			if err != nil {
				m.Err <- err
				continue
			}
			m.Reply <- response

			for _, event := range events {
				data, err := proto.Marshal(event)
				if err != nil {
					m.Err <- err
					continue
				}
				if err := a.wal.writeEntry(data); err != nil {
					m.Err <- err
					continue
				}
			}

		case ModifyOrderMsg:
			response, events, err := a.engine.ModifyOrderInternal(m.Symbol, m.OrderID, m.UserID, m.ClientModifyID, m.NewPrice, m.NewQuantity)

			if err != nil {
				m.Err <- err
				continue
			}
			m.Reply <- response

			for _, event := range events {
				data, err := proto.Marshal(event)
				if err != nil {
					m.Err <- err
					continue
				}
				if err := a.wal.writeEntry(data); err != nil {
					m.Err <- err
					continue
				}

			}

		default:
			panic("unknown actor message")
		}

	}
}
