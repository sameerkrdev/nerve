# Matching Engine — Full System Design Document

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Architecture](#2-architecture)
3. [Directory Structure](#3-directory-structure)
4. [Data Structures](#4-data-structures)
5. [Core Algorithms](#5-core-algorithms)
6. [Event System](#6-event-system)
7. [gRPC API](#7-grpc-api)
8. [Actor Model & Concurrency](#8-actor-model--concurrency)
9. [Write-Ahead Log (WAL)](#9-write-ahead-log-wal)
10. [Kafka Event Pipeline](#10-kafka-event-pipeline)
11. [Order Lifecycle (End-to-End)](#11-order-lifecycle-end-to-end)
12. [WAL Replay & Recovery](#12-wal-replay--recovery)
13. [Error Handling](#13-error-handling)
14. [Miro Diagram Guide](#14-miro-diagram-guide)

---

## 1. System Overview

The matching engine is a **high-performance, event-driven, exchange-grade order matching system** written in Go. It processes orders for multiple trading symbols (BTCUSD, ETHUSD, SOLUSD) with strict **price-time priority** using an actor model that guarantees per-symbol sequential consistency.

### Key Properties

| Property           | Value                               |
| ------------------ | ----------------------------------- |
| Language           | Go                                  |
| Transport          | gRPC (protobuf)                     |
| Persistence        | Write-Ahead Log (WAL)               |
| Event broadcast    | Kafka (`engine-events` topic)       |
| Matching algorithm | Price-Time Priority (FIFO)          |
| Order types        | LIMIT, MARKET                       |
| Supported symbols  | BTCUSD, ETHUSD, SOLUSD              |
| gRPC port          | `localhost:50052`                   |
| Kafka brokers      | `localhost:19092`, `19093`, `19094` |

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLIENTS                                      │
│   PlaceOrder / CancelOrder / ModifyOrder / SubscribeSymbol (gRPC)    │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    gRPC SERVER  (:50052)                             │
│   server.go — PlaceOrder, CancelOrder, ModifyOrder, Subscribe        │
└───┬──────────────────┬──────────────────┬───────────────────────────┘
    │                  │                  │
    ▼                  ▼                  ▼
┌─────────┐      ┌─────────┐      ┌─────────┐
│  ACTOR  │      │  ACTOR  │      │  ACTOR  │
│ BTCUSD  │      │ ETHUSD  │      │ SOLUSD  │
│inbox:8k │      │inbox:8k │      │inbox:8k │
└────┬────┘      └────┬────┘      └────┬────┘
     │                │                │
     ▼                ▼                ▼
┌─────────┐      ┌─────────┐      ┌─────────┐
│MATCHING │      │MATCHING │      │MATCHING │
│ ENGINE  │      │ ENGINE  │      │ ENGINE  │
│ BTCUSD  │      │ ETHUSD  │      │ SOLUSD  │
└────┬────┘      └────┬────┘      └────┬────┘
     │                │                │
     ├──────────────────────────────── ┤
     │          DUAL OUTPUT            │
     ▼                                 ▼
┌──────────────────┐      ┌────────────────────────┐
│   WAL (per sym)  │      │  gRPC Streams (push)   │
│  wal/BTCUSD/     │      │  All subscribed clients │
│  wal/ETHUSD/     │      └────────────────────────┘
│  wal/SOLUSD/     │
└────────┬─────────┘
         │  (batch read every 2s)
         ▼
┌────────────────────────────────────────┐
│   Kafka Producer  →  engine-events     │
│   checkpoint.meta tracks last offset   │
└────────────────────────────────────────┘
```

---

## 3. Directory Structure

```
matching-engine/
├── cmd/
│   └── matching-engine/
│       └── main.go              # Entry point: gRPC server init, actor setup
├── internal/
│   ├── engine.go                # Core matching logic, price levels, actors (1372 lines)
│   ├── server.go                # gRPC handler implementations (126 lines)
│   ├── actor_registry.go        # Actor dispatch, message routing (162 lines)
│   ├── kafka.go                 # Kafka producer wrapper (186 lines)
│   ├── wal.go                   # Write-ahead log (469 lines)
│   └── utils.go                 # Protobuf encoding helpers (112 lines)
├── wal/
│   ├── BTCUSD/
│   │   ├── 0.log                # Segment 0 (up to 64 MB)
│   │   ├── 1.log                # Segment 1 (auto-rotated)
│   │   └── checkpoint.meta      # Last Kafka-emitted offset
│   ├── ETHUSD/
│   └── SOLUSD/
├── go.mod
├── makefile
└── .air.toml                    # Hot-reload config
```

---

## 4. Data Structures

### 4.1 Order

The fundamental unit of the system. Stored in price levels as a doubly-linked list node.

```
Order
├── Symbol          int64       trading symbol code
├── Price           int64       limit price (0 for MARKET)
├── AveragePrice    int64       weighted avg fill price
├── ExecutedValue   int64       total fill value
├── Quantity        int64       original order quantity
├── FilledQuantity  int64       how much has been matched
├── RemainingQuantity int64     quantity still to fill
├── CancelledQuantity int64     quantity cancelled
├── Side            enum        BUY | SELL
├── Type            enum        LIMIT | MARKET
├── Status          enum        PENDING | ACCEPTED | FILLED | PARTIAL_FILLED | CANCELLED | REJECTED
├── UserID          string      owner of this order
├── ClientOrderID   string      unique ID supplied by client (used as map key)
├── StatusMessage   string      human-readable status reason
├── ClientTimestamp *Timestamp  when client sent
├── GatewayTimestamp *Timestamp when gateway received
├── EngineTimestamp *Timestamp  when engine processed
├── Prev  *Order               ← linked list pointer (within price level)
├── Next  *Order               → linked list pointer (within price level)
└── PriceLevel *PriceLevel     pointer back to parent level
```

### 4.2 PriceLevel

Represents all orders at a single price. Maintains FIFO order queue and links to adjacent price levels.

```
PriceLevel
├── Price         int64           the price point
├── TotalVolume   uint64          sum of RemainingQuantity of all orders
├── OrderCount    uint64          number of live orders
├── HeadOrder     *Order          first (oldest) order — matched first
├── TailOrder     *Order          last (newest) order — added to tail
├── PrevPrice     *PriceLevel     next better price (toward book top)
└── NextPrice     *PriceLevel     next worse price (toward book bottom)
```

**Price Level Sorted Order:**

- **BID side**: descending (highest price = BestPriceLevel, i.e. 100 → 99 → 98...)
- **ASK side**: ascending (lowest price = BestPriceLevel, i.e. 90 → 91 → 92...)

### 4.3 OrderBookSide

One half of the order book (either all bids or all asks).

```
OrderBookSide
├── Side           enum                  BUY | SELL
├── PriceLevels    map[int64]*PriceLevel  O(1) lookup by price
└── BestPriceLevel *PriceLevel            pointer to top of book
```

### 4.4 MatchingEngine

The state machine for one symbol. Always lives inside exactly one SymbolActor.

```
MatchingEngine
├── Symbol          string
├── Bids            *OrderBookSide    all buy orders
├── Asks            *OrderBookSide    all sell orders
├── AllOrders       map[string]*Order all active orders by ClientOrderID
├── TotalMatches    uint64
├── TotalVolume     uint64
├── TradeSequence   uint64            monotonic trade counter
├── OrderSequence   uint64            monotonic order counter
└── wal             *SymbolWAL        reference for logging
```

### 4.5 SymbolActor

Owns one matching engine. All operations are serialized through the inbox channel.

```
SymbolActor
├── symbol        string
├── inbox         chan EngineMsg         buffered, capacity 8192
├── engine        *MatchingEngine
├── wal           *SymbolWAL
├── kafkaEmitter  *KafkaProducerWorker
├── grpcStreams    []SubscribeServer      active subscriber streams
└── mu            sync.RWMutex           guards grpcStreams
```

### 4.6 SymbolWAL

Per-symbol segmented write-ahead log.

```
SymbolWAL
├── dirPath               string         e.g. "wal/BTCUSD"
├── symbol                string
├── bufferWriter          *bufio.Writer  in-memory write buffer
├── nextOffset            uint64         next sequence number to assign
├── currentSegmentFile    *os.File       file handle
├── maxFileSize           int64          67,108,864 bytes (64 MB)
├── currentSegmentIndex   int            0 → 1 → 2 ... (0.log, 1.log, ...)
├── shouldFsync           bool           fsync after flush
├── syncIntervalMM        int            400 ms periodic flush
├── syncTimer             *time.Timer
├── ctx / cancel                         context for goroutine lifecycle
└── mu                    sync.Mutex     guards writes and rotation
```

### 4.7 KafkaProducerWorker

Reads WAL in batches and publishes to Kafka.

```
KafkaProducerWorker
├── producer        sarama.SyncProducer
├── Symbol          string
├── dirPath         string
├── batchSize       int                 300 events per batch
├── emitTimeMM      int                 2000 ms between emit ticks
├── wal             *SymbolWAL
├── checkpointFile  *os.File            checkpoint.meta
└── ctx             context.Context
```

### 4.8 Trade

Immutable record of a matched trade.

```
Trade
├── TradeID         string     "{Symbol}-T{timestamp}-{sequence}"
├── Symbol          string
├── TradeSequence   uint64
├── Price           int64      price at which trade executed (resting order's price)
├── Quantity        uint64     matched quantity
├── Timeline        *Timestamp
├── BuyerID         string
├── SellerID        string
├── BuyOrderID      string
├── SellOrderID     string
└── IsBuyerMaker    bool       true if the resting order was the BUY side
```

### 4.9 Message Types (Actor Inbox)

```
PlaceOrderMsg
├── Order   *Order
├── replay  chan *AddOrderInternalResponse
└── Err     chan error

CancelOrderMsg
├── OrderID string
├── UserID  string
├── Symbol  string
├── replay  chan *CancelOrderInternalResponse
└── Err     chan error

ModifyOrderMsg
├── OrderID     string
├── UserID      string
├── Symbol      string
├── NewPrice    *int64
├── NewQuantity *int64
├── replay      chan *ModifyOrderInternalResponse
└── Err         chan error
```

---

## 5. Core Algorithms

### 5.1 Matching Algorithm (Price-Time Priority)

**Entry point:** `MatchingEngine.AddOrderInternal(order)`

```
1. Check duplicate ClientOrderID → error if found

2. Determine opposite book:
   BUY order → check Asks
   SELL order → check Bids

3. MARKET order special case:
   If opposite book empty → REJECT (no liquidity)

4. Matching loop:
   while incoming.RemainingQuantity > 0 AND CanMatch(incoming, oppositeBook):

       a. Get BestPriceLevel from opposite book
       b. Get HeadOrder (oldest order at best price) — FIFO
       c. matchQty = min(incoming.RemainingQuantity, resting.RemainingQuantity)
       d. Execute trade at RESTING order's price
       e. incoming.RemainingQuantity -= matchQty
          incoming.FilledQuantity   += matchQty
          resting.RemainingQuantity -= matchQty
          resting.FilledQuantity    += matchQty
       f. Update average prices for both sides
       g. If resting.RemainingQuantity == 0:
            Remove resting from PriceLevel
            Add resting to filledRestingOrders[]
            If PriceLevel.OrderCount == 0: RemovePriceLevel()
       h. Record Trade

5. After loop:
   Set incoming status:
     incoming.RemainingQuantity == 0 → FILLED
     incoming.FilledQuantity > 0     → PARTIAL_FILLED (resting)
     else                            → stays PENDING (goes to book)

6. MARKET with remaining after loop → CANCEL remainder

7. LIMIT with remaining → add to own book:
   GetOrCreatePriceLevel(incoming.Price)
   PriceLevel.Push(incoming)
   AllOrders[incoming.ClientOrderID] = incoming
```

### 5.2 CanMatch (Price Validation)

```
MARKET order:  always true (liquidity check done before loop)

LIMIT BUY:     best ASK price <= incoming BUY price  → match
LIMIT SELL:    best BID price >= incoming SELL price  → match
```

### 5.3 Average Price Calculation

```
New FilledQty = old FilledQty + matchQty
New AvgPrice  = ((old AvgPrice × old FilledQty) + (matchPrice × matchQty)) / New FilledQty
ExecutedValue = AvgPrice × FilledQuantity
```

### 5.4 Price Level Linked List Insertion (LinkPriceLevel)

```
BID side (descending insertion):
  Start at BestPriceLevel
  Traverse NextPrice until:
    current == nil (new worst price) OR
    current.Price < newLevel.Price (insert before current)
  If new level beats BestPriceLevel → becomes new best

ASK side (ascending insertion):
  Start at BestPriceLevel
  Traverse NextPrice until:
    current == nil (new worst price) OR
    current.Price > newLevel.Price (insert before current)
  If new level beats BestPriceLevel → becomes new best
```

### 5.5 Modify Order Logic

Three possible outcomes:

| Condition                                         | Action                                                    |
| ------------------------------------------------- | --------------------------------------------------------- |
| Price changed OR new quantity > original quantity | **Replace**: cancel existing order + place new order      |
| New quantity < current remaining quantity         | **Reduce**: update quantities in-place (no priority loss) |
| No meaningful change                              | **No-op**                                                 |

Validation:

- Order must exist and belong to the requesting user
- Order must still be active (RemainingQuantity > 0)
- New quantity must be >= already-executed quantity

---

## 6. Event System

### 6.1 Event Types

| EventType              | Trigger                                   | Data Payload                       | Written to WAL | Sent to gRPC Streams |
| ---------------------- | ----------------------------------------- | ---------------------------------- | -------------- | -------------------- |
| `ORDER_ACCEPTED`       | Order passes validation                   | `OrderStatusEvent`                 | Yes            | Yes                  |
| `ORDER_REJECTED`       | MARKET with no liquidity, or duplicate ID | `OrderStatusEvent`                 | Yes            | Yes                  |
| `TRADE_EXECUTED`       | Two orders match                          | `TradeEvent`                       | Yes            | Yes                  |
| `ORDER_FILLED`         | Order fully matched                       | `OrderStatusEvent`                 | Yes            | Yes                  |
| `ORDER_PARTIAL_FILLED` | Order partially matched, resting          | `OrderStatusEvent`                 | Yes            | Yes                  |
| `ORDER_CANCELLED`      | User cancel or replace during modify      | `OrderStatusEvent`                 | Yes            | Yes                  |
| `ORDER_REDUCED`        | Quantity reduced in-place                 | `OrderReducedEvent`                | Yes            | Yes                  |
| `DEPTH`                | Any book change                           | `DepthEvent` (top 100 levels)      | **No**         | Yes                  |
| `TICKER`               | Trade executes                            | `TickerEvent` (last/bid/ask price) | **No**         | Yes                  |

> DEPTH and TICKER are ephemeral market-data events — they are NOT persisted to WAL and NOT replayed during recovery.

### 6.2 Event Sequence for a Matched Order

```
Incoming LIMIT BUY order partially fills 2 resting SELL orders:

1. ORDER_ACCEPTED        (incoming order acknowledged)
2. TRADE_EXECUTED        (match #1 with resting order A)
3. DEPTH                 (book updated after match #1)
4. TICKER                (price update after match #1)
5. ORDER_FILLED          (resting order A fully filled)
6. TRADE_EXECUTED        (match #2 with resting order B)
7. DEPTH                 (book updated after match #2)
8. TICKER                (price update after match #2)
9. ORDER_FILLED          (resting order B fully filled)
10. ORDER_PARTIAL_FILLED (incoming still has remaining qty → rests in book)
11. DEPTH                (final book state)
```

### 6.3 buildEvents Logic

```
func buildEvents(order, trades, filledResting):

  if order.REJECTED:
    return [ORDER_REJECTED]

  events = [ORDER_ACCEPTED]

  for each trade:
    events += [TRADE_EXECUTED]
    events += [DEPTH]
    events += [TICKER]

  for each filledRestingOrder:
    events += [ORDER_FILLED]

  switch order.Status:
    FILLED         → events += [ORDER_FILLED]
    PARTIAL_FILLED → events += [ORDER_PARTIAL_FILLED]
    CANCELLED      → events += [ORDER_CANCELLED]

  events += [DEPTH]  // always emit final depth

  return events
```

### 6.4 Depth Event (Market Depth)

Collects top 100 price levels from each side:

- **Bids**: descending from BestBidPrice (100, 99, 98...)
- **Asks**: ascending from BestAskPrice (90, 91, 92...)

Each level includes: price, total volume, order count.

### 6.5 Ticker Event

```
TickerEvent {
  last_price: price of most recent trade
  bid_price:  current best bid (BestPriceLevel.Price from Bids)
  ask_price:  current best ask (BestPriceLevel.Price from Asks)
}
```

---

## 7. gRPC API

**Service: `MatchingEngine`** (port 50052)

### PlaceOrder

```
Request:
  PlaceOrderRequest {
    symbol, price, quantity
    side (BUY|SELL), type (LIMIT|MARKET)
    user_id, client_order_id
    client_timestamp, gateway_timestamp
  }

Response:
  PlaceOrderResponse {
    order { all Order fields after processing }
  }

Behavior:
  - Synchronous: blocks until matching complete
  - Returns final order state (FILLED, PARTIAL_FILLED, REJECTED, or PENDING)
  - All events generated and persisted before response
```

### CancelOrder

```
Request:
  CancelOrderRequest {
    order_id, user_id, symbol
  }

Response:
  CancelOrderResponse {
    order { cancelled order state }
  }

Errors:
  - "order not found"
  - "unauthorized cancel" (wrong user_id)
  - "order already completed" (remaining qty = 0)
```

### ModifyOrder

```
Request:
  ModifyOrderRequest {
    order_id, user_id, symbol
    new_price (optional)
    new_quantity (optional)
  }

Response:
  ModifyOrderResponse {
    order { final order state }
  }

Errors:
  - "order not found"
  - "order does not belong to this user"
  - "order symbol mismatch"
  - "order is not modifiable"
  - "new quantity < executed quantity"
```

### SubscribeSymbol

```
Request:
  SubscribeRequest { symbol }

Stream:
  → EngineEvent (continuously pushed)

Behavior:
  - Server pushes events to client as they occur
  - Client added to actor's grpcStreams list
  - Removed automatically when context cancelled
  - Receives ALL event types for that symbol
```

---

## 8. Actor Model & Concurrency

### 8.1 Goroutine Topology

```
main goroutine
│
├── gRPC server (managed by grpc framework)
│
├── SymbolActor.Run()  [BTCUSD]      ← single goroutine per symbol
│   ├── wal.keepSyncing()             ← periodic WAL flush (400ms)
│   └── kafkaEmitter.Run()            ← periodic Kafka batch (2000ms)
│
├── SymbolActor.Run()  [ETHUSD]
│   ├── wal.keepSyncing()
│   └── kafkaEmitter.Run()
│
└── SymbolActor.Run()  [SOLUSD]
    ├── wal.keepSyncing()
    └── kafkaEmitter.Run()

Total goroutines: 1 (main) + 1 (gRPC) + 3×3 (per symbol) = ~11 goroutines
```

### 8.2 Message Flow (Request/Reply via Channels)

```
gRPC handler (goroutine N)
│
│  Create reply + error channels (buffered 1)
│  Build PlaceOrderMsg{Order, replay, Err}
│  actor.inbox <- msg
│
│  block on:
│    select {
│      case res := <-replay → return res
│      case err := <-Err    → return error
│    }
│
└──────────────────────────────────────────┐
                                           ▼
                                  actor.Run() loop
                                  │
                                  │  msg := <-inbox
                                  │  switch msg.(type):
                                  │    PlaceOrderMsg:
                                  │      resp, events, err = engine.AddOrderInternal()
                                  │      handle events (WAL, streams)
                                  │      msg.replay <- resp
                                  │      continue
                                  │
                                  └──────────────────
```

### 8.3 Locking Summary

| Lock                       | Owner         | Protects                              | Pattern                                       |
| -------------------------- | ------------- | ------------------------------------- | --------------------------------------------- |
| `SymbolActor.mu` (RWMutex) | SymbolActor   | `grpcStreams` slice                   | Write: subscribe/unsubscribe. Read: broadcast |
| `SymbolWAL.mu` (Mutex)     | SymbolWAL     | All file operations, sequence counter | Every WAL write, rotation, sync               |
| `kafkaOnce` (sync.Once)    | package-level | Kafka producer initialization         | One-time singleton                            |

**No locks on MatchingEngine** — all access is serialized through actor inbox (single goroutine processes all messages).

---

## 9. Write-Ahead Log (WAL)

### 9.1 File Layout

```
wal/
└── {SYMBOL}/
    ├── 0.log          first segment
    ├── 1.log          second segment (after rotation)
    └── checkpoint.meta  last Kafka-emitted WAL offset (uint64 as string)
```

### 9.2 Entry Wire Format

```
┌──────────────────────────────────────────────────────┐
│  [4 bytes]  entry length (little-endian int32)        │
├──────────────────────────────────────────────────────┤
│  WAL_Entry (protobuf):                               │
│    sequence_number  uint64                           │
│    data             bytes  (serialized EngineEvent)  │
│    CRC              uint32 (CRC32 of data+seq_bytes) │
└──────────────────────────────────────────────────────┘
```

### 9.3 Write Path

```
WriteEntry(data []byte):
  1. Lock mutex
  2. rotateFile() if (file_size + buffered) >= 64 MB
  3. Encode sequence as [8]byte little-endian
  4. CRC = CRC32(data + seqBytes)
  5. Build WAL_Entry{sequence, data, CRC}
  6. Marshal to protobuf bytes
  7. Write [4-byte length][marshaled entry] to bufio.Writer
  8. Increment nextOffset
  9. Unlock mutex
```

### 9.4 File Rotation

```
rotateFile():
  if currentFile.Size() + bufferWriter.Buffered() >= maxFileSize:
    Sync()                        // flush + fsync
    currentFile.Close()
    currentSegmentIndex++
    Create new file: "{index}.log"
    Reset bufio.Writer on new file
```

### 9.5 Periodic Sync

```
keepSyncing() goroutine:
  for {
    select {
      case <-syncTimer.C:
        Lock()
        bufferWriter.Flush()
        file.Sync() if shouldFsync
        Unlock()
        reset timer (400ms)
    }
  }
```

**Durability guarantee**: Events buffered in memory for at most 400ms before reaching disk. On crash within 400ms window, last events may be lost.

### 9.6 Sequence Numbering

- Starts at 0 from empty WAL
- On restart: reads last entry in most recent `.log` file, sets `nextOffset = lastSeq + 1`
- Monotonically increasing, never resets across file rotations
- Used as stable offset for Kafka checkpoint

### 9.7 Corruption Detection

Every read verifies:

```
actual = CRC32(entry.data + encode(entry.sequence_number))
if actual != entry.CRC → error "CRC mismatch: data may be corrupted"
```

---

## 10. Kafka Event Pipeline

### 10.1 Producer Configuration

```
RequiredAcks    = WaitForAll    // all ISR replicas must ACK
MaxRetries      = 5
Idempotent      = true          // exactly-once per session
MaxOpenRequests = 1             // required for idempotent mode
Topic           = "engine-events"
MessageKey      = Symbol        // ensures all symbol events → same partition
```

### 10.2 Batch Emit Loop

```
kafkaEmitter.Run():
  every 2000ms:
    processBatch()

processBatch():
  1. checkpoint = loadCheckpoint()         // read checkpoint.meta
  2. entries = wal.ReadFromTo(            // read WAL batch
       from: checkpoint + 1,
       to:   checkpoint + batchSize       // 300 events
     )
  3. if len(entries) == 0: return
  4. messages = build Kafka messages from entries
  5. err = producer.SendMessages(messages)
  6. if err != nil:
       log error
       return  // ← do NOT save checkpoint → batch will retry
  7. saveCheckpoint(entries.last.sequence_number)
```

### 10.3 At-Least-Once Delivery Guarantee

```
Timeline:
  t=0  WAL entry written (seq=100)
  t=2s batch sent to Kafka (seq 1-100)
  t=2s Kafka ACK received
  t=2s checkpoint.meta = "100"

  If crash at t=2s before checkpoint saved:
  t=restart  checkpoint still = "50"
  t=restart  batch resent for seq 51-100 (duplicates for 51-100)
  Consumers must handle idempotency by seq number or event ID
```

### 10.4 Kafka Topic Partitioning

```
engine-events topic
├── Partition 0 — BTCUSD events  (key="BTCUSD")
├── Partition 1 — ETHUSD events  (key="ETHUSD")
└── Partition 2 — SOLUSD events  (key="SOLUSD")
```

---

## 11. Order Lifecycle (End-to-End)

### 11.1 LIMIT BUY with Partial Fill

```
Client → PlaceOrder(BUY, LIMIT, price=100, qty=10)

[gRPC handler]
  Build Order struct
  PlaceOrderMsg → actor.inbox

[Actor.Run()]
  AddOrderInternal(order)
    MatchOrder():
      BestASK = 98 (≤ 100, can match)
      HeadOrder at 98 = resting SELL qty=6
      matchQty = min(10, 6) = 6
      Trade { price=98, qty=6 }
      resting SELL → FILLED, removed from book
      BestASK = 99 (≤ 100, can match)
      HeadOrder at 99 = resting SELL qty=7
      matchQty = min(4, 7) = 4
      Trade { price=99, qty=4 }
      incoming BUY → FILLED (remaining=0)
      resting SELL qty → 3 remaining

    AvgPrice = (98×6 + 99×4) / 10 = 98.4

    buildEvents():
      ORDER_ACCEPTED
      TRADE_EXECUTED (qty=6, price=98)
      DEPTH
      TICKER
      ORDER_FILLED  (resting SELL at 98)
      TRADE_EXECUTED (qty=4, price=99)
      DEPTH
      TICKER
      ORDER_FILLED  (incoming BUY)
      DEPTH

  For each event:
    → Write to WAL (except DEPTH/TICKER)
    → Send to all gRPC subscribers

  Reply → actor sends on replay channel

[gRPC handler]
  recv from replay channel
  Return PlaceOrderResponse
```

### 11.2 MARKET Order Rejection

```
Client → PlaceOrder(BUY, MARKET, qty=5)

MatchOrder():
  Asks.IsEmpty() == true
  incoming.Status = REJECTED
  incoming.StatusMessage = "Market order rejected: no liquidity..."

buildEvents():
  return [ORDER_REJECTED]

WAL.WriteEntry(ORDER_REJECTED)
Stream: send ORDER_REJECTED to subscribers
Reply: PlaceOrderResponse{Status: REJECTED}
```

### 11.3 Cancel Order

```
Client → CancelOrder(order_id="X", user_id="U", symbol="BTCUSD")

CancelOrderInternal():
  order = AllOrders["X"]
  if order.UserID != "U"         → error "unauthorized cancel"
  if order.RemainingQuantity == 0 → error "order already completed"
  Remove from PriceLevel
  if PriceLevel.OrderCount == 0: RemovePriceLevel()
  order.CancelledQuantity = order.RemainingQuantity
  order.RemainingQuantity = 0
  order.Status = CANCELLED
  buildCancelEvent()

WAL.WriteEntry(ORDER_CANCELLED)
Stream: send ORDER_CANCELLED
Reply: CancelOrderResponse
```

---

## 12. WAL Replay & Recovery

### 12.1 Startup Sequence

```
main():
  for each symbol:
    1. OpenWAL(symbol)
       - find last .log file
       - read last entry → set nextOffset = lastSeq + 1
    2. NewMatchingEngine(symbol, wal)
    3. NewKafkaProducerWorker(symbol, wal)
    4. actor = NewSymbolActor(symbol, engine, wal, kafka)
    5. actor.replayWAL(from=0)   ← reconstruct order book
    6. go actor.Run()
    7. go wal.keepSyncing()
    8. go kafkaEmitter.Run()
```

### 12.2 replayWAL Event Handling

```
replayWAL(from uint64):
  entries = wal.ReadFromToLast(from)
  for each entry:
    event = unmarshal(entry.data)
    switch event.Type:

      ORDER_ACCEPTED:
        order = reconstruct from OrderStatusEvent
        GetOrCreatePriceLevel(order.Price)
        PriceLevel.Push(order)
        AllOrders[order.ClientOrderID] = order

      TRADE_EXECUTED:
        update buy and sell orders: qty, avg price, status

      ORDER_CANCELLED:
        remove order from PriceLevel
        delete from AllOrders

      ORDER_REDUCED:
        update remaining + cancelled qty on order

      ORDER_REJECTED:
        delete from AllOrders (if exists)

      ORDER_FILLED:
        remove from PriceLevel
        delete from AllOrders
```

**Result**: After replay, `MatchingEngine.Bids`, `MatchingEngine.Asks`, and `AllOrders` are identical to their state at the moment of the crash.

---

## 13. Error Handling

### 13.1 Validation Errors (Returned to Client)

| Scenario                          | Error Message                                            |
| --------------------------------- | -------------------------------------------------------- |
| Duplicate ClientOrderID           | `"Duplicate Order ID: {id}"`                             |
| MARKET with no opposite liquidity | `"Market order rejected: no liquidity on opposite side"` |
| Cancel: order not found           | `"order not found"`                                      |
| Cancel: wrong user                | `"unauthorized cancel"`                                  |
| Cancel: already completed         | `"order already completed"`                              |
| Modify: new qty < executed        | `"new quantity < executed quantity"`                     |
| Modify: order not modifiable      | `"order is not modifiable"`                              |

### 13.2 I/O Error Strategy

| Error                   | Strategy                                                        |
| ----------------------- | --------------------------------------------------------------- |
| WAL write failure       | Skip remaining events, return error to caller via `Err` channel |
| Kafka emit failure      | Log error, **do not checkpoint**, retry on next 2s tick         |
| Checkpoint save failure | Log error, continue (next batch will retry from same offset)    |
| WAL CRC mismatch        | Return error from read function, propagates to replay           |

### 13.3 Stream Error Handling

```
On stream.Send(event) failure:
  error logged
  stream removed from grpcStreams list
  client must reconnect via SubscribeSymbol
```

---

## 14. Miro Diagram Guide

> Miro MCP is not connected to this session. Use the layout below to build the diagram manually in Miro. Create one board with 5 frames.

---

### Frame 1 — High-Level Architecture

**Shapes to create (left to right, top to bottom):**

```
[Clients]
  → (arrow) → [gRPC Server :50052]
                  → (arrow) → [Actor: BTCUSD | inbox:8192]
                  → (arrow) → [Actor: ETHUSD | inbox:8192]
                  → (arrow) → [Actor: SOLUSD | inbox:8192]

Each Actor:
  → (arrow down) → [Matching Engine]
  → (arrow right) → [WAL  wal/{SYMBOL}/]
  → (arrow right, dashed) → [gRPC Streams → Subscribers]

WAL:
  → (arrow down, timer icon) → [Kafka Producer Worker]
  → (arrow right) → [Kafka: engine-events topic]
                        ↓ Partition 0: BTCUSD
                        ↓ Partition 1: ETHUSD
                        ↓ Partition 2: SOLUSD

checkpoint.meta
  → (arrow) → [Kafka Producer Worker]  (shows last emitted offset)
```

**Color coding:**

- Clients: blue
- gRPC layer: purple
- Actor + Engine: green
- WAL: orange
- Kafka: red

---

### Frame 2 — Order Book Structure

**Shapes to create:**

```
OrderBookSide (BID)
  BestPriceLevel → PriceLevel[100]
                        TotalVolume: 50
                        OrderCount: 3
                        HeadOrder → [Order A] ↔ [Order B] ↔ [Order C]
                   ↓
                   PriceLevel[99]
                        HeadOrder → [Order D] ↔ [Order E]
                   ↓
                   PriceLevel[98]
                        HeadOrder → [Order F]

OrderBookSide (ASK)
  BestPriceLevel → PriceLevel[101]
                        HeadOrder → [Order G] ↔ [Order H]
                   ↓
                   PriceLevel[102]
                        HeadOrder → [Order I]
```

**Notes to add:**

- "BID side sorted descending (best = highest)"
- "ASK side sorted ascending (best = lowest)"
- "Orders at each level follow FIFO — Head matched first"
- "PrevPrice / NextPrice = doubly linked list"
- "Prev / Next on Order = FIFO queue within level"

---

### Frame 3 — Order Matching Flow

**Flowchart (top to bottom):**

```
START: PlaceOrder gRPC call
  ↓
Build Order struct
  ↓
Send PlaceOrderMsg to actor.inbox (block on reply channel)
  ↓
[Actor.Run() receives from inbox]
  ↓
AddOrderInternal()
  ↓
Duplicate ClientOrderID? ──YES──→ Error to caller
  ↓ NO
Order type = MARKET? ──YES──→ Opposite book empty? ──YES──→ ORDER_REJECTED
  ↓                                   ↓ NO
  ↓                               continue to match
  ↓
[Matching Loop]
  Can match? (CanMatch()) ──NO──→ [Exit loop]
  ↓ YES
  Get BestPriceLevel from opposite book
  ↓
  Get HeadOrder (FIFO)
  ↓
  matchQty = min(incoming.remaining, resting.remaining)
  ↓
  ExecuteTrade() → create Trade record
  ↓
  Update both orders: qty, avgPrice, status
  ↓
  resting.remaining == 0? ──YES──→ Remove from PriceLevel → filledResting[]
  ↓ NO (partial fill of resting)
  ↓
  incoming.remaining == 0? ──YES──→ Exit loop (FILLED)
  ↓ NO
  [Loop back to CanMatch]
  ↓
[After loop]
  MARKET with remaining? ──YES──→ Cancel remainder
  ↓ NO
  LIMIT with remaining? ──YES──→ Add to own order book (GetOrCreatePriceLevel → Push)
  ↓
buildEvents()
  ↓
For each event: WAL.WriteEntry() + stream.Send() to subscribers
  ↓
Send reply on replay channel
  ↓
gRPC returns PlaceOrderResponse
  ↓
END
```

---

### Frame 4 — Event Flow & WAL Pipeline

**Swimlane diagram (3 lanes):**

```
Lane 1: Matching Engine
  → Generates events []EngineEvent

Lane 2: Per-Event Processing
  For each event:
    Is DEPTH or TICKER?
      YES → only → gRPC stream.Send()
      NO  → WAL.WriteEntry(event) + gRPC stream.Send()

Lane 3: WAL → Kafka (async)
  keepSyncing() goroutine:
    Every 400ms → flush bufio.Writer → fsync

  kafkaEmitter.Run() goroutine:
    Every 2000ms:
      read checkpoint.meta
      read WAL batch [offset+1 .. offset+300]
      send batch to Kafka "engine-events" (key=symbol)
      if ACK: save new checkpoint
      if FAIL: do not checkpoint (retry next tick)
```

**Add notes:**

- "WAL seq numbers are monotonic across file rotations"
- "Each .log file max 64 MB then rotated"
- "CRC32 checksum on every WAL entry"
- "Kafka key=Symbol ensures partition affinity"
- "At-least-once: checkpoint saved ONLY after Kafka ACK"

**Event sequence box (right side):**

```
ORDER_ACCEPTED
TRADE_EXECUTED × N
DEPTH × N          ← not WAL
TICKER × N         ← not WAL
ORDER_FILLED × N   (resting)
ORDER_FILLED / PARTIAL_FILLED / CANCELLED  (incoming)
DEPTH              ← not WAL
```

---

### Frame 5 — Startup & WAL Recovery

**Timeline diagram (left to right):**

```
CRASH ──────────────────────────────────────────────────→ RESTART
                                                              ↓
                                                         OpenWAL()
                                                         Find last .log
                                                         Read last entry
                                                         nextOffset = lastSeq+1
                                                              ↓
                                                         replayWAL(from=0)
                                                              ↓
                                                    For each WAL entry:
                                                    ┌─────────────────────┐
                                                    │ ORDER_ACCEPTED       │
                                                    │ → add to book        │
                                                    │ ORDER_FILLED         │
                                                    │ → remove from book   │
                                                    │ ORDER_CANCELLED      │
                                                    │ → remove from book   │
                                                    │ TRADE_EXECUTED       │
                                                    │ → update quantities  │
                                                    │ ORDER_REDUCED        │
                                                    │ → update quantities  │
                                                    └─────────────────────┘
                                                              ↓
                                                    Order book fully reconstructed
                                                              ↓
                                                    Actor.Run() starts
                                                    kafkaEmitter.Run() starts
                                                    (resumes from checkpoint.meta)
```

**Add note:**

- "DEPTH and TICKER events are NOT in WAL — they are reconstructed live"
- "Kafka checkpoint is independent of WAL — Kafka resumes from checkpoint.meta"

---

### Miro Setup Steps

1. Open Miro → New Board → name it **"Matching Engine Architecture"**
2. Create 5 frames (use F key): name them as above
3. In each frame, add shapes using the toolbar:
   - Rectangles for systems/components
   - Diamonds for decisions
   - Swimlanes for event flows
   - Arrows for data flow (solid = sync, dashed = async)
4. Color code:
   - **Blue**: external clients
   - **Purple**: gRPC layer
   - **Green**: actor + matching engine
   - **Orange**: WAL
   - **Red**: Kafka

---

_Document generated: 2026-04-16_
_Codebase: ~2,475 lines of Go across 7 files_
