package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/proto"
)

type Event struct {
	EventType pbType.EventType
	Data      []byte
}

type WSGateway struct {
	symbolStreams map[string]pb.MatchingEngine_SubscribeSymbolClient
	streamsMu     sync.RWMutex

	engineConnection *grpc.ClientConn

	Users     map[string]map[*User]bool
	UsersByID map[string]*User
	mu        sync.RWMutex

	upgrader websocket.Upgrader
}

type User struct {
	ID            string
	Conn          *websocket.Conn
	Channel       chan *Event
	Subscriptions map[string]bool
}

func NewWSGateway() *WSGateway {
	return &WSGateway{
		symbolStreams: make(map[string]pb.MatchingEngine_SubscribeSymbolClient),
		Users:         make(map[string]map[*User]bool),
		UsersByID:     make(map[string]*User),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
			return true
		}},
	}
}

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
	fmt.Println("stream", stream)

	if err != nil {
		fmt.Println("stream1", err)
		return err
	}

	wsg.symbolStreams[symbol] = stream

	log.Printf("Subscribed to %s via dedicated stream", symbol)

	// Start receiving from this dedicated stream
	go wsg.receiveFromSymbolStream(symbol, stream)

	return nil
}

func (wsg *WSGateway) receiveFromSymbolStream(symbol string, stream grpc.ServerStreamingClient[pb.EngineEvent]) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Printf("%s stream error: %v", symbol, err)

			// Remove dead stream
			wsg.streamsMu.Lock()
			delete(wsg.symbolStreams, symbol)
			wsg.streamsMu.Unlock()

			// Reconnect logic here
			time.Sleep(time.Second)
			wsg.subscribeToSymbol(symbol)
			return
		}

		wsg.boardcastToUsers(symbol, msg)
	}
}

// TODO:- currently if user is subscribe to a symbol then he will recieve all
// symbol events but we want to send only envent which are subscibed by users like only depth, only order, etc.
func (wsg *WSGateway) boardcastToUsers(symbol string, msg *pb.EngineEvent) {
	switch msg.EventType {
	case
		pbType.EventType_ORDER_ACCEPTED,
		pbType.EventType_ORDER_CANCELLED,
		pbType.EventType_ORDER_FILLED,
		pbType.EventType_ORDER_REJECTED,
		pbType.EventType_ORDER_REDUCED:

		if user, exist := wsg.UsersByID[msg.UserId]; exist {
			user.Channel <- &Event{
				EventType: msg.EventType,
				Data:      msg.Data,
			}
		}

	case pbType.EventType_TRADE_EXECUTED:
		var data pb.TradeEvent
		if err := proto.Unmarshal(msg.Data, &data); err != nil {
			log.Printf("failed to unmashal the trade event of symbol %s", symbol)
			return
		}

		if user, exist := wsg.UsersByID[data.BuyerId]; exist {
			select {
			case user.Channel <- &Event{
				EventType: msg.EventType,
				Data:      msg.Data,
			}:
			default:
				log.Printf("User %s buffer full", user.ID)
			}
		}

		if user, exist := wsg.UsersByID[data.SellerId]; exist {
			select {
			case user.Channel <- &Event{
				EventType: msg.EventType,
				Data:      msg.Data,
			}:
			default:
				log.Printf("User %s buffer full", user.ID)
			}
		}
	case pbType.EventType_DEPTH, pbType.EventType_TICKER:
		wsg.mu.RLock()
		users, exists := wsg.Users[symbol]
		if !exists {
			wsg.mu.RUnlock()
			return
		}

		userList := make([]*User, 0, len(users))
		for user := range users {
			userList = append(userList, user)
		}
		wsg.mu.RUnlock()

		for _, user := range userList {
			select {
			case user.Channel <- &Event{
				EventType: msg.EventType,
				Data:      msg.Data,
			}:
			default:
				log.Printf("User %s buffer full", user.ID)
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

	log.Printf("User %s subscribed to %s (total: %d)",
		user.ID, symbol, len(wsg.Users[symbol]))

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
				log.Printf("Closed stream for %s (no more subscribers)", symbol)
			}
			wsg.streamsMu.Unlock()
		}
	}
}

type UserMessage struct {
	Action string `json:"action"`
	Symbol string `json:"symbol"`
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
	}
}

func (u *User) readPump(wsg *WSGateway) {
	defer func() {
		for symbol := range u.Subscriptions {
			wsg.unsubscribe(u, symbol)
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
	timer := time.NewTicker(54 * time.Second)
	defer timer.Stop()

	for {
		select {
		case event, ok := <-u.Channel:
			// todo: channel has event type property so that to determine the type and unmashal the data and send it to client
			u.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			if !ok {
				u.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			switch event.EventType {
			case pbType.EventType_ORDER_ACCEPTED,
				pbType.EventType_ORDER_CANCELLED,
				pbType.EventType_ORDER_FILLED,
				pbType.EventType_ORDER_REJECTED:

				var unmarshalData pb.OrderStatusEvent

				if err := proto.Unmarshal(event.Data, &unmarshalData); err != nil {
					log.Printf("failed to unmarshal order status event for user %s: %v", u.ID, err)
					return
				}

				resp := &struct {
					EventType string               `json:"eventType"`
					Data      *pb.OrderStatusEvent `json:"data"`
				}{
					EventType: event.EventType.String(),
					Data:      &unmarshalData,
				}

				if err := u.Conn.WriteJSON(resp); err != nil {
					log.Printf("failed to write order status event to user %s: %v", u.ID, err)
					return
				}

			case pbType.EventType_ORDER_REDUCED:
			case pbType.EventType_TRADE_EXECUTED:
			case pbType.EventType_DEPTH:
			case pbType.EventType_TICKER:
			}
		case <-timer.C:
			u.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := u.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

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
		ID:            id,
		Conn:          conn,
		Channel:       make(chan *Event, 1024),
		Subscriptions: make(map[string]bool),
	}

	go user.readPump(wsg)
	go user.writePump()
}
