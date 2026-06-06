# Place Order — End-to-End Data Flow

## System Overview

```
                                    SYNCHRONOUS PATH (gRPC chain)
                                    ─────────────────────────────

  ┌──────────┐     HTTP POST      ┌──────────────┐     gRPC       ┌───────────────┐     gRPC       ┌─────────────────┐
  │  Client  │ ──────────────────→│ API Gateway  │──────────────→│ Order Service │──────────────→│ Matching Engine │
  │          │ ←──────────────────│   (Express)  │←──────────────│   (gRPC srv)  │←──────────────│    (Go/gRPC)    │
  └──────────┘     JSON Response  └──────────────┘   Response     └───────────────┘   Response    └────────┬────────┘
                                                                                                          │
                                                                                                    WAL (disk)
                                                                                                          │
                                    ASYNCHRONOUS PATH (Kafka)                                             │
                                    ─────────────────────────                                              │
                                                                                                          ▼
                                                                                               ┌──────────────────┐
                                                                                               │  Kafka Producer  │
                                                                                               │  (batch/2s)      │
                                                                                               └────────┬─────────┘
                                                                                                        │
                                                                                              topic: "engine-events"
                                                                                                        │
                                              ┌─────────────────────────────────────────────────────────┤
                                              │                                                         │
                                              ▼                                                         ▼
                                  ┌────────────────────────┐                              ┌───────────────────────┐
                                  │ Order-Trade-Store Svc  │                              │  Candle Service       │
                                  │ (Kafka Consumer - TS)  │                              │  (Kafka Consumer - Go)│
                                  │                        │                              │                       │
                                  │ → Postgres (Prisma)    │                              │ → In-Memory Store     │
                                  │   - orders table       │                              │ → Redis (Pub/Sub + L2)│
                                  │   - trades table       │                              │ → Kafka "candles"     │
                                  └────────────────────────┘                              └───────────────────────┘
```

---

## Step-by-Step Flow

### Step 1 — Client → API Gateway (HTTP)

**Service**: `api-gateway` (Node.js / Express)
**Endpoint**: `POST /api/v1/orders`
**File**: `apps/api-gateway/src/routers/order.route.ts`

```
Client sends HTTP POST /api/v1/orders
Body: { symbol, price, quantity, side, type }
```

1. **Zod validation middleware** (`zod.validator.middleware.ts`) validates the request body against `PlaceOrderValidator` schema
2. **OrderController.createOrder()** (`controllers/order.controller.ts`) is invoked:
   - Extracts `symbol`, `price`, `quantity`, `side`, `type` from `req.body`
   - Attaches `userId` (hardcoded TODO — will come from auth)
   - Adds `clientTimestamp` and `gatewayTimestamp`
   - Converts `side`/`type` string enums to protobuf enum values
   - Calls **Order Service** via gRPC: `orderClient.createOrder(grpcRequest)`
3. Blocks until gRPC response arrives, then returns JSON:
   ```json
   { "message": "Order is placed successfully", "data": { ...orderResponse } }
   ```

---

### Step 2 — API Gateway → Order Service (gRPC)

**Service**: `order-service` (Node.js / gRPC server)
**Proto Service**: `OrderService.CreateOrder`
**File**: `apps/order-service/src/controllers/order.controller.ts`

1. **GrpcServer** (`grpc.server.ts`) initializes:
   - Creates a `MatchingEngineGrpcClient` connection to the matching engine
   - Registers `OrderServerController` as the `OrderService` implementation
2. **OrderServerController.placeOrder()** receives the gRPC call:
   - Generates a unique `clientOrderId` via `crypto.randomUUID()`
   - Builds the `PlaceOrderRequest` protobuf for the matching engine
   - Calls **Matching Engine** via gRPC: `matchingEngineClient.placeOrder(request)`
   - Blocks until the matching engine responds
   - Returns the merged response back to the API Gateway

> The Order Service is a **thin proxy** — it generates the order ID and forwards to the matching engine. It does not persist anything.

---

### Step 3 — Order Service → Matching Engine (gRPC)

