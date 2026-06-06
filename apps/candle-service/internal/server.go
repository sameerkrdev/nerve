package internal

import (
	"context"
	"log"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/clickhouse"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/engine"
	memorystore "github.com/sameerkrdev/nerve/apps/candle-service/internal/memoryStore"
	pbAggeration "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
)

type Server struct {
	router          *engine.WorkerRouter
	clickhouseConn  driver.Conn
	pbAggeration.UnimplementedCandleServiceServer
}

func NewGrpcServer(router *engine.WorkerRouter, clickhouseConn driver.Conn, netListener net.Listener) *grpc.Server {
	s := &Server{
		router:         router,
		clickhouseConn: clickhouseConn,
	}

	srv := grpc.NewServer()
	pbAggeration.RegisterCandleServiceServer(srv, s)
	reflection.Register(srv)

	go func() {
		if err := srv.Serve(netListener); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	return srv
}

func (s *Server) GetCandles(ctx context.Context, req *pbAggeration.GetCandlesRequest) (*pbAggeration.GetCandlesResponse, error) {
	tf := req.GetTimeframe()
	symbol := req.GetSymbol()

	if symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}
	if tf == pbAggeration.Timeframe_TIMEFRAME_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "invalid timeframe")
	}

	count := req.GetNumberOfCandlesticks()
	if count <= 0 {
		count = 500
	}

	tfName, ok := pbAggeration.Timeframe_name[int32(tf)]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid timeframe")
	}

	// L1: in-memory
	candles, err := s.router.GetCandles(symbol, tf)
	if err != nil {
		return nil, status.Error(codes.Internal, "something went wrong. try again later")
	}

	// L2: Redis
	if len(candles) == 0 {
		candles, err = memorystore.GetCandlesFromRedis(symbol, tfName, count)
		if err != nil {
			slog.Warn("redis L2 miss", "symbol", symbol, "timeframe", tfName, "error", err)
			candles = nil
		}
	}

	// L3: ClickHouse — aggregate OHLCV from trades table on-the-fly
	if len(candles) == 0 && s.clickhouseConn != nil {
		candles, err = clickhouse.FetchCandles(ctx, s.clickhouseConn, symbol, int32(tf), count)
		if err != nil {
			slog.Error("clickhouse L3 fetch failed", "error", err)
			return nil, status.Error(codes.Internal, "something went wrong. try again later")
		}
	}

	candleBatch := &pbAggeration.CandleBatch{
		Symbol:    symbol,
		Timeframe: tf,
		Candles:   candles,
	}
	return &pbAggeration.GetCandlesResponse{
		Result: &pbAggeration.GetCandlesResponse_Data{Data: candleBatch},
	}, nil
}
