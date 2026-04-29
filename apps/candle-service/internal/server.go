package internal

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/sameerkrdev/nerve/apps/candle-service/internal/engine"
	pbAggeration "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
)

type Server struct {
	router *engine.WorkerRouter
	pbAggeration.UnimplementedCandleServiceServer
}

func NewGrpcServer(router *engine.WorkerRouter, netListener net.Listener) *grpc.Server {
	s := &Server{
		router: router,
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

// TODO: Implement the from, to feature :- inmemory, redis, clickhouse
func (s *Server) GetCandles(ctx context.Context, req *pbAggeration.GetCandlesRequest) (*pbAggeration.GetCandlesResponse, error) {
	tf := req.GetTimeframe()
	symbol := req.GetSymbol()

	if symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if tf == pbAggeration.Timeframe_TIMEFRAME_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "invalid timeframe")
	}

	candles, err := s.router.GetCandles(symbol, tf)
	if err != nil {
		return nil, status.Error(codes.Internal, "something went wrong. try again later")
	}

	candleBatch := &pbAggeration.CandleBatch{
		Symbol:    symbol,
		Timeframe: tf,
		Candles:   candles,
	}
	response := &pbAggeration.GetCandlesResponse{
		Result: &pbAggeration.GetCandlesResponse_Data{
			Data: candleBatch,
		},
	}

	return response, nil
}

// func HealthCheck(w http.ResponseWriter, r *http.Request) {
// 	resp := utils.APIReponse{
// 		Success: true,
// 		Data: map[string]string{
// 			"message": "Yooo!, I am healthy",
// 		},
// 		Error: "",
// 	}

// 	w.WriteHeader(http.StatusOK)
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(resp)
// }

// func (s *Server) GetCandles(w http.ResponseWriter, r *http.Request) {
// 	symbol := r.URL.Query().Get("symbol")
// 	interval := r.URL.Query().Get("timeframe")

// 	if symbol == "" || interval == "" {
// 		w.WriteHeader(http.StatusBadRequest)
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(utils.APIReponse{Success: false, Error: "symbol and timeframe are required"})
// 		return
// 	}

// 	timeframeEnumVal, ok := pbAggeration.Timeframe_value[interval]
// 	if !ok {
// 		w.WriteHeader(http.StatusBadRequest)
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(utils.APIReponse{Success: false, Error: "invalid timeframe"})
// 		return
// 	}

// 	candles, err := s.router.GetCandles(symbol, pbAggeration.Timeframe(timeframeEnumVal))
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode((utils.APIReponse{Success: false, Error: "something went wrong"}))
// 	}

// 	w.WriteHeader(http.StatusOK)
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(utils.APIReponse{Success: true, Data: candles})
// }