**Service**: `matching-engine` (Go / gRPC server on `:50052`)
**Proto Service**: `MatchingEngine.PlaceOrder`
**File**: `apps/matching-engine/internal/server.go`

1. **Server.PlaceOrder()** (`server.go`) converts the protobuf request into an internal `Order` struct:
   - Sets `RemainingQuantity = Quantity` (nothing filled yet)
   - Stamps `EngineTimestamp = time.Now()`
2. Calls `PlaceOrder(order)` in the **actor registry** (`actor_registy.go`):
   - Looks up the `SymbolActor` for this symbol (e.g., `actors["BTCUSD"]`)
   - Creates reply + error channels
   - Sends `PlaceOrderMsg` into the actor's inbox channel
   - **Blocks** on `select { replay | err }` waiting for the actor

---

### Step 4 — Actor Processes the Order (Matching Engine Core)

**File**: `apps/matching-engine/internal/engine.go`

The `SymbolActor.Run()` goroutine picks up the message from its inbox:

#### 4a. AddOrderInternal()

1. **Duplicate check**: rejects if `ClientOrderID` already exists in `AllOrders` map
2. **MatchOrder()** — the core matching loop:
   - Determines the opposite book (BUY → check Asks, SELL → check Bids)
   - **MARKET + empty book** → immediately `REJECTED`
   - **Matching loop**: while `RemainingQuantity > 0` AND prices cross:
     - Gets `BestPriceLevel` from opposite book
     - Gets `HeadOrder` at best price (FIFO — oldest order first)
     - `matchQty = min(incoming.Remaining, resting.Remaining)`
     - **ExecuteTrade()** at the resting order's price → creates `Trade` record
     - Updates quantities, average prices on both orders
     - If resting order fully filled → removes from book
   - After loop: sets status to `FILLED` or `PARTIAL_FILLED`
3. **MARKET with remaining** → cancels the unfilled remainder
4. **LIMIT with remaining** → rests in the order book (`GetOrCreatePriceLevel` → `Push`)

#### 4b. buildEvents()

Generates an ordered sequence of `EngineEvent` protobuf messages:

| #   | Event                                           | Condition                           |
| --- | ----------------------------------------------- | ----------------------------------- |
| 1   | `ORDER_ACCEPTED`                                | Always (unless rejected)            |
| 2   | `TRADE_EXECUTED`                                | Per match                           |
| 3   | `DEPTH`                                         | After each trade (not persisted)    |
| 4   | `TICKER`                                        | After each trade (not persisted)    |
| 5   | `ORDER_FILLED`                                  | Per fully-filled resting order      |
| 6   | `ORDER_FILLED` / `PARTIAL_FILLED` / `CANCELLED` | Final state of incoming order       |
| 7   | `DEPTH`                                         | Final book snapshot (not persisted) |

If rejected: returns only `[ORDER_REJECTED]`

#### 4c. Event Dispatch (still inside the actor loop)

For each event in the list:

1. **gRPC Streams**: sends to all subscribed gateway streams via `stream.Send(event)`
2. **WAL Write**: persists to the Write-Ahead Log (skips `DEPTH` and `TICKER` — they are ephemeral)
3. Serializes the event via `proto.Marshal(event)` before writing

#### 4d. Reply

Sends `AddOrderInternalResponse` back on the reply channel → unblocks the gRPC handler → response flows back through Order Service → API Gateway → Client.

---

### Step 5 — WAL → Kafka (Asynchronous, every 2 seconds)

**File**: `apps/matching-engine/internal/kafka.go`

The `KafkaProducerWorker` runs in a background goroutine per symbol:

1. Every **2000ms**, calls `processBatch()`:
   - Reads `checkpoint.meta` to get the last emitted WAL offset
   - Reads up to **300** WAL entries from `checkpoint + 1`
   - Sends batch to Kafka topic **`engine-events`** (key = symbol name)
   - On success: saves new checkpoint offset
   - On failure: does NOT checkpoint → retries on next tick (at-least-once delivery)

**Kafka config**: `RequiredAcks=WaitForAll`, `Idempotent=true`, topic `engine-events`, key = symbol (partition affinity).

---

