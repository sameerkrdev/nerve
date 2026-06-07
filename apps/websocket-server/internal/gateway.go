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
)

const (
	userChannelBuf  = 1024
	wsReadDeadline  = 60 * time.Second
	wsWriteDeadline = 10 * time.Second
	wsPingInterval  = 54 * time.Second
	reconnectDelay  = time.Second
)

type Event struct {
	EventType pbType.EventType
	Data      []byte
}

type WSGateway struct {
	ctx    context.Context
	cancel context.CancelFunc

	depthStreams   map[string]*redis.PubSub
	depthStreamsMu sync.RWMutex
	depthUsers     map[string]map[*User]bool
	depthUsersMu   sync.RWMutex

	tickerStreams   map[string]*redis.PubSub
	tickerStreamsMu sync.RWMutex
	tickerUsers     map[string]map[*User]bool
	tickerUsersMu   sync.RWMutex

	connectedUsers   map[string]*User
	connectedUsersMu sync.RWMutex

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
		depthStreams:   make(map[string]*redis.PubSub),
		depthUsers:     make(map[string]map[*User]bool),
		tickerStreams:  make(map[string]*redis.PubSub),
		tickerUsers:    make(map[string]map[*User]bool),
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
		ID:         id,
		Conn:       conn,
		send:       make(chan *Event, userChannelBuf),
		depthSubs:  make(map[string]bool),
		tickerSubs: make(map[string]bool),
		candles:    make(map[string]bool),
	}

	wsg.registerUser(user)

	go user.writePump()
	go user.readPump(wsg)

	wsg.startOrderStream(user)

}

func (wsg *WSGateway) registerUser(user *User) {
	wsg.connectedUsersMu.Lock()
	wsg.connectedUsers[user.ID] = user
	wsg.connectedUsersMu.Unlock()
	slog.Info("user connected", "id", user.ID)
}

func (wsg *WSGateway) deregisterUser(user *User) {
	// remove from connectedUsers FIRST so reconnect guards see user as gone
	wsg.connectedUsersMu.Lock()
	delete(wsg.connectedUsers, user.ID)
	wsg.connectedUsersMu.Unlock()

	for symbol := range user.depthSubs {
		wsg.unsubscribeDepth(user, symbol)
	}
	for symbol := range user.tickerSubs {
		wsg.unsubscribeTicker(user, symbol)
	}
	for key := range user.candles {
		wsg.unsubscribeCandle(user, key)
	}
	wsg.stopOrderStream(user)

	slog.Info("user disconnected", "id", user.ID)
}

type baseMsg struct {
	Action string `json:"action"`
}

type SymbolMsg struct {
	Symbol string `json:"symbol"`
}

type CandleMsg struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
}

func (wsg *WSGateway) handleMessage(user *User, raw []byte) {
	var base baseMsg
	if err := json.Unmarshal(raw, &base); err != nil {
		return
	}

	switch base.Action {
	case "subscribe_depth", "unsubscribe_depth",
		"subscribe_ticker", "unsubscribe_ticker":
		var msg SymbolMsg
		if err := json.Unmarshal(raw, &msg); err != nil || msg.Symbol == "" {
			return
		}
		switch base.Action {
		case "subscribe_depth":
			wsg.subscribeDepth(user, msg.Symbol)
		case "unsubscribe_depth":
			if user.depthSubs[msg.Symbol] {
				wsg.unsubscribeDepth(user, msg.Symbol)
			}
		case "subscribe_ticker":
			wsg.subscribeTicker(user, msg.Symbol)
		case "unsubscribe_ticker":
			if user.tickerSubs[msg.Symbol] {
				wsg.unsubscribeTicker(user, msg.Symbol)
			}
		}

	case "subscribe_orders":
		wsg.startOrderStream(user)
	case "unsubscribe_orders":
		wsg.stopOrderStream(user)

	case "subscribe_candles", "unsubscribe_candles":
		var msg CandleMsg
		if err := json.Unmarshal(raw, &msg); err != nil || msg.Symbol == "" || msg.Timeframe == "" {
			return
		}
		switch base.Action {
		case "subscribe_candles":
			if timeframeValid(msg.Timeframe) {
				wsg.subscribeToCandle(user, msg.Symbol, msg.Timeframe)
			}
		case "unsubscribe_candles":
			key := candleKey(msg.Symbol, msg.Timeframe)
			if user.candles[key] {
				wsg.unsubscribeCandle(user, key)
			}
		}
	}
}
