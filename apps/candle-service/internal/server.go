package internal

import (
	"encoding/json"
	"net/http"

	"github.com/sameerkrdev/nerve/apps/candle-service/internal/engine"
	"github.com/sameerkrdev/nerve/apps/candle-service/internal/utils"
	pbAggeration "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
)

type Server struct {
	router *engine.WorkerRouter
}

func NewServer(router *engine.WorkerRouter) *http.ServeMux {
	s := &Server{
		router,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", HealthCheck)
	mux.HandleFunc("GET /candles", s.GetCandles)

	return mux
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	resp := utils.APIReponse{
		Success: true,
		Data: map[string]string{
			"message": "Yooo!, I am healthy",
		},
		Error: "",
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) GetCandles(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	interval := r.URL.Query().Get("timeframe")

	if symbol == "" || interval == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(utils.APIReponse{Success: false, Error: "symbol and timeframe are required"})
		return
	}

	timeframeEnumVal, ok := pbAggeration.Timeframe_value[interval]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(utils.APIReponse{Success: false, Error: "invalid timeframe"})
		return
	}

	candles, err := s.router.GetCandles(symbol, pbAggeration.Timeframe(timeframeEnumVal))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode((utils.APIReponse{Success: false, Error: "something went wrong"}))
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(utils.APIReponse{Success: true, Data: candles})
}