### Step 6 — Order-Trade-Store Service (Kafka Consumer → Postgres)

**Service**: `order-trade-store-service` (Node.js / Kafka consumer)
**File**: `apps/order-trade-store-service/src/kafka.consumer.ts`
**Database**: PostgreSQL via Prisma ORM

Consumes from Kafka topic `engine-events`, decodes `EngineEvent` protobuf, and handles:

| Event Type        | Action                                                                                                                                                                                                                       | Database Table      |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| `ORDER_ACCEPTED`  | `orderRepo.create()` — inserts the new order with all fields                                                                                                                                                                 | `orders`            |
| `TRADE_EXECUTED`  | `tradeRepo.create()` — inserts trade record linking buyer/seller/orders. Then `orderRepo.update()` on BOTH buy and sell orders — updates `filled_quantity`, `remaining_quantity`, `average_price`, `executedValue`, `status` | `trades` + `orders` |
| `ORDER_CANCELLED` | `orderRepo.update()` — sets `status=CANCELLED`, updates `remaining_quantity` and `cancelled_quantity`                                                                                                                        | `orders`            |
| `ORDER_REDUCED`   | `orderRepo.update()` — updates `remaining_quantity` and `cancelled_quantity`                                                                                                                                                 | `orders`            |
| `ORDER_REJECTED`  | `orderRepo.create()` — inserts the order with rejected status                                                                                                                                                                | `orders`            |

> This is the **system of record** for orders and trades — queryable by users and other services.

---

### Step 7 — Candle Service (Kafka Consumer → In-Memory + Redis + Kafka)

**Service**: `candle-service` (Go / Kafka consumer + gRPC server)
**File**: `apps/candle-service/internal/kafka/consumerHandler.go`
**Consumes from**: Kafka topic `trades`

1. **ConsumerHandler** deserializes `EngineEvent` protobuf from each message
2. Filters for `TRADE_EXECUTED` events only (ignores all other event types)
3. Routes to a **Worker** via consistent hashing on symbol (`WorkerRouter.Route()`)
4. **Worker.Process()** deserializes the `TradeEvent` from the event data
5. **CandleStore.AddNewCandle()** updates OHLCV candles for ALL timeframes simultaneously:
   - Computes the time bucket for the trade: `openTimeBucket = (timestamp / tfSeconds) * tfSeconds`
   - If no active candle exists → creates new candle with `O=H=L=C=tradePrice`, `V=tradeQty`
   - If trade falls in the same bucket → updates `H`, `L`, `C`, `V` on the active candle
   - If trade crosses into a new bucket → **closes** the current candle, pushes to history, creates new candle
6. **Fan-out on every candle update**:
   - **Redis Pub/Sub**: publishes live candle updates for WebSocket servers (`PublishCandleEventToRedis`)
   - **Redis List (L2 cache)**: stores last 5000 closed candles per symbol/timeframe (`PushCandle`)
   - **In-Memory (L1 cache)**: keeps last 1000 closed candles in `history` map
   - **Kafka**: publishes closed candles to `candles` topic for downstream services (`PublishCandleEventToKafka`)

---

## Complete Timeline for a Single Place Order

