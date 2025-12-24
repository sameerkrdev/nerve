package internal

import (
	"context"
	"log/slog"
	"time"

	pbTypes "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedMatchingEngineServer
}

func (s *Server) PlaceOrder(ctx context.Context, in *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	slog.Info("Place Order:", in)

	return &pb.PlaceOrderResponse{
		Id:              in.Id,
		Symbol:          in.Symbol,
		Status:          pbTypes.OrderStatus_PENDING,
		Reason:          "Order received",
		EngineTimestamp: timestamppb.New(time.Now()),
	}, nil
}
