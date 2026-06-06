package internal

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	pbType "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
	"google.golang.org/protobuf/proto"
)

// User represents a connected WebSocket client.
// symbols and candles are only accessed from readPump — no mutex needed.
type User struct {
	ID   string
	Conn *websocket.Conn
	send chan *Event

	symbols map[string]bool // engine symbol subscriptions
	candles map[string]bool // candle key subscriptions (candleKey → true)
}

// outboundMsg is the JSON envelope written to the WS client.
type outboundMsg struct {
	EventType string `json:"eventType"`
	Data      any    `json:"data"`
}

// sendProtoJSON unmarshals proto bytes into target then JSON-encodes to WS.
func (u *User) sendProtoJSON(eventType string, data []byte, target proto.Message) error {
	if err := proto.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return u.Conn.WriteJSON(&outboundMsg{EventType: eventType, Data: target})
}

// readPump reads client messages and dispatches actions.
// One goroutine per user. Owns the WS read path.
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

// writePump drains user.send and writes events to the WS connection.
// One goroutine per user. Owns the WS write path.
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

// dispatch routes a single event to the WS connection.
func (u *User) dispatch(event *Event) error {
	switch event.EventType {
	case EventTypeCandle:
		// Data is pre-encoded JSON — write directly
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

// emit sends an event to the user's channel non-blocking.
// Drops silently if the buffer is full (live data — missed tick is acceptable).
func (u *User) emit(event *Event) {
	select {
	case u.send <- event:
	default:
		slog.Warn("user channel full, dropping event", "user", u.ID)
	}
}
