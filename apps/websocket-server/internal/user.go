package internal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

type User struct {
	ID              string
	Conn            *websocket.Conn
	send            chan *Event
	isAuthenticated bool
	orderSub        *redis.PubSub
}

type outboundMsg struct {
	EventType string `json:"eventType"`
	Data      any    `json:"data"`
}

type errorPayload struct {
	EventType string `json:"eventType"`
	Error     string `json:"error"`
}

func (u *User) sendProtoJSON(eventType string, data []byte, target proto.Message) error {
	if err := proto.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return u.Conn.WriteJSON(&outboundMsg{EventType: eventType, Data: target})
}

func (u *User) readPump(wsg *WSGateway) {
	defer func() {
		wsg.deregisterUser(u)
		u.Conn.Close()
	}()

	u.Conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
	u.Conn.SetPongHandler(func(string) error {
		u.Conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
		return nil
	})

	for {
		_, raw, err := u.Conn.ReadMessage()
		if err != nil {
			break
		}
		wsg.handleMessage(u, raw)
	}
}

func (u *User) writePump() {
	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()

	for {
		select {
		case event, ok := <-u.send:
			u.Conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
			if !ok {
				u.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := u.dispatch(event); err != nil {
				slog.Warn("write failed, closing", "user", u.ID, "err", err)
				return
			}

		case <-ping.C:
			u.Conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
			if err := u.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (u *User) dispatch(event *Event) error {
	switch event.EventType {
	case EventTypeError:
		var p errorPayload
		if err := json.Unmarshal(event.Data, &p); err != nil {
			return err
		}
		return u.Conn.WriteJSON(&p)

	case EventTypeCandle:
		return u.Conn.WriteMessage(websocket.TextMessage, event.Data)

	case pbType.EventType_ORDER_ACCEPTED,
		pbType.EventType_ORDER_CANCELLED,
		pbType.EventType_ORDER_FILLED,
		pbType.EventType_ORDER_REJECTED:
		return u.sendProtoJSON(event.EventType.String(), event.Data, &pb.OrderStatusEvent{})

	case pbType.EventType_ORDER_REDUCED:
		return u.sendProtoJSON(event.EventType.String(), event.Data, &pb.OrderReducedEvent{})

	case pbType.EventType_TRADE_EXECUTED:
		return u.sendProtoJSON(event.EventType.String(), event.Data, &pb.TradeEvent{})

	case pbType.EventType_DEPTH:
		return u.sendProtoJSON(event.EventType.String(), event.Data, &pb.DepthEvent{})

	case pbType.EventType_TICKER:
		return u.sendProtoJSON(event.EventType.String(), event.Data, &pb.TickerEvent{})
	}

	return nil
}

// Drops silently if the buffer is full (live data — missed tick is acceptable).
func (u *User) emit(event *Event) {
	select {
	case u.send <- event:
	default:
		slog.Warn("user channel full, dropping event", "user", u.ID)
	}
}
