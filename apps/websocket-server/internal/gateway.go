package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	pbAgg "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/proto"
)

// ─── Event ───────────────────────────────────────────────────────────────────

type Event struct {
	EventType pbType.EventType
	Data      []byte
}

// ─── WSGateway ────────────────────────────────────────────────────────────────

type WSGateway struct {
	// engine gRPC streams (one per symbol)
	symbolStreams  map[string]pb.MatchingEngine_SubscribeSymbolClient
	streamsMu      sync.RWMutex
	engineConnection *grpc.ClientConn

	// engine event subscribers (symbol → users)
	Users     map[string]map[*User]bool
	UsersByID map[string]*User
	mu        sync.RWMutex

	// candle Redis pub/sub streams (candleKey → PubSub)
	redisClient    *redis.Client
	candleStreams   map[string]*redis.PubSub
	candleStreamsMu sync.RWMutex

	// candle subscribers (candleKey → users)
	candleUsers   map[string]map[*User]bool
	candleUsersMu sync.RWMutex

	upgrader websocket.Upgrader
}

// ─── User ─────────────────────────────────────────────────────────────────────

type User struct {
	ID                  string
	Conn                *websocket.Conn
	Channel             chan *Event
	Subscriptions       map[string]bool // symbol → true (engine events)
	CandleSubscriptions map[string]bool // candleKey → true
}

// ─── Constructor ──────────────────────────────────────────────────────────────

func NewWSGateway(redisClient *redis.Client) *WSGateway {
	return &WSGateway{
		symbolStreams:  make(map[string]pb.MatchingEngine_SubscribeSymbolClient),
		Users:          make(map[string]map[*User]bool),
		UsersByID:      make(map[string]*User),
		redisClient:    redisClient,
		candleStreams:  make(map[string]*redis.PubSub),
		candleUsers:   make(map[string]map[*User]bool),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
			return true
		}},
	}
}

// ─── Engine gRPC ──────────────────────────────────────────────────────────────

func (wsg *WSGateway) ConnectToEngine(uri string) error {
	opt := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	conn, err := grpc.NewClient(uri, opt...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	wsg.engineConnection = conn
	return nil
}

func (wsg *WSGateway) subscribeToSymbol(symbol string) error {
	wsg.streamsMu.Lock()
	defer wsg.streamsMu.Unlock()

	if _, exist := wsg.symbolStreams[symbol]; exist {
		return nil
	}

	client := pb.NewMatchingEngineClient(wsg.engineConnection)
	req := &pb.SubscribeRequest{
		Symbol:    symbol,
		GatewayId: "123",
		Timestamp: time.Now().UnixNano(),
	}

	stream, err := client.SubscribeSymbol(context.Background(), req)
	if err != nil {
		return err
	}

	wsg.symbolStreams[symbol] = stream
	log.Printf("subscribed to engine stream for %s", symbol)
	go wsg.receiveFromSymbolStream(symbol, stream)
	return nil
}

func (wsg *WSGateway) receiveFromSymbolStream(symbol string, stream grpc.ServerStreamingClient[pb.EngineEvent]) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Printf("%s engine stream error: %v", symbol, err)

			wsg.streamsMu.Lock()
			delete(wsg.symbolStreams, symbol)
			wsg.streamsMu.Unlock()

			time.Sleep(time.Second)
			wsg.subscribeToSymbol(symbol)
			return
		}

		wsg.broadcastToUsers(symbol, msg)
	}
}

