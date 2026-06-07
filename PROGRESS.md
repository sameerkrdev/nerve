# Nerve — 10-Day Progress Tracker

Started: 2026-06-02

---

## Legend

- ✅ Done
- 🔄 In Progress
- ⬜ Not Started
- ❌ Blocked / Incomplete

---

## matching-engine — Fan-out Refactor ✅

- ✅ `streamSender` — buffered per-stream channel (cap 2048), drain goroutine; actor never blocks on gRPC write
- ✅ Redis pub/sub fan-out: `depth:{SYM}`, `ticker:{SYM}` publish inner proto bytes; `order:{userID}` publishes full EngineEvent bytes
- ✅ `event.Symbol` stamped on all events in actor loop before broadcast/publish
- ✅ `REDIS_URL` env — non-fatal if missing (WAL/Kafka unaffected)
- ✅ `PublishEngineEvent` — goroutine-dispatched, non-blocking to actor loop

---

## Phase 1 — Market Data Pipeline (Days 1–3)

### 1. candle-service `apps/candle-service` ✅

- ✅ Kafka consumer (reads trade events)
- ✅ Worker pool + router
- ✅ In-memory CandleStore (L1) — OHLCV per symbol per timeframe, auto-closes candles
- ✅ Redis pub/sub publish on every tick (`PublishCandleEventToRedis`) — proto-marshalled bytes
- ✅ Redis L2 storage — `PushCandle` called on candle close, LTrim 5000 cap
- ✅ gRPC server — `GetCandles` with L1 → L2 (Redis) → L3 (ClickHouse) fallthrough
- ✅ ClickHouse L3 — `FetchCandles` aggregates OHLCV from `trades` table on-the-fly (no separate candles table)
- ✅ `onCandleClosed` composite: PushCandle (L2) + Kafka publish (no CH write — trades table is source of truth)

---

### 2. trade-ingestor-service `apps/trade-ingestor-service` ✅

- ✅ Kafka consumer client
- ✅ `TradeBatcher` — size + time-based flush (500 msgs or 50ms)
- ✅ ClickHouse batch insert (`InsertTrades`) — marks Kafka offsets only on success
- ✅ `EnsureTradesSchema` — creates `trades` table on startup
- ✅ Consumer handler wired to batcher — no immediate MarkMessage

---

### 3. indicator-service ⬜ (not created yet)

