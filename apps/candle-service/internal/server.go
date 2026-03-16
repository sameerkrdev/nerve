package internal

import (
	"encoding/json"
	"net/http"
)

//* func: define mux server and start consumer and workers
//* func: start the kafka consumer
//* func: start the workers
//*	 - each worker recieve gets single symbol trade data via channel
//	 - calculate the candlestick data for multiple timeframe
//	 - L1: In-memory (last 500 candles)
//	 - L2: Redis Memory (last 5000 candles)
//	 - L3: store the trades into clickhouse which will eventually generate the candles data
//	 - publish to kafka or redis pub/sub for indicator service
// func: to get the historical data of candles
// func: graceful shutdown

func NewServer() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", HealthCheck)
	mux.HandleFunc("GET /candles", GetCandles)

	return mux
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	res := APIReponse{
		Success: true,
		Data: map[string]string{
			"message": "Yooo!, I am healthy",
		},
		Error: "",
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func GetCandles(w http.ResponseWriter, r *http.Request) {

}