func (wsg *WSGateway) broadcastToUsers(symbol string, msg *pb.EngineEvent) {
	switch msg.EventType {
	case
		pbType.EventType_ORDER_ACCEPTED,
		pbType.EventType_ORDER_CANCELLED,
		pbType.EventType_ORDER_FILLED,
		pbType.EventType_ORDER_REJECTED,
		pbType.EventType_ORDER_REDUCED:

		if user, exist := wsg.UsersByID[msg.UserId]; exist {
			user.Channel <- &Event{EventType: msg.EventType, Data: msg.Data}
		}

	case pbType.EventType_TRADE_EXECUTED:
		var data pb.TradeEvent
		if err := proto.Unmarshal(msg.Data, &data); err != nil {
			log.Printf("failed to unmarshal trade event for symbol %s", symbol)
			return
		}

		event := &Event{EventType: msg.EventType, Data: msg.Data}

		if user, exist := wsg.UsersByID[data.BuyerId]; exist {
			select {
			case user.Channel <- event:
			default:
				log.Printf("user %s channel full", user.ID)
			}
		}

		if user, exist := wsg.UsersByID[data.SellerId]; exist {
			select {
			case user.Channel <- event:
			default:
				log.Printf("user %s channel full", user.ID)
			}
		}

	case pbType.EventType_DEPTH, pbType.EventType_TICKER:
		wsg.mu.RLock()
		users, exists := wsg.Users[symbol]
		if !exists {
			wsg.mu.RUnlock()
			return
		}
		snapshot := make([]*User, 0, len(users))
		for u := range users {
			snapshot = append(snapshot, u)
		}
		wsg.mu.RUnlock()

		event := &Event{EventType: msg.EventType, Data: msg.Data}
		for _, u := range snapshot {
			select {
			case u.Channel <- event:
			default:
				log.Printf("user %s channel full", u.ID)
			}
		}
	}
}

func (wsg *WSGateway) subscribe(user *User, symbol string) {
	wsg.subscribeToSymbol(symbol)

	wsg.mu.Lock()
	defer wsg.mu.Unlock()

	if wsg.Users[symbol] == nil {
		wsg.Users[symbol] = make(map[*User]bool)
	}

	wsg.Users[symbol][user] = true
	wsg.UsersByID[user.ID] = user
	user.Subscriptions[symbol] = true

	log.Printf("user %s subscribed to engine stream %s", user.ID, symbol)
}

func (wsg *WSGateway) unsubscribe(user *User, symbol string) {
	wsg.mu.Lock()
	defer wsg.mu.Unlock()

	if users, exists := wsg.Users[symbol]; exists {
		delete(users, user)
		delete(wsg.UsersByID, user.ID)
		delete(user.Subscriptions, symbol)

		if len(users) == 0 {
			delete(wsg.Users, symbol)

			wsg.streamsMu.Lock()
			if _, exists := wsg.symbolStreams[symbol]; exists {
				delete(wsg.symbolStreams, symbol)
				log.Printf("closed engine stream for %s (no subscribers)", symbol)
			}
			wsg.streamsMu.Unlock()
		}
	}
}

// ─── Candle Redis pub/sub ─────────────────────────────────────────────────────

func (wsg *WSGateway) subscribeToCandle(user *User, symbol, timeframe string) {
	key := candleKey(symbol, timeframe)

	// idempotency — user already on this channel
	if user != nil && user.CandleSubscriptions[key] {
		return
	}

	wsg.candleStreamsMu.Lock()

	if _, exists := wsg.candleStreams[key]; exists {
		// goroutine already running — just add user
		wsg.candleUsersMu.Lock()
		if user != nil {
			wsg.candleUsers[key][user] = true
		}
		wsg.candleUsersMu.Unlock()
		wsg.candleStreamsMu.Unlock()

		if user != nil {
			user.CandleSubscriptions[key] = true
		}
		log.Printf("user %s joined existing candle stream %s", user.ID, key)
		return
	}

	// first subscriber — create PubSub + start goroutine
	pubsub := wsg.redisClient.Subscribe(context.Background(), key)
	wsg.candleStreams[key] = pubsub

	wsg.candleUsersMu.Lock()
	wsg.candleUsers[key] = make(map[*User]bool)
	if user != nil {
		wsg.candleUsers[key][user] = true
	}
	wsg.candleUsersMu.Unlock()

	wsg.candleStreamsMu.Unlock()

	if user != nil {
		user.CandleSubscriptions[key] = true
	}

	log.Printf("user %s started new candle stream %s", user.ID, key)
	go wsg.receiveCandleFromRedis(key, pubsub)
}

func (wsg *WSGateway) unsubscribeFromCandleByKey(user *User, key string) {
	wsg.candleStreamsMu.Lock()
	defer wsg.candleStreamsMu.Unlock()

	wsg.candleUsersMu.Lock()
	delete(wsg.candleUsers[key], user)
	remaining := len(wsg.candleUsers[key])
	wsg.candleUsersMu.Unlock()

	delete(user.CandleSubscriptions, key)

	if remaining == 0 {
		if pubsub, exists := wsg.candleStreams[key]; exists {
			pubsub.Close()
			delete(wsg.candleStreams, key)
		}
		delete(wsg.candleUsers, key)
		log.Printf("closed candle stream %s (no subscribers)", key)
	}
}

