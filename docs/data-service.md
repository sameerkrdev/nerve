1. Complete System Overview

```mermaid
graph TB
subgraph "External"
EX[Exchange APIs<br/>Binance, Coinbase]
end

    subgraph "Data Ingestion Layer"
        KT[Kafka Topics<br/>trades, candles-1m, candles-5m]
        EX -->|Trade Events| KT
    end

    subgraph "Stream Processing Layer"
        CB[Candle Builder<br/>Stateful Stream]
        PIC[Popular Indicator Computer<br/>EMA, RSI, MACD]
        DIC[Dynamic Indicator Computer<br/>User Custom]

        KT -->|trades| CB
        CB -->|candles-1m| KT
        KT -->|candles-1m| PIC
        KT -->|candles-1m| DIC
    end

    subgraph "Storage Layer"
        CH[(ClickHouse<br/>OLAP Database)]
        RD[(Redis<br/>Cache + Pub/Sub)]

        CB -->|Persist Candles| CH
        PIC -->|Cache Popular| RD
        DIC -->|Cache Custom| RD
    end

    subgraph "Data Service"
        DS[Data Service<br/>gRPC Server]

        DS <-->|Query/Write| CH
        DS <-->|Cache Get/Set| RD
    end

    subgraph "WebSocket Gateway Layer"
        WS1[WebSocket Server 1]
        WS2[WebSocket Server 2]
        WS3[WebSocket Server N]
        LB[Load Balancer]

        LB --> WS1
        LB --> WS2
        LB --> WS3
    end

    subgraph "Clients"
        C1[Web Browser]
        C2[Mobile App]
        C3[Desktop App]

        C1 --> LB
        C2 --> LB
        C3 --> LB
    end

    RD -->|Pub/Sub| WS1
    RD -->|Pub/Sub| WS2
    RD -->|Pub/Sub| WS3

    WS1 <-->|gRPC| DS
    WS2 <-->|gRPC| DS
    WS3 <-->|gRPC| DS

    PIC -->|Publish Updates| RD
    DIC -->|Publish Updates| RD

    style KT fill:#ff9999
    style CH fill:#99ccff
    style RD fill:#ffcc99
    style DS fill:#99ff99
    style LB fill:#cc99ff
```

---

2. Real-Time Data Flow (Trade → Client)

```mermaid
sequenceDiagram
    participant Exchange
    participant Kafka
    participant CandleBuilder
    participant Redis
    participant WebSocket
    participant Client

    Exchange->>Kafka: Trade Event<br/>{BTCUSDT, price:50000, vol:0.5}

    Kafka->>CandleBuilder: Consume Trade

    Note over CandleBuilder: Update In-Memory<br/>OHLCV Candle

    alt Candle Still Open
        CandleBuilder->>CandleBuilder: Update High/Low/Close/Volume
    end

    alt Candle Closed (1 min elapsed)
        CandleBuilder->>Kafka: Publish Candle<br/>Topic: candles-1m
        CandleBuilder->>ClickHouse: Persist Candle
        CandleBuilder->>Redis: Publish Update<br/>Channel: BTCUSDT:1m

        Redis->>WebSocket: Fan Out Update

        WebSocket->>Client: Binary Protobuf Message<br/>MarketDataUpdate
    end

    Note over Client: Update Chart<br/>in Real-Time
```

---

3. Indicator Subscription Flow

```mermaid
sequenceDiagram
    participant Client
    participant WebSocket
    participant SubscriptionMgr
    participant DataService
    participant Redis
    participant ClickHouse

    Client->>WebSocket: Subscribe Request<br/>BTCUSDT:1m + RSI(14)

    WebSocket->>SubscriptionMgr: Register Subscription

    SubscriptionMgr->>Redis: Subscribe to Channel<br/>"BTCUSDT:1m:rsi-14"

    WebSocket->>DataService: GetMarketData<br/>(Historical Data)

    DataService->>Redis: Check Cache

    alt Cache Hit
        Redis-->>DataService: Cached Data
    else Cache Miss
        DataService->>ClickHouse: Query Candles
        ClickHouse-->>DataService: OHLCV Data
        DataService->>DataService: Compute RSI(14)
        DataService->>Redis: Cache Result
    end

    DataService-->>WebSocket: Historical Candles + RSI

    WebSocket->>Client: Send Historical Data<br/>(Replay 100 bars)

    WebSocket->>Client: Subscription Confirmed

    Note over Client,Redis: Real-time updates begin

    loop Every New Candle
        Redis->>WebSocket: Update via Pub/Sub
        WebSocket->>Client: MarketDataUpdate
    end

```

---

4. WebSocket Server Internal Architecture

