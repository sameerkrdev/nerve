package internal

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

type wsClaims struct {
	Email string `json:"email"`
	JTI   string `json:"jti"`
	jwt.RegisteredClaims
}

type WSGateway struct {
	ctx          context.Context
	cancel       context.CancelFunc
	jwtPublicKey *rsa.PublicKey
	upgrader     websocket.Upgrader
	redis        *redis.Client

	depth  *fanoutStream
	ticker *fanoutStream
	candle *fanoutStream

	connectedUsersMu sync.RWMutex
	connectedUsers   map[string]*User
}

func NewWSGateway(redisClient *redis.Client, jwtPublicKey *rsa.PublicKey) *WSGateway {
	ctx, cancel := context.WithCancel(context.Background())

	wsg := &WSGateway{
		ctx:            ctx,
		cancel:         cancel,
		jwtPublicKey:   jwtPublicKey,
		redis:          redisClient,
		connectedUsers: make(map[string]*User),
		upgrader:       websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}

	wsg.depth = newFanoutStream(ctx, redisClient, "depth", depthKey,
		func(_ string, data []byte) (*Event, error) {
			return &Event{EventType: pbType.EventType_DEPTH, Data: data}, nil
		})
	wsg.ticker = newFanoutStream(ctx, redisClient, "ticker", tickerKey,
		func(_ string, data []byte) (*Event, error) {
			return &Event{EventType: pbType.EventType_TICKER, Data: data}, nil
		})
	wsg.candle = newFanoutStream(ctx, redisClient, "candle",
		func(key string) string { return key },
		makeCandleEvent)

	return wsg
}

func (wsg *WSGateway) Shutdown() {
	wsg.cancel()
}

func (wsg *WSGateway) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	var userID string
	isAuthenticated := false

	if tokenStr := r.URL.Query().Get("token"); tokenStr != "" {
		claims := &wsClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return wsg.jwtPublicKey, nil
		}, jwt.WithLeeway(30*time.Second))
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if claims.JTI != "" {
			n, err := wsg.redis.Exists(r.Context(), "bl:"+claims.JTI).Result()
			if err == nil && n > 0 {
				http.Error(w, "token revoked", http.StatusUnauthorized)
				return
			}
		}
		userID = claims.Subject
		isAuthenticated = true
	} else {
		userID = fmt.Sprintf("anon-%d", time.Now().UnixNano())
	}

	conn, err := wsg.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	user := &User{
		ID:              userID,
		Conn:            conn,
		send:            make(chan *Event, userChannelBuf),
		isAuthenticated: isAuthenticated,
	}

	wsg.registerUser(user)
	go user.writePump()
	go user.readPump(wsg)

	if isAuthenticated {
		wsg.startOrderStream(user)
	}
}

func (wsg *WSGateway) registerUser(user *User) {
	wsg.connectedUsersMu.Lock()
	wsg.connectedUsers[user.ID] = user
	wsg.connectedUsersMu.Unlock()
	slog.Info("user connected", "id", user.ID)
}

func (wsg *WSGateway) deregisterUser(user *User) {
	wsg.connectedUsersMu.Lock()
	delete(wsg.connectedUsers, user.ID)
	wsg.connectedUsersMu.Unlock()

	wsg.depth.removeUser(user)
	wsg.ticker.removeUser(user)
	wsg.candle.removeUser(user)
	wsg.stopOrderStream(user)

	slog.Info("user disconnected", "id", user.ID)
}

// authRequiredActions lists WS actions that require an authenticated connection.
var authRequiredActions = map[string]bool{
	"subscribe_orders":   true,
	"unsubscribe_orders": true,
}

type baseMsg struct {
	Action string `json:"action"`
}

type symbolMsg struct {
	Symbol string `json:"symbol"`
}

type candleMsg struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
}

func (wsg *WSGateway) handleMessage(user *User, raw []byte) {
	var base baseMsg
	if err := json.Unmarshal(raw, &base); err != nil {
		return
	}

	if authRequiredActions[base.Action] && !user.isAuthenticated {
		data, _ := json.Marshal(&errorPayload{EventType: "ERROR", Error: "authentication required"})
		user.emit(&Event{EventType: EventTypeError, Data: data})
		return
	}

	switch base.Action {
	case "subscribe_depth", "unsubscribe_depth",
		"subscribe_ticker", "unsubscribe_ticker":
		var msg symbolMsg
		if err := json.Unmarshal(raw, &msg); err != nil || msg.Symbol == "" {
			return
		}
		switch base.Action {
		case "subscribe_depth":
			wsg.depth.subscribe(user, msg.Symbol)
		case "unsubscribe_depth":
			wsg.depth.unsubscribe(user, msg.Symbol)
		case "subscribe_ticker":
			wsg.ticker.subscribe(user, msg.Symbol)
		case "unsubscribe_ticker":
			wsg.ticker.unsubscribe(user, msg.Symbol)
		}

	case "subscribe_orders":
		wsg.startOrderStream(user)
	case "unsubscribe_orders":
		wsg.stopOrderStream(user)

	case "subscribe_candles", "unsubscribe_candles":
		var msg candleMsg
		if err := json.Unmarshal(raw, &msg); err != nil || msg.Symbol == "" || msg.Timeframe == "" {
			return
		}
		switch base.Action {
		case "subscribe_candles":
			if timeframeValid(msg.Timeframe) {
				wsg.candle.subscribe(user, candleKey(msg.Symbol, msg.Timeframe))
			}
		case "unsubscribe_candles":
			wsg.candle.unsubscribe(user, candleKey(msg.Symbol, msg.Timeframe))
		}
	}
}
