# Exchange Platform Documentation

## 1. System Design

### **Overview:**

The exchange platform is designed for high throughput trading, with components for order ingestion, matching, market data, ledger management, notifications, and analytics. The system ensures durability using Kafka, Redis, and Postgres/ClickHouse.

### **Components:**

- **API Gateway:** REST + WebSocket endpoints, validates orders and forwards to Matching Engine via gRPC.
- **Matching Engine:** In-memory order book and trade execution engine. Receives orders via gRPC and produces trades.
- **Kafka:** Event streaming for durability and asynchronous processing.
- **Redis:** For caching, WebSocket pub/sub, and tick streams.
- **Ledger Service:** Handles balances, margin, and fee calculations using Prisma + Postgres.
- **Market Data Service:** Consumes trades from Kafka and generates ticks, stores in ClickHouse.
- **Notification Service:** Sends real-time updates to clients via WebSocket.
- **Analytics Service:** Processes historical data, generates OHLC, and provides reporting.

### **Data Flow:**

1. Client sends order via API Gateway (REST/WebSocket).
2. API Gateway forwards order via gRPC to Matching Engine.
3. Matching Engine processes in-memory order book.
4. Engine publishes trades and order updates to Kafka.
5. Ledger Service consumes trades, updates balances.
6. Market Data Service consumes trades, generates ticks → Redis Streams + ClickHouse.
7. Notification Service pushes events to WebSocket clients.
8. Analytics Service aggregates trades/ticks for reporting.

---

## 2. Folder Structure and Responsibilities

```
exchange-platform/
├── apps/
│   ├── api-gateway/        # REST + WebSocket API
│   ├── matching-engine/    # Core matching engine (in-memory order book)
│   ├── market-data-service/ # Tick generation and analytics
│   ├── ledger-service/     # User balances, margin, fees
│   ├── notification-service # WebSocket events
│   ├── analytics-service/  # Historical data & reporting
│   └── web-client/         # React/Next.js frontend
├── packages/
│   ├── proto-defs/         # gRPC proto files and generated code
│   ├── types/              # Shared TS types and enums
│   ├── kafka-lib/          # Kafka producer/consumer wrapper
│   ├── redis-lib/          # Redis client and helpers
│   ├── clickhouse/         # ClickHouse client and helpers
│   ├── prisma/             # Prisma schema and DB client
│   ├── logger/             # Centralized logging
│   ├── ws-events/          # WebSocket event definitions
│   ├── ui/                 # Shared UI components
│   └── config/             # Centralized config loader
├── infra/                  # Docker Compose + init scripts
├── docs/                   # Documentation and diagrams
└── .turbo/                  # Turborepo pipeline
```

### **Package Responsibilities:**

- **proto-defs:** gRPC schemas.
- **types:** TypeScript shared types.
- **kafka-lib:** Kafka producer/consumer abstractions.
- **redis-lib:** Redis Streams, caching, pub/sub.
- **clickhouse:** Tick & trade analytics storage.
- **prisma:** ORM schema and migrations.
- **logger:** Structured logging.
- **ws-events:** WebSocket event management.
- **ui:** Shared frontend components.
- **config:** Environment and constants.

---

## 3. Flow

### **Order Flow:**

```text
Client --> API Gateway (REST/WebSocket) --> gRPC --> Matching Engine --> Trades/Order Updates Kafka --> Ledger/Market Data/Notification --> Redis/ClickHouse --> WebSocket Clients
```

### **Detailed Steps:**

1. **Order Submission:** Client sends order to API Gateway.
2. **Validation:** Gateway checks user balance, order format.
3. **Forward to Engine:** Gateway calls Matching Engine via gRPC.
4. **Matching Engine:** Updates in-memory order book, executes trades.
5. **Event Publishing:** Trades and order updates published to Kafka.
6. **Ledger Update:** Ledger Service consumes trades, updates balances and margin.
7. **Tick Generation:** Market Data Service consumes trades, creates ticks, stores in ClickHouse, streams via Redis.
8. **Notification:** Notification Service sends real-time updates to WebSocket clients.
9. **Analytics:** Analytics Service aggregates historical trades/ticks for charts and reports.

---

## 4. Technology Stack

| Layer/Component       | Technology                    |
| --------------------- | ----------------------------- |
| Backend Language      | Go / Rust / Node.js           |
| Frontend              | React / Next.js / TailwindCSS |
| RPC                   | gRPC + Protobuf               |
| Event Streaming       | Kafka (kafkajs / sarama)      |
| Caching & Streams     | Redis (Streams + Pub/Sub)     |
| Relational DB         | Postgres (via Prisma ORM)     |
| Analytics DB          | ClickHouse                    |
| Logging               | Pino / Zap                    |
| Repository Management | Turborepo                     |
| Containerization      | Docker / Docker Compose       |
| Deployment            | Kubernetes (optional)         |

---

## 5. Timeline (45 Days)

### **Week 1: Foundation & Infra (Days 1–7)**

- Setup Turborepo structure
- Install Docker, Kafka, Redis, Postgres, ClickHouse
- Define gRPC proto files
- API Gateway skeleton
- WebSocket skeleton

### **Week 2: Matching Engine Core (Days 8–14)**

- In-memory order book
- Matching logic
- Trade generation
- Partial fills, cancel/update
- Unit tests
- gRPC integration

### **Week 3: Kafka Integration (Days 15–21)**

- Kafka producers/consumers
- Event persistence
- Replay mechanism
- Snapshot management

### **Week 4: Market Data + WebSockets (Days 22–28)**

- Tick generation
- Redis Streams integration
- ClickHouse storage
- WebSocket live updates
- Frontend live charts

### **Week 5: Ledger, Margin, Notifications (Days 29–37)**

- Ledger and balance management
- Margin calculations
- Trade fee deduction
- Notifications service
- User trade history

### **Week 6: Hardening + Analytics (Days 38–45)**

- Analytics dashboards
- Load testing & optimization
- Chaos testing & recovery
- Security review
- Deployment scripts
- Final documentation

---

**End of Documentation**