```mermaid
graph TB
    subgraph "WebSocket Server Process"
        HTTP[HTTP Handler<br/>:8080/v1/stream]

        subgraph "Connection Management"
            UP[Upgrader<br/>HTTP → WebSocket]
            CP[Client Pool<br/>sync.Map]
        end

        subgraph "Client Goroutines"
            direction LR
            C1[Client 1]
            C2[Client 2]
            CN[Client N]

            subgraph "Per Client"
                RP[Read Pump<br/>Goroutine]
                WP[Write Pump<br/>Goroutine]
                SC[Send Channel<br/>buffered 256]
            end
        end

        subgraph "Subscription Management"
            SM[Subscription Manager]
            SB[Subscriptions Map<br/>symbol:tf → []clients]
        end

        subgraph "External Communication"
            GRPC[gRPC Client<br/>→ Data Service]
            RPUB[Redis Pub/Sub<br/>Receiver]
        end

        HTTP --> UP
        UP --> CP
        CP --> C1
        CP --> C2
        CP --> CN

        C1 --> RP
        C1 --> WP
        RP --> SC
        SC --> WP

        RP -->|Subscribe Request| SM
        SM --> SB
        SM -->|Subscribe Channel| RPUB

        RP -->|Get Data Request| GRPC
        GRPC -->|Response| SC

        RPUB -->|Update| SM
        SM -->|Fan Out| SC
    end

    EXT_DS[Data Service<br/>gRPC Server]
    EXT_RD[Redis<br/>Pub/Sub]

    GRPC <-->|gRPC| EXT_DS
    RPUB <-->|Pub/Sub| EXT_RD

    style HTTP fill:#ffcccc
    style CP fill:#ccffcc
    style SM fill:#ccccff
    style GRPC fill:#ffffcc
    style RPUB fill:#ffccff
```

---

6. Data Service Request Flow

```mermaid
graph LR
    subgraph "WebSocket Server"
        WS[Client Connection]
    end

    subgraph "Data Service"
        GRPC[gRPC Handler]

        subgraph "Cache Layer"
            RC[Redis Cache]
            CHK{Cache Hit?}
        end

        subgraph "Computation Layer"
            IC[Indicator Computer]

            subgraph "Indicator Types"
                PRSI[RSI Calculator]
                PMACD[MACD Calculator]
                PBB[Bollinger Calculator]
                PCUST[Pine Script Engine]
            end
        end

        subgraph "Storage Layer"
            CH[(ClickHouse)]
        end
    end

    WS -->|GetMarketData<br/>BTCUSDT:1m<br/>RSI(14)| GRPC

    GRPC --> CHK

    CHK -->|Hit| RC
    RC -->|Cached Data| GRPC

    CHK -->|Miss| CH
    CH -->|OHLCV Candles| IC

    IC --> PRSI
    PRSI -->|RSI Values| IC

    IC -->|Cache Write| RC
    IC -->|Result| GRPC

    GRPC -->|Response| WS

    style CHK fill:#ffff99
    style RC fill:#99ff99
    style IC fill:#ff99ff
    style CH fill:#9999ff
```

---

7. Subscription Manager Architecture

```mermaid
graph TB
    subgraph "Subscription Manager"
        SM[Manager Instance]

        subgraph "Subscription Storage"
            SMAP["subscriptions map<br/>key: 'BTCUSDT:1m'<br/>value: []*Subscription"]
        end

        subgraph "Redis Integration"
            direction LR
            RSB[Redis Subscribe]
            RUS[Redis Unsubscribe]
            RHD[Redis Handler]
        end

        subgraph "Fan-Out Engine"
            FO[Fan Out to Clients]

            subgraph "Client Channels"
                CH1[Client 1 Send Chan]
                CH2[Client 2 Send Chan]
                CHN[Client N Send Chan]
            end
        end

        subgraph "Lifecycle Management"
            GC[Garbage Collector<br/>Every 1 min]
            INACT[Remove Inactive<br/>>5 min idle]
        end
    end

    CLI1[Client 1<br/>Subscribe]
    CLI2[Client 2<br/>Subscribe]
    CLI3[Client 3<br/>Unsubscribe]

    RD[(Redis Pub/Sub)]

    CLI1 -->|Subscribe| SM
    CLI2 -->|Subscribe| SM
    CLI3 -->|Unsubscribe| SM

    SM --> SMAP

    SM -->|First Subscriber| RSB
    RSB -->|Subscribe| RD

    SM -->|Last Subscriber| RUS
    RUS -->|Unsubscribe| RD

    RD -->|Update Message| RHD
    RHD --> FO

    FO --> CH1
    FO --> CH2
    FO --> CHN

    GC -->|Check Activity| SMAP
    INACT -->|Cleanup| SMAP

    style SMAP fill:#ffcccc
    style RHD fill:#ccffcc
    style FO fill:#ccccff
    style GC fill:#ffffcc
```

