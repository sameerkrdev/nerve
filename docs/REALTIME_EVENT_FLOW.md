# Real-Time Event Flow — WebSocket Delivery

How events produced inside the system reach a connected browser client.

---

## Overview

```
                    PUBLIC (no token required)
                    ───────────────────────────────────────────────────────────────────
  Matching Engine ──→ Redis pub/sub ─────────────────────────────→ websocket-server
   depth:{SYM}                                                          │
   ticker:{SYM}                                                         │  fan-out to all
                                                                        │  subscribers
  Candle Service  ──→ Redis pub/sub ─────────────────────────────→ websocket-server
   candles:{SYM}:{tf}                                                   │
                                                                        │
                    PRIVATE (JWT token required)                        ▼
                    ───────────────────────────────────────────────────────────────────
  Matching Engine ──→ Redis pub/sub ─────────────────────────────→ websocket-server ──→ Client
   order:{userID}                                                       │ per-user sub
```

---

## Connection & Auth

```
Client                             websocket-server
  │                                       │
  │  GET /api/v1/ws                       │
  │  ?token=<JWT>  ──────────────────────→│
  │                                       │  1. parse RS256 JWT (local key — no auth-service call)
  │                                       │  2. check Redis bl:{jti} (revoked token guard)
  │                                       │  3. upgrade to WebSocket
  │  ←────────────────────── 101 ─────────│
  │                                       │  if token valid  → isAuthenticated=true, ID=claims.sub
  │                                       │  if no token     → isAuthenticated=false, ID=anon-{nanos}
  │                                       │  if bad token    → 401, connection refused
```

Token is **optional**. Unauthenticated connections get depth, ticker, and candle streams.
Order stream requires authentication.

---

## Client Message Protocol

All messages are JSON. Client sends an action, server fans out events.

### Subscribe / Unsubscribe

| Action                | Auth required | Payload fields          |
| --------------------- | ------------- | ----------------------- |
| `subscribe_depth`     | No            | `{ symbol }`            |
| `unsubscribe_depth`   | No            | `{ symbol }`            |
| `subscribe_ticker`    | No            | `{ symbol }`            |
| `unsubscribe_ticker`  | No            | `{ symbol }`            |
| `subscribe_candles`   | No            | `{ symbol, timeframe }` |
| `unsubscribe_candles` | No            | `{ symbol, timeframe }` |
| `subscribe_orders`    | **Yes**       | `{}`                    |
| `unsubscribe_orders`  | **Yes**       | `{}`                    |

Sending an auth-required action without a valid token returns:

```json
{ "eventType": "ERROR", "error": "authentication required" }
```

---

## Event Types Received by Client

### Depth (`subscribe_depth`)

```json
{
  "eventType": "DEPTH",
  "data": {
    "symbol": "BTCUSD",
    "bids": [{ "price": 89900, "quantity": 2.5 }, ...],
    "asks": [{ "price": 90000, "quantity": 1.0 }, ...]
  }
}
```

Source: matching engine → `depth:{SYM}` Redis channel → websocket-server fan-out.
Fired after every trade execution and at final book state.

---

### Ticker (`subscribe_ticker`)

```json
{
  "eventType": "TICKER",
  "data": {
    "symbol": "BTCUSD",
    "lastPrice": "89950",
    "volume24h": "1240.5",
    "high24h": "91000",
    "low24h": "88500"
  }
}
```

Source: matching engine → `ticker:{SYM}` Redis channel → websocket-server fan-out.

---

### Candle (`subscribe_candles`)

```json
{
  "eventType": "candle",
  "symbol": "BTCUSD",
  "timeframe": "1M",
  "data": {
    "open": "89000",
    "high": "90100",
    "low": "88900",
    "close": "89950",
    "volume": "42.1",
    "openTime": "1748000000"
  }
}
```

Source: candle-service → `candles:{SYM}:{tf}` Redis channel → websocket-server fan-out.
Fires on every trade that updates the active candle (not just on close).

---

### Order Events (`subscribe_orders` — auth only)

All order events share the structure:

```json
{ "eventType": "<EVENT_TYPE>", "data": { ...proto fields } }
```