func (wsg *WSGateway) receiveCandleFromRedis(key string, pubsub *redis.PubSub) {
	defer func() {
		// remove dead stream entry
		wsg.candleStreamsMu.Lock()
		delete(wsg.candleStreams, key)
		wsg.candleStreamsMu.Unlock()

		// reconnect only if users still subscribed (intentional close = 0 users)
		wsg.candleUsersMu.RLock()
		hasUsers := len(wsg.candleUsers[key]) > 0
		wsg.candleUsersMu.RUnlock()

		if hasUsers {
			log.Printf("candle stream %s dropped, reconnecting in 1s", key)
			time.Sleep(time.Second)
			go wsg.reconnectCandleStream(key)
		}
	}()

	symbol, timeframe := parseKeyParts(key)
	tfDisplay := strings.ToUpper(timeframe) // e.g. "TIMEFRAME_1M"

	for msg := range pubsub.Channel() {
		candle := &pbAgg.Candle{}
		if err := proto.Unmarshal([]byte(msg.Payload), candle); err != nil {
			log.Printf("candle unmarshal failed on %s: %v", key, err)
			continue
		}

		// encode once, fan-out to all subscribers
		payload, err := json.Marshal(&CandleWSPayload{
			EventType: "candle",
			Symbol:    symbol,
			Timeframe: tfDisplay,
			Data:      candle,
		})
		if err != nil {
			log.Printf("candle json marshal failed: %v", err)
			continue
		}

		event := &Event{EventType: EventTypeCandle, Data: payload}

		wsg.candleUsersMu.RLock()
		snapshot := make([]*User, 0, len(wsg.candleUsers[key]))
		for u := range wsg.candleUsers[key] {
			snapshot = append(snapshot, u)
		}
		wsg.candleUsersMu.RUnlock()

		for _, u := range snapshot {
			select {
			case u.Channel <- event:
			default:
				log.Printf("user %s channel full, dropping candle on %s", u.ID, key)
			}
		}
	}
}

func (wsg *WSGateway) reconnectCandleStream(key string) {
	wsg.candleUsersMu.RLock()
	hasUsers := len(wsg.candleUsers[key]) > 0
	wsg.candleUsersMu.RUnlock()

	if !hasUsers {
		return
	}

	pubsub := wsg.redisClient.Subscribe(context.Background(), key)

	wsg.candleStreamsMu.Lock()
	wsg.candleStreams[key] = pubsub
	wsg.candleStreamsMu.Unlock()

	go wsg.receiveCandleFromRedis(key, pubsub)
}

// ─── Client message handling ──────────────────────────────────────────────────

type UserMessage struct {
	Action    string `json:"action"`
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"` // for subscribe_candles / unsubscribe_candles
}

func (wsg *WSGateway) handleUserMessage(user *User, message []byte) {
	var msg UserMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	switch msg.Action {
	case "subscribe":
		wsg.subscribe(user, msg.Symbol)
	case "unsubscribe":
		wsg.unsubscribe(user, msg.Symbol)
	case "subscribe_candles":
		wsg.handleSubscribeCandles(user, msg)
	case "unsubscribe_candles":
		wsg.handleUnsubscribeCandles(user, msg)
	}
}

func (wsg *WSGateway) handleSubscribeCandles(user *User, msg UserMessage) {
	if msg.Symbol == "" {
		return
	}
	if !timeframeValid(msg.Timeframe) {
		log.Printf("user %s sent invalid timeframe %q", user.ID, msg.Timeframe)
		return
	}
	wsg.subscribeToCandle(user, msg.Symbol, msg.Timeframe)
}

func (wsg *WSGateway) handleUnsubscribeCandles(user *User, msg UserMessage) {
	if msg.Symbol == "" || msg.Timeframe == "" {
		return
	}
	key := candleKey(msg.Symbol, msg.Timeframe)
	if user.CandleSubscriptions[key] {
		wsg.unsubscribeFromCandleByKey(user, key)
	}
}

// ─── Read / Write pumps ───────────────────────────────────────────────────────