---

8. Stream Processor State Management

```mermaid
graph TB
    subgraph "Stream Processor"
        KCS[Kafka Consumer]

        subgraph "Per-Symbol State"
            direction TB

            STATE["State Store<br/>(RocksDB/Flink)"]

            subgraph "Symbol: BTCUSDT"
                S1["Current Candle<br/>{open, high, low, close, vol}"]

                subgraph "Indicator States"
                    I1["RSI(14)<br/>{avg_gain, avg_loss, prices[14]}"]
                    I2["MACD(12,26,9)<br/>{ema12, ema26, signal}"]
                    I3["BB(20,2)<br/>{sma, std_dev}"]
                end
            end

            subgraph "Symbol: ETHUSDT"
                S2["Current Candle"]
                I4["RSI(14)"]
            end
        end

        subgraph "Output"
            KP[Kafka Producer]
            RDP[Redis Publisher]
            CHW[ClickHouse Writer]
        end

        subgraph "Checkpointing"
            CP[Checkpoint<br/>Every 60s]
            SNAP[State Snapshot]
        end
    end

    KAFKA[(Kafka<br/>trades topic)]
    REDIS[(Redis)]
    CH[(ClickHouse)]

    KAFKA -->|Trade Events| KCS

    KCS -->|Update State| STATE

    STATE --> S1
    S1 --> I1
    S1 --> I2
    S1 --> I3

    STATE --> S2
    S2 --> I4

    S1 -->|Candle Closed| KP
    I1 -->|New Value| RDP
    I2 -->|New Value| RDP

    KP -->|candles-1m| KAFKA
    RDP -->|Pub/Sub| REDIS
    S1 -->|Persist| CHW
    CHW --> CH

    STATE -->|Periodic| CP
    CP --> SNAP
    SNAP -.->|Recovery| STATE

    style STATE fill:#ffcccc
    style S1 fill:#ccffcc
    style I1 fill:#ccccff
    style CP fill:#ffffcc
```

9. Horizontal Scaling Architecture

```mermaid
graph TB
    subgraph "Load Balancers"
        LB1[Load Balancer<br/>WebSocket]
        LB2[Load Balancer<br/>gRPC]
    end

    subgraph "WebSocket Tier - Stateless"
        WS1[WS Server 1<br/>40K connections]
        WS2[WS Server 2<br/>40K connections]
        WS3[WS Server 3<br/>20K connections]
    end

    subgraph "Data Service Tier - Stateless"
        DS1[Data Service 1]
        DS2[Data Service 2]
        DS3[Data Service 3]
    end

    subgraph "Stream Processing Tier - Stateful"
        direction LR

        subgraph "Candle Builders"
            CB1[Builder 1<br/>Partition 0-5]
            CB2[Builder 2<br/>Partition 6-11]
            CB3[Builder 3<br/>Partition 12-15]
        end

        subgraph "Indicator Computers"
            IC1[Computer 1<br/>Partition 0-5]
            IC2[Computer 2<br/>Partition 6-11]
            IC3[Computer 3<br/>Partition 12-15]
        end
    end

    subgraph "Shared State"
        KAFKA[(Kafka<br/>16 Partitions)]
        REDIS[(Redis Cluster<br/>3 Master + 3 Replica)]
        CH[(ClickHouse<br/>Distributed)]
    end

    CLIENTS[100K Clients]

    CLIENTS --> LB1

    LB1 --> WS1
    LB1 --> WS2
    LB1 --> WS3

    WS1 --> LB2
    WS2 --> LB2
    WS3 --> LB2

    LB2 --> DS1
    LB2 --> DS2
    LB2 --> DS3

    DS1 --> REDIS
    DS1 --> CH
    DS2 --> REDIS
    DS2 --> CH
    DS3 --> REDIS
    DS3 --> CH

    KAFKA --> CB1
    KAFKA --> CB2
    KAFKA --> CB3

    KAFKA --> IC1
    KAFKA --> IC2
    KAFKA --> IC3

    CB1 --> KAFKA
    CB2 --> KAFKA
    CB3 --> KAFKA

    CB1 --> REDIS
    CB2 --> REDIS
    CB3 --> REDIS

    IC1 --> REDIS
    IC2 --> REDIS
    IC3 --> REDIS

    WS1 -.->|Pub/Sub| REDIS
    WS2 -.->|Pub/Sub| REDIS
    WS3 -.->|Pub/Sub| REDIS

    style LB1 fill:#ff9999
    style WS1 fill:#99ccff
    style DS1 fill:#99ff99
    style CB1 fill:#ffcc99
    style IC1 fill:#cc99ff
    style KAFKA fill:#ffcccc
    style REDIS fill:#ccffff
```
