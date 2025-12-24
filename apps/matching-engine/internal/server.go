package internal

import (
	"context"
	// "log/slog"
	"math/rand/v2"
	"strconv"
	"time"

	pbTypes "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedMatchingEngineServer
}

func (s *Server) PlaceOrder(ctx context.Context, in *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	// slog.Info("Place Order:", in)

	auctionNumber := int(rand.Float64() * 1_000_000) // TODO: Use a per-symbol monotonic sequence (not random) so replay can rebuild the book.

	return &pb.PlaceOrderResponse{
		ClientOrderId:     in.ClientOrderId,
		Symbol:            in.Symbol,
		Status:            pbTypes.OrderStatus_OPEN,
		Price:             in.Price,
		Quantity:          in.Quantity,
		RemainingQuantity: in.Quantity,
		Side:              in.Side,
		Type:              in.Type,
		UserId:            in.UserId,

		AuctionNumber:    strconv.Itoa(auctionNumber),
		ClientTimestamp:  in.ClientTimestamp,
		GatewayTimestamp: in.GatewayTimestamp,
		EngineTimestamp:  timestamppb.New(time.Now()),
	}, nil
}
