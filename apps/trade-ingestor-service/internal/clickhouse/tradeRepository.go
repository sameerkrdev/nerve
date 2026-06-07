package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/IBM/sarama"
	pbEngine "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

// EnsureSchema creates all required tables, materialized views, and the candles
// query view. Safe to call on every startup (CREATE IF NOT EXISTS / OR REPLACE).
// Owns schema for both trade-ingestor-service (trades) and candle-service L3 (candles_state, candles view).
func EnsureSchema(ctx context.Context, conn driver.Conn) error {
	stmts := []string{
		// raw trades
		`CREATE TABLE IF NOT EXISTS trades (
			trade_id       String,
			symbol         LowCardinality(String),
			trade_sequence UInt64,
			price          Float64,
			quantity       Int64,
			buyer_id       String,
			seller_id      String,
			buy_order_id   String,
			sell_order_id  String,
			is_buyer_maker Bool,
			timestamp      DateTime64(9, 'UTC')
		) ENGINE = MergeTree()
		PARTITION BY toYYYYMMDD(timestamp)
		ORDER BY (symbol, timestamp, trade_id)`,

		// single state table for all timeframes
		`CREATE TABLE IF NOT EXISTS candles_state (
			symbol         LowCardinality(String),
			timeframe_secs UInt32,
			candle_time    DateTime('UTC'),
			open_state     AggregateFunction(argMin, Float64, DateTime64(9)),
			high_state     AggregateFunction(max, Float64),
			low_state      AggregateFunction(min, Float64),
			close_state    AggregateFunction(argMax, Float64, DateTime64(9)),
			volume_state   AggregateFunction(sum, Int64)
		) ENGINE = AggregatingMergeTree()
		PARTITION BY toYYYYMM(candle_time)
		ORDER BY (symbol, timeframe_secs, candle_time)`,

		candleMV("candles_1m_mv", 60),
		candleMV("candles_3m_mv", 180),
		candleMV("candles_5m_mv", 300),
		candleMV("candles_15m_mv", 900),
		candleMV("candles_30m_mv", 1800),
		candleMV("candles_1h_mv", 3600),
		candleMV("candles_2h_mv", 7200),
		candleMV("candles_4h_mv", 14400),
		candleMV("candles_6h_mv", 21600),
		candleMV("candles_12h_mv", 43200),
		candleMV("candles_1d_mv", 86400),
		candleMV("candles_1w_mv", 604800),
		candleMV("candles_1mon_mv", 2592000),

		// query view — merges states on read
		`CREATE OR REPLACE VIEW candles AS
		SELECT
			symbol,
			timeframe_secs,
			toInt64(toUnixTimestamp(candle_time)) AS open_time,
			argMinMerge(open_state)              AS o,
			maxMerge(high_state)                 AS h,
			minMerge(low_state)                  AS l,
			argMaxMerge(close_state)             AS c,
			sumMerge(volume_state)               AS v
		FROM candles_state
		GROUP BY symbol, timeframe_secs, candle_time`,
	}

	for _, stmt := range stmts {
		if err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("schema exec: %w", err)
		}
	}
	return nil
}

func candleMV(name string, tfSecs int) string {
	return fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s
		TO candles_state AS
		SELECT
			symbol,
			%d AS timeframe_secs,
			toDateTime(intDiv(toUnixTimestamp64Second(timestamp), %d) * %d, 'UTC') AS candle_time,
			argMinState(price, timestamp) AS open_state,
			maxState(price)               AS high_state,
			minState(price)               AS low_state,
			argMaxState(price, timestamp) AS close_state,
			sumState(quantity)            AS volume_state
		FROM trades
		GROUP BY symbol, timeframe_secs, candle_time`,
		name, tfSecs, tfSecs, tfSecs)
}

type BatchItem struct {
	msg   *sarama.ConsumerMessage
	trade *pbEngine.TradeEvent
}

type TradeBatcher struct {
	ch      chan BatchItem
	buffer  []BatchItem

	maxSize  int
	flushDur time.Duration

	conn driver.Conn

	mu      sync.Mutex
	session sarama.ConsumerGroupSession
}

func NewTradeBatcher(conn driver.Conn, maxSize int, flushDur time.Duration) *TradeBatcher {
	return &TradeBatcher{
		ch:       make(chan BatchItem, maxSize*2),
		buffer:   make([]BatchItem, 0, maxSize),
		maxSize:  maxSize,
		flushDur: flushDur,
		conn:     conn,
	}
}

func (b *TradeBatcher) SetSession(session sarama.ConsumerGroupSession) {
	b.mu.Lock()
	b.session = session
	b.mu.Unlock()
}

func (b *TradeBatcher) Insert(msg *sarama.ConsumerMessage, trade *pbEngine.TradeEvent) {
	b.ch <- BatchItem{msg: msg, trade: trade}
}

func (b *TradeBatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(b.flushDur)
	defer ticker.Stop()

	for {
		select {
		case item := <-b.ch:
			b.buffer = append(b.buffer, item)
			if len(b.buffer) >= b.maxSize {
				b.flush(ctx)
			}
		case <-ticker.C:
			if len(b.buffer) > 0 {
				b.flush(ctx)
			}
		case <-ctx.Done():
			for {
				select {
				case item := <-b.ch:
					b.buffer = append(b.buffer, item)
				default:
					goto drain
				}
			}
		drain:
			if len(b.buffer) > 0 {
				b.flush(context.Background())
			}
			return
		}
	}
}

func (b *TradeBatcher) flush(ctx context.Context) {
	if err := InsertTrades(ctx, b.conn, b.buffer); err != nil {
		slog.Error("clickhouse flush failed — not marking messages", "error", err, "count", len(b.buffer))
		b.buffer = b.buffer[:0]
		return
	}

	b.mu.Lock()
	session := b.session
	b.mu.Unlock()

	if session != nil {
		for _, item := range b.buffer {
			session.MarkMessage(item.msg, "")
		}
	}
	b.buffer = b.buffer[:0]
}

// InsertTrades batch-inserts into nerve.trades (connected to nerve db, so just "trades").
// price is stored as Float64: raw int64 cents divided by 100.
// Schema + materialized views are managed by infra/docker/clickhouse/init-scripts/01-init.sql.
func InsertTrades(ctx context.Context, conn driver.Conn, batch []BatchItem) error {
	b, err := conn.PrepareBatch(ctx, "INSERT INTO trades")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, item := range batch {
		t := item.trade
		var ts time.Time
		if t.Timestamp != nil {
			ts = t.Timestamp.AsTime()
		}
		if err := b.Append(
			t.TradeId,
			t.Symbol,
			t.TradeSequence,
			float64(t.Price)/100.0,
			t.Quantity,
			t.BuyerId,
			t.SellerId,
			t.BuyOrderId,
			t.SellOrderId,
			t.IsBuyerMaker,
			ts,
		); err != nil {
			return fmt.Errorf("batch append: %w", err)
		}
	}

	return b.Send()
}