func (u *User) readPump(wsg *WSGateway) {
	defer func() {
		for symbol := range u.Subscriptions {
			wsg.unsubscribe(u, symbol)
		}
		for key := range u.CandleSubscriptions {
			wsg.unsubscribeFromCandleByKey(u, key)
		}
		u.Conn.Close()
	}()

	u.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	u.Conn.SetPongHandler(func(appData string) error {
		u.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := u.Conn.ReadMessage()
		if err != nil {
			fmt.Print(err)
			break
		}
		wsg.handleUserMessage(u, message)
	}
}

func (u *User) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-u.Channel:
			u.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			if !ok {
				u.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			switch event.EventType {
			case EventTypeCandle:
				// Data is already JSON — send directly
				if err := u.Conn.WriteMessage(websocket.TextMessage, event.Data); err != nil {
					log.Printf("failed to write candle event to user %s: %v", u.ID, err)
					return
				}

			case pbType.EventType_ORDER_ACCEPTED,
				pbType.EventType_ORDER_CANCELLED,
				pbType.EventType_ORDER_FILLED,
				pbType.EventType_ORDER_REJECTED:

				var data pb.OrderStatusEvent
				if err := proto.Unmarshal(event.Data, &data); err != nil {
					log.Printf("failed to unmarshal order status event for user %s: %v", u.ID, err)
					return
				}
				resp := &struct {
					EventType string               `json:"eventType"`
					Data      *pb.OrderStatusEvent `json:"data"`
				}{EventType: event.EventType.String(), Data: &data}
				if err := u.Conn.WriteJSON(resp); err != nil {
					log.Printf("failed to write order status event to user %s: %v", u.ID, err)
					return
				}

			case pbType.EventType_ORDER_REDUCED:
				var data pb.OrderReducedEvent
				if err := proto.Unmarshal(event.Data, &data); err != nil {
					log.Printf("failed to unmarshal order reduced event for user %s: %v", u.ID, err)
					return
				}
				resp := &struct {
					EventType string                `json:"eventType"`
					Data      *pb.OrderReducedEvent `json:"data"`
				}{EventType: event.EventType.String(), Data: &data}
				if err := u.Conn.WriteJSON(resp); err != nil {
					log.Printf("failed to write order reduced event to user %s: %v", u.ID, err)
					return
				}

			case pbType.EventType_TRADE_EXECUTED:
				var data pb.TradeEvent
				if err := proto.Unmarshal(event.Data, &data); err != nil {
					log.Printf("failed to unmarshal trade event for user %s: %v", u.ID, err)
					return
				}
				resp := &struct {
					EventType string         `json:"eventType"`
					Data      *pb.TradeEvent `json:"data"`
				}{EventType: event.EventType.String(), Data: &data}
				if err := u.Conn.WriteJSON(resp); err != nil {
					log.Printf("failed to write trade event to user %s: %v", u.ID, err)
					return
				}

			case pbType.EventType_DEPTH:
				var data pb.DepthEvent
				if err := proto.Unmarshal(event.Data, &data); err != nil {
					log.Printf("failed to unmarshal depth event for user %s: %v", u.ID, err)
					return
				}
				resp := &struct {
					EventType string         `json:"eventType"`
					Data      *pb.DepthEvent `json:"data"`
				}{EventType: event.EventType.String(), Data: &data}
				if err := u.Conn.WriteJSON(resp); err != nil {
					log.Printf("failed to write depth event to user %s: %v", u.ID, err)
					return
				}

			case pbType.EventType_TICKER:
				var data pb.TickerEvent
				if err := proto.Unmarshal(event.Data, &data); err != nil {
					log.Printf("failed to unmarshal ticker event for user %s: %v", u.ID, err)
					return
				}
				resp := &struct {
					EventType string          `json:"eventType"`
					Data      *pb.TickerEvent `json:"data"`
				}{EventType: event.EventType.String(), Data: &data}
				if err := u.Conn.WriteJSON(resp); err != nil {
					log.Printf("failed to write ticker event to user %s: %v", u.ID, err)
					return
				}
			}

		case <-ticker.C:
			u.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := u.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ─── HTTP handler ─────────────────────────────────────────────────────────────

func (wsg *WSGateway) HandelWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsg.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		id = fmt.Sprintf("client-%d", time.Now().UnixNano())
	}

	user := &User{
		ID:                  id,
		Conn:                conn,
		Channel:             make(chan *Event, 1024),
		Subscriptions:       make(map[string]bool),
		CandleSubscriptions: make(map[string]bool),
	}

	go user.readPump(wsg)
	go user.writePump()
}