```
t=0ms     Client sends HTTP POST /api/v1/orders { symbol: "BTCUSD", price: 90000, quantity: 5, side: "BUY", type: "LIMIT" }
            │
t=1ms     API Gateway: Zod validation passes
            │
t=2ms     API Gateway → Order Service (gRPC: CreateOrder)
            │
t=3ms     Order Service: generates clientOrderId (UUID), forwards to Matching Engine (gRPC: PlaceOrder)
            │
t=4ms     Matching Engine: actor receives PlaceOrderMsg from inbox
            │
t=4ms     Matching Engine: AddOrderInternal()
            ├── MatchOrder(): finds 2 matching SELL orders at prices 89900 and 89950
            ├── ExecuteTrade() × 2 → creates 2 Trade records
            ├── Incoming order fully FILLED (remaining = 0)
            ├── buildEvents() → [ORDER_ACCEPTED, TRADE_EXECUTED, DEPTH, TICKER, ORDER_FILLED(resting),
            │                     TRADE_EXECUTED, DEPTH, TICKER, ORDER_FILLED(resting), ORDER_FILLED(incoming), DEPTH]
            │
t=5ms     For each event:
            ├── gRPC stream.Send(event) → pushes to all subscribed gateways (real-time)
            └── WAL.WriteEntry(event) → buffered write to disk (ORDER_ACCEPTED, TRADE_EXECUTED, ORDER_FILLED only)
            │
t=5ms     Actor sends response on reply channel
            │
t=6ms     Matching Engine → Order Service → API Gateway → Client
            │   Response: { status: FILLED, avgPrice: 89925, filledQty: 5, remainingQty: 0 }
            │
            │                         ──── SYNCHRONOUS PATH COMPLETE ────
            │
            │                         ──── ASYNCHRONOUS PATH BEGINS ────
            │
t=400ms   WAL keepSyncing(): flushes buffer to disk, fsync
            │
t=2000ms  KafkaProducerWorker: reads WAL batch → sends to Kafka "engine-events" topic → saves checkpoint
            │
            ├─────────────────────────────────────────────────────────┐
            │                                                         │
            ▼                                                         ▼
t=2100ms  Order-Trade-Store Service:                        Candle Service:
            ├── ORDER_ACCEPTED → INSERT into orders           ├── TRADE_EXECUTED → route to worker
            ├── TRADE_EXECUTED → INSERT into trades           ├── Update OHLCV for all timeframes
            │                    UPDATE both orders           ├── Redis Pub/Sub (live candle)
            ├── ORDER_FILLED  → UPDATE resting orders         └── On candle close:
            └── ORDER_FILLED  → UPDATE incoming order               ├── Redis L2 cache
                                                                    ├── In-memory L1 cache
                                                                    └── Kafka "candles" topic
```

---

## Service Summary

| Service               | Language   | Transport                    | Role in Place Order Flow                                                     |
| --------------------- | ---------- | ---------------------------- | ---------------------------------------------------------------------------- |
| **API Gateway**       | TypeScript | HTTP (Express) → gRPC client | Entry point. Validates request, converts to gRPC, returns response to client |
| **Order Service**     | TypeScript | gRPC server + gRPC client    | Generates order UUID, proxies to matching engine                             |
| **Matching Engine**   | Go         | gRPC server                  | Core. Matches orders, generates trades, writes WAL, pushes gRPC streams      |
| **Order-Trade-Store** | TypeScript | Kafka consumer               | Persists orders and trades to PostgreSQL (Prisma)                            |
| **Candle Service**    | Go         | Kafka consumer + gRPC server | Computes OHLCV candles, stores in memory/Redis, publishes to Kafka           |

---

## Kafka Topics

| Topic           | Producer                            | Consumers                              | Content                                                                                                         |
| --------------- | ----------------------------------- | -------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `engine-events` | Matching Engine (WAL → Kafka batch) | Order-Trade-Store Service              | All engine events: ORDER_ACCEPTED, TRADE_EXECUTED, ORDER_FILLED, ORDER_CANCELLED, ORDER_REDUCED, ORDER_REJECTED |
| `trades`        | —                                   | Candle Service, Trade Ingestor Service | Trade events (TRADE_EXECUTED)                                                                                   |
| `candles`       | Candle Service                      | Downstream services                    | Closed candle OHLCV data per symbol/timeframe                                                                   |

---

## Databases

| Database                | Service                                                   | Purpose                                                   |
| ----------------------- | --------------------------------------------------------- | --------------------------------------------------------- |
| **PostgreSQL** (Prisma) | Order-Trade-Store Service                                 | System of record for orders and trades                    |
| **WAL** (local files)   | Matching Engine                                           | Crash recovery journal, Kafka source                      |
| **Redis**               | Candle Service                                            | L2 candle cache (5000 per key) + Pub/Sub for live updates |
| **ClickHouse**          | API Gateway (reads), Trade Ingestor (writes - WIP)        | Historical trade/candle analytics                         |
| **In-Memory**           | Matching Engine (order book), Candle Service (L1 candles) | Hot path data — no disk latency                           |
