-- Docker local dev only.
-- ClickHouse Cloud: schema is created by trade-ingestor-service EnsureSchema() on startup.
-- Set CLICKHOUSE_DATABASE env var to match the database used here.

-- ================================
-- 1. DATABASE
-- ================================
CREATE DATABASE IF NOT EXISTS nerve;


-- ================================
-- 2. RAW TRADES TABLE
-- Mirrors TradeEvent proto fields.
-- price stored as Float64 (raw int64 cents / 100 on insert).
-- ================================
CREATE TABLE IF NOT EXISTS nerve.trades (
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
ORDER BY (symbol, timestamp, trade_id)
SETTINGS index_granularity = 8192;


-- ================================
-- 3. UNIFIED CANDLES STATE TABLE
-- Single AggregatingMergeTree for all timeframes.
-- timeframe_secs matches Timeframe enum values in proto (1m=60, 1h=3600, …).
-- ================================
CREATE TABLE IF NOT EXISTS nerve.candles_state (
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
ORDER BY (symbol, timeframe_secs, candle_time);


-- ================================
-- 4. MATERIALIZED VIEWS
-- One MV per timeframe, all writing into candles_state.
-- ================================

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_1m_mv
TO nerve.candles_state AS
SELECT
    symbol,
    60 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 60) * 60, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_3m_mv
TO nerve.candles_state AS
SELECT
    symbol,
    180 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 180) * 180, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_5m_mv
TO nerve.candles_state AS
SELECT
    symbol,
    300 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 300) * 300, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_15m_mv
TO nerve.candles_state AS
SELECT
    symbol,
    900 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 900) * 900, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_30m_mv
TO nerve.candles_state AS
SELECT
    symbol,
    1800 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 1800) * 1800, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_1h_mv
TO nerve.candles_state AS
SELECT
    symbol,
    3600 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 3600) * 3600, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_2h_mv
TO nerve.candles_state AS
SELECT
    symbol,
    7200 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 7200) * 7200, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_4h_mv
TO nerve.candles_state AS
SELECT
    symbol,
    14400 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 14400) * 14400, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_6h_mv
TO nerve.candles_state AS
SELECT
    symbol,
    21600 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 21600) * 21600, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_12h_mv
TO nerve.candles_state AS
SELECT
    symbol,
    43200 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 43200) * 43200, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_1d_mv
TO nerve.candles_state AS
SELECT
    symbol,
    86400 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 86400) * 86400, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_1w_mv
TO nerve.candles_state AS
SELECT
    symbol,
    604800 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 604800) * 604800, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;

CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_1mon_mv
TO nerve.candles_state AS
SELECT
    symbol,
    2592000 AS timeframe_secs,
    toDateTime(intDiv(toUnixTimestamp64Second(timestamp), 2592000) * 2592000, 'UTC') AS candle_time,
    argMinState(price, timestamp) AS open_state,
    maxState(price)               AS high_state,
    minState(price)               AS low_state,
    argMaxState(price, timestamp) AS close_state,
    sumState(quantity)            AS volume_state
FROM nerve.trades
GROUP BY symbol, timeframe_secs, candle_time;


-- ================================
-- 5. CANDLES QUERY VIEW
-- Merges aggregate states. Filter by symbol + timeframe_secs.
-- ================================
CREATE OR REPLACE VIEW nerve.candles AS
SELECT
    symbol,
    timeframe_secs,
    toInt64(toUnixTimestamp(candle_time)) AS open_time,
    argMinMerge(open_state)              AS o,
    maxMerge(high_state)                 AS h,
    minMerge(low_state)                  AS l,
    argMaxMerge(close_state)             AS c,
    sumMerge(volume_state)               AS v
FROM nerve.candles_state
GROUP BY symbol, timeframe_secs, candle_time;