- Subscribe to Redis pub/sub candle channel (`candles:<SYMBOL>:<tf>`)
- Compute indicators on new candle (RSI, EMA, MACD, Bollinger — pick what's needed)
- Store latest indicator values in Redis (L2) and in-memory (L1)
- Publish indicator updates to Redis pub/sub for WS consumers

---

## Phase 2 — Real-Time WebSocket for Market Data (Day 4)

### 4. websocket-server — candle + depth + order streams `apps/websocket-server` ✅

- ✅ Candle stream — Redis pub/sub per (symbol, timeframe), fan-out to subscribers
- ✅ `subscribe_candles` / `unsubscribe_candles` message routing
- ✅ Depth/ticker stream — Redis pub/sub per symbol (`depth:{SYM}` + `ticker:{SYM}`), fan-out to subscribers
- ✅ `subscribe_depth` / `unsubscribe_depth` message routing
- ✅ Order/trade stream — per-user Redis sub (`order:{userID}`) started on connect, stopped on disconnect
- ✅ Auto-reconnect on Redis drop (only if users still subscribed / connected)
- ✅ Pre-encode JSON once at fan-out time (zero per-user marshal work)
- ✅ Full refactor into 6 files: gateway, user, depth_stream, candle_stream, order_stream, candle (keys)
- ✅ Non-blocking `writePump` via buffered per-user channel (1024 cap, drop with warn on full)
- ❌ No indicator stream (indicator-service not built yet)
- ❌ gRPC engine connection removed — order/depth events now via Redis (see matching-engine below)

---

## Phase 3 — User Auth & Ledger (Days 5–6)

### 5. User Auth — in-memory + Prisma DB ⬜

- JWT-based auth (access + refresh)
- In-memory session cache (map[userID]session)
- Prisma DB for persistence (users table)
- Register / Login / Refresh / Logout routes
- Middleware for order-service, api-gateway

### 6. ledger-service ⬜

- Add / withdraw balance (dummy — no real payment gateway)
- Hold / release margin
- gRPC interface for order-service to call
- In-memory + DB persistence

---

## Phase 4 — Order Risk & Derivatives (Days 7–8)

### 7. Margin calculations in order-service ⬜

- Initial margin = (price × quantity) / leverage
- Available margin check before order placement
- Call ledger-service to hold margin on order placement
- Release margin on cancel / fill

### 8. Leverage & Liquidation ⬜

- Per-position leverage (isolated or cross)
- Liquidation price calculation
- Liquidation trigger — background monitor watching mark price vs liquidation price
- Force-close position via order-service when triggered

### 9. Stop Loss / Take Profit ⬜

- New order types: STOP_LOSS, TAKE_PROFIT
- Trigger monitor (compare mark price against SL/TP)
- Place market order when triggered
- Cancel on position close

---

## Phase 5 — Async Kafka & Reliability (Day 9)

### 10. Matching engine — sync → async Kafka publish ⬜

- Current: matching engine publishes to Kafka synchronously (blocks engine loop)
- Goal: async — engine writes to internal channel, separate goroutine flushes to Kafka
- WAL checkpoint file already exists (`wal/*/checkpoint.meta`) — track publish offset there
- Candle-service: already async (consumer-side), no change needed
- Risk: on crash between WAL write and Kafka publish, replay from checkpoint offset
- https://chatgpt.com/s/t_69ee8613950881918281d91d6e8a6849

**Migration path:**

1. Add internal `kafkaQueue chan []byte` to engine
2. Flush goroutine: dequeue → produce → update checkpoint offset
3. On startup: read checkpoint, seek Kafka consumer to that offset for replay

---

## Phase 6 — Frontend (Day 10)

### 11. Frontend `apps/web` ⬜ (React + Vite scaffold only)

- Trading chart (TradingView Lightweight Charts or similar)
- Real-time candle feed via WS
- Orderbook / depth view
- Order placement form
- Portfolio / positions panel
- Auth pages (login, register)

---

## Services Map

| Service                   | Language | Status       | Transport                                      |
| ------------------------- | -------- | ------------ | ---------------------------------------------- |
| matching-engine           | Go       | ✅ Core done | gRPC (orders) + Redis pub/sub (events)         |
| candle-service            | Go       | ✅ 100%      | gRPC + Kafka + Redis + ClickHouse              |
| trade-ingestor-service    | Go       | ✅ 100%      | Kafka → ClickHouse                             |
| indicator-service         | -        | ⬜ 0%        | Redis pub/sub                                  |
| websocket-server          | Go       | ✅ ~90%      | WS + Redis pub/sub (depth/ticker/order/candle) |
| order-service             | TS       | 🔄 basic     | gRPC                                           |
| order-trade-store-service | TS       | 🔄 basic     | Kafka → DB                                     |
| api-gateway               | TS       | 🔄 basic     | HTTP                                           |
| market-maker              | TS       | ⬜ stub      | -                                              |
| ledger-service            | -        | ⬜ 0%        | gRPC                                           |
| web                       | TS/React | ⬜ scaffold  | WS + HTTP                                      |

---

## Daily Log

| Day | Date       | Done                                                                                                                                                                  |
| --- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | 2026-06-02 | Audit & roadmap created. candle-service L1/L2/pub-sub mostly wired. Identified bugs.                                                                                  |
| 2   | 2026-06-03 | Phase 1 complete. candle-service L1→L2→L3 fallthrough + CH insert. trade-ingestor TradeBatcher with size+time flush.                                                  |
| 3   | 2026-06-06 | Phase 2 complete. WS server candle stream. Full refactor into 6 files. CH materialized views + EnsureSchema for cloud. proto.Marshal fix for Redis publish.           |
| 4   | 2026-06-07 | matching-engine fan-out refactor. streamSender non-blocking broadcast. Redis pub/sub for depth/ticker/order events. WS server drops gRPC — pure Redis event delivery. |
| 5   |            |                                                                                                                                                                       |
| 6   |            |                                                                                                                                                                       |
| 6   |            |                                                                                                                                                                       |
| 7   |            |                                                                                                                                                                       |
| 8   |            |                                                                                                                                                                       |
| 9   |            |                                                                                                                                                                       |
| 10  |            |                                                                                                                                                                       |
