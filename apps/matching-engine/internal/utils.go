package internal

import (
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

func StrPtr(s string) *string {
	return &s
}

func EncodeOrderStatusEvent(order *Order, statusMessage *string, isAcceptEvent bool) ([]byte, error) {
	data := &pb.OrderStatusEvent{
		OrderId:       order.ClientOrderID,
		UserId:        order.UserID,
		Symbol:        order.Symbol,
		Status:        order.Status,
		StatusMessage: statusMessage,
		Side:          order.Side,
		Type:          order.Type,

		Price:         order.Price,
		ExecutedValue: order.ExecutedValue,
		AveragePrice:  order.AveragePrice,

		Quantity:          order.Quantity,
		FilledQuantity:    order.FilledQuantity,
		RemainingQuantity: order.RemainingQuantity,
		CancelledQuantity: order.CancelledQuantity,

		GatewayTimestamp: order.GatewayTimestamp,
		ClientTimestamp:  order.ClientTimestamp,
		EngineTimestamp:  order.EngineTimestamp,
	}

	if data.StatusMessage == nil || *data.StatusMessage == "" {
		data.StatusMessage = &order.StatusMessage
	}

	if isAcceptEvent {
		data.FilledQuantity = 0
		data.CancelledQuantity = 0
		data.RemainingQuantity = order.Quantity
		data.ExecutedValue = 0
		data.AveragePrice = 0
	}

	eventByte, err := proto.Marshal(data)
	if err != nil {
		return nil, err
	}

	return eventByte, nil
}

func EncodeOrderReducedEvent(order *Order, oldQuantity int64, oldRemainingQuantiy int64, newCancelledQuantity int64, oldCancelledQuantity int64) ([]byte, error) {
	eventByte, err := proto.Marshal(&pb.OrderReducedEvent{
		Order: &pb.OrderStatusEvent{
			OrderId: order.ClientOrderID,
			UserId:  order.UserID,
			Symbol:  order.Symbol,
			Status:  order.Status,
			Side:    order.Side,
			Type:    order.Type,

			Price:         order.Price,
			ExecutedValue: order.ExecutedValue,
			AveragePrice:  order.AveragePrice,

			Quantity:          order.Quantity,
			FilledQuantity:    order.FilledQuantity,
			RemainingQuantity: order.RemainingQuantity,
			CancelledQuantity: order.CancelledQuantity,

			GatewayTimestamp: order.GatewayTimestamp,
			ClientTimestamp:  order.ClientTimestamp,
			EngineTimestamp:  order.EngineTimestamp,
		},
		OldQuantity:          oldQuantity,
		NewQuantity:          order.Quantity,
		OldRemainingQuantity: oldRemainingQuantiy,
		NewRemainingQuantity: order.RemainingQuantity,
		OldCancelledQuantity: oldCancelledQuantity,
		NewCancelledQuantity: newCancelledQuantity,
	})
	if err != nil {
		return nil, err
	}

	return eventByte, nil
}

func EncodeTradeEvent(trade *Trade) ([]byte, error) {
	eventByte, err := proto.Marshal(&pb.TradeEvent{
		TradeId:       trade.TradeID,
		Symbol:        trade.Symbol,
		TradeSequence: trade.TradeSequence,
		Price:         trade.Price,
		Quantity:      trade.Quantity,
		BuyerId:       trade.BuyerID,
		SellerId:      trade.SellerID,
		BuyOrderId:    trade.BuyOrderID,
		SellOrderId:   trade.SellOrderID,
		IsBuyerMaker:  trade.IsBuyerMaker,
		Timestamp:     trade.Timeline,
	})
	if err != nil {
		return nil, err
	}

	return eventByte, nil
}
