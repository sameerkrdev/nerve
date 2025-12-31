package internal

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	// "log/slog"

	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedMatchingEngineServer
}

func (s *Server) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	// slog.Info("Place Order:", in)

	order := &Order{
		Symbol:            req.Symbol,
		Price:             req.Price,
		Quantity:          req.Quantity,
		RemainingQuantity: req.Quantity,
		Side:              req.Side,
		Type:              req.Type,
		ClientOrderID:     req.ClientOrderId,
		UserID:            req.UserId,
		GatewayTimestamp:  req.GatewayTimestamp,
		ClientTimestamp:   req.ClientTimestamp,
		EngineTimestamp:   timestamppb.New(time.Now()),
	}

	res, err := PlaceOrder(order)

	if err != nil {
		slog.Error("Failed to process order",
			"Error", err,
			"orderId", req.ClientOrderId,
		)
		return nil, err
	}

	fmt.Println(res.Order, res.Trades)

	return &pb.PlaceOrderResponse{
		ClientOrderId:     res.Order.ClientOrderID,
		Symbol:            res.Order.Symbol,
		Status:            res.Order.Status,
		Price:             res.Order.Price,
		AveragePrice:      res.Order.AveragePrice,
		Quantity:          res.Order.Quantity,
		RemainingQuantity: res.Order.RemainingQuantity,
		FilledQuantity:    res.Order.FilledQuantity,
		CancelledQuantity: res.Order.CancelledQuantity,
		ExecutedValue:     res.Order.ExecutedValue,
		Side:              res.Order.Side,
		Type:              res.Order.Type,
		UserId:            res.Order.UserID,

		AuctionNumber:    strconv.FormatUint(uint64(res.Order.OrderSequence), 10),
		ClientTimestamp:  res.Order.ClientTimestamp,
		GatewayTimestamp: res.Order.GatewayTimestamp,
	}, nil
}

func (s *Server) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {

	res, err := CancelOrder(req.Id, req.UserId, req.Symbol)

	if err != nil {
		slog.Error("Failed to cancel order")
		return nil, err
	}

	return &pb.CancelOrderResponse{
		Id:            res.ID,
		Status:        res.Status,
		StatusMessage: &res.StatusMessage,
	}, nil
}