| eventType         | When fired                                          | Key fields                                  |
| ----------------- | --------------------------------------------------- | ------------------------------------------- |
| `ORDER_ACCEPTED`  | Order received by engine, not yet matched           | orderId, symbol, side, price, quantity      |
| `TRADE_EXECUTED`  | A match occurred (buyer AND seller both receive)    | tradeId, price, quantity, buyerId, sellerId |
| `ORDER_FILLED`    | An order's remaining quantity reached zero          | orderId, avgPrice, filledQty                |
| `ORDER_REDUCED`   | Part of a resting order was cancelled               | orderId, reducedQty, remainingQty           |
| `ORDER_CANCELLED` | Order fully cancelled                               | orderId                                     |
| `ORDER_REJECTED`  | Order rejected (market order with empty book, etc.) | orderId, reason                             |

---

## Internal Fan-out Architecture

### Depth / Ticker / Candle (shared-subscription fan-out)

```
First subscriber for BTCUSD depth:
  fanoutStream.subscribe(user, "BTCUSD")
    → redis.Subscribe("depth:BTCUSD")
    → goroutine: receive() reads channel → emit to all users

Nth subscriber for BTCUSD depth:
  fanoutStream.subscribe(user, "BTCUSD")
    → existing PubSub reused, user added to set
    → same goroutine fans out to N users

Last subscriber unsubscribes:
  fanoutStream.unsubscribe(user, "BTCUSD")
    → PubSub.Close() called
    → goroutine exits, no reconnect
```

One Redis subscription per (stream, key) pair regardless of subscriber count.

### Order (per-user subscription)

```
On authenticated connect:
  startOrderStream(user)
    → redis.Subscribe("order:{userID}")
    → goroutine: receives EngineEvent proto bytes → emit to user

On disconnect:
  stopOrderStream(user) → PubSub.Close()
```

### Reconnect on Redis Drop

Both fan-out and order streams auto-reconnect after 1s delay if users are still subscribed/connected and the gateway context is not cancelled.

---

## Full Path: Trade → Both Traders' Screens

```
t=0    Matching engine executes trade between Alice (buyer) and Bob (seller)

t=0    PublishEngineEvent(TRADE_EXECUTED):
         → redis.Publish("order:alice-id", engineEventBytes)
         → redis.Publish("order:bob-id",   engineEventBytes)
         → redis.Publish("depth:BTCUSD",   depthBytes)
         → redis.Publish("ticker:BTCUSD",  tickerBytes)

t=1ms  websocket-server order stream goroutine (Alice):
         receives bytes → proto.Unmarshal(EngineEvent)
         → emit(&Event{TRADE_EXECUTED, data}) → Alice's send channel
         → writePump → conn.WriteJSON → Alice's browser

t=1ms  websocket-server order stream goroutine (Bob):
         same path → Bob's browser

t=1ms  websocket-server depth fanout goroutine:
         receives bytes → emit to all BTCUSD depth subscribers
         → writePump → conn.WriteJSON → all depth subscribers' browsers

t=~2s  Candle service processes TRADE_EXECUTED from Kafka:
         → updates BTCUSD candles
         → redis.Publish("candles:BTCUSD:1M", candleBytes)

t=~2s  websocket-server candle fanout goroutine:
         receives bytes → JSON marshal (CandleWSPayload)
         → emit to all BTCUSD:1M candle subscribers' browsers
```

---

## Redis Keys Reference

| Key pattern          | Publisher       | Subscriber                    | Content                          |
| -------------------- | --------------- | ----------------------------- | -------------------------------- |
| `depth:{SYM}`        | Matching engine | websocket-server              | DepthEvent proto bytes           |
| `ticker:{SYM}`       | Matching engine | websocket-server              | TickerEvent proto bytes          |
| `order:{userID}`     | Matching engine | websocket-server              | EngineEvent proto bytes          |
| `candles:{SYM}:{tf}` | Candle service  | websocket-server              | Candle proto bytes               |
| `bl:{jti}`           | auth-service    | websocket-server, api-gateway | `"1"` TTL = remaining token life |
| `rt:{tokenID}`       | auth-service    | auth-service                  | Refresh token record             |
