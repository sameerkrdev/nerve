package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const (
	userChannelBuf  = 1024
	wsReadDeadline  = 60 * time.Second
	wsWriteDeadline = 10 * time.Second
	wsPingInterval  = 54 * time.Second
	reconnectDelay  = time.Second
)

// Event is the internal message passed from broadcaster goroutines to writePump.
type Event struct {
	EventType pbType.EventType
	Data      []byte
}

type WSGateway struct {
	ctx    context.Context
	cancel context.CancelFunc

	// gRPC engine connection
	engineConn *grpc.ClientConn

	// one gRPC stream per symbol
	symbolStreams   map[string]pb.MatchingEngine_SubscribeSymbolClient
	symbolStreamsMu sync.RWMutex

	// symbol → users (depth / ticker fan-out)
	symbolUsers   map[string]map[*User]bool
	symbolUsersMu sync.RWMutex

	// all connected users by ID (order / trade routing)
	// populated on connect, removed on disconnect — NOT on subscribe/unsubscribe
	connectedUsers   map[string]*User
	connectedUsersMu sync.RWMutex

	// Redis candle pub/sub
	redisClient     *redis.Client
	candleStreams   map[string]*redis.PubSub
	candleStreamsMu sync.RWMutex
	candleUsers     map[string]map[*User]bool
	candleUsersMu   sync.RWMutex

	upgrader websocket.Upgrader
}

func NewWSGateway(redisClient *redis.Client) *WSGateway {
	ctx, cancel := context.WithCancel(context.Background())
	return &WSGateway{
		ctx:            ctx,
		cancel:         cancel,
		symbolStreams:  make(map[string]pb.MatchingEngine_SubscribeSymbolClient),
		symbolUsers:    make(map[string]map[*User]bool),
		connectedUsers: make(map[string]*User),
		redisClient:    redisClient,
		candleStreams:  make(map[string]*redis.PubSub),
		candleUsers:    make(map[string]map[*User]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (wsg *WSGateway) Shutdown() {
	wsg.cancel()
}

// ConnectToEngine dials the matching engine gRPC server.
func (wsg *WSGateway) ConnectToEngine(uri string) error {
	conn, err := grpc.NewClient(uri,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return fmt.Errorf("connect to engine: %w", err)
	}
	wsg.engineConn = conn
	return nil
}

// HandleWebSocket upgrades the HTTP connection and starts the user read/write pumps.
func (wsg *WSGateway) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsg.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		id = fmt.Sprintf("client-%d", time.Now().UnixNano())
	}

	user := &User{
		ID:      id,
		Conn:    conn,
		send:    make(chan *Event, userChannelBuf),
		symbols: make(map[string]bool),
		candles: make(map[string]bool),
	}

	wsg.registerUser(user)

	go user.readPump(wsg)
	go user.writePump()
}

func (wsg *WSGateway) registerUser(user *User) {
	wsg.connectedUsersMu.Lock()
	wsg.connectedUsers[user.ID] = user
	wsg.connectedUsersMu.Unlock()
	slog.Info("user connected", "id", user.ID)
}

func (wsg *WSGateway) deregisterUser(user *User) {
	// unsubscribe from all engine symbol streams
	for symbol := range user.symbols {
		wsg.unsubscribeEngineEvents(user, symbol)
	}
	// unsubscribe from all candle streams
	for key := range user.candles {
		wsg.unsubscribeCandle(user, key)
	}

	wsg.connectedUsersMu.Lock()
	delete(wsg.connectedUsers, user.ID)
	wsg.connectedUsersMu.Unlock()

	slog.Info("user disconnected", "id", user.ID)
}

// ─── Client message dispatch ──────────────────────────────────────────────────

type UserMessage struct {
	Action    string `json:"action"`
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
}

func (wsg *WSGateway) handleMessage(user *User, raw []byte) {
	var msg UserMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Action {
	case "subscribe_engine_events":
		if msg.Symbol != "" {
			wsg.subscribeEngineEvents(user, msg.Symbol)
		}
	case "unsubscribe_engine_events":
		if msg.Symbol != "" {
			wsg.unsubscribeEngineEvents(user, msg.Symbol)
		}
	case "subscribe_candles":
		wsg.handleSubscribeCandles(user, msg)
	case "unsubscribe_candles":
		wsg.handleUnsubscribeCandles(user, msg)
	}
}
