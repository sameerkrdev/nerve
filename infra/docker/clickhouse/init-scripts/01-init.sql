-- ================================
-- 1️⃣ CREATE DATABASE
-- ================================
-- Create the database if it does not exist
CREATE DATABASE IF NOT EXISTS nerve;


-- ================================
-- 2️⃣ RAW TRADE DATA TABLE
-- ================================
-- Stores all raw trade events (tick-by-tick trades)
-- Partitioned by trade day for fast pruning
-- Ordered by (symbol, engine_timestamp) for efficient range queries
CREATE TABLE IF NOT EXISTS nerve.trade_data (
    id UUID,
    client_timestamp DateTime64(3), -- Time from client
    engine_timestamp DateTime64(3), -- Time when trade was processed (use this for aggregation)
    symbol LowCardinality(String),
    price Float64,
    volume Float64,
    side Enum('buy' = 1, 'sell' = 0),
    user_id String,
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(engine_timestamp)
ORDER BY (symbol, engine_timestamp)
SETTINGS index_granularity = 8192;

-- Optional indexes for faster point/range lookups
ALTER TABLE nerve.trade_data ADD INDEX IF NOT EXISTS idx_price  price   TYPE minmax GRANULARITY 4;
ALTER TABLE nerve.trade_data ADD INDEX IF NOT EXISTS idx_userId user_id TYPE minmax GRANULARITY 4;
ALTER TABLE nerve.trade_data ADD INDEX IF NOT EXISTS idx_id     id      TYPE minmax GRANULARITY 4;


-- ================================
-- 3️⃣ CANDLE (OHLCV) TABLES
-- ================================
-- These tables store aggregated candles for different timeframes.
-- Using ReplacingMergeTree to allow updates if needed.

-- ---- 1-Minute Candles ----
CREATE TABLE IF NOT EXISTS nerve.candles_1m (
    symbol LowCardinality(String),
    candle_time DateTime,          -- Start of the 1-minute interval
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume Float64,
    trade_count UInt64,
    volume_weighted_sum Float64,   -- sum(price * volume) - used to calculate VWAP
    vwap Float64,                  -- Volume Weighted Average Price (calculated)
    created_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toYYYYMM(candle_time)
ORDER BY (symbol, candle_time);

-- ---- 5-Minute Candles ----
CREATE TABLE IF NOT EXISTS nerve.candles_5m (
    symbol LowCardinality(String),
    candle_time DateTime,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume Float64,
    trade_count UInt64,
    volume_weighted_sum Float64,   -- sum(price * volume) - used to calculate VWAP
    vwap Float64,                  -- Volume Weighted Average Price (calculated)
    created_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toYYYYMM(candle_time)
ORDER BY (symbol, candle_time);

-- ---- 4-Hour Candles ----
CREATE TABLE IF NOT EXISTS nerve.candles_4h (
    symbol LowCardinality(String),
    candle_time DateTime,
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume Float64,
    trade_count UInt64,
    volume_weighted_sum Float64,   -- sum(price * volume) - used to calculate VWAP
    vwap Float64,                  -- Volume Weighted Average Price (calculated)
    created_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toYYYYMMDD(candle_time)
ORDER BY (symbol, candle_time);

-- ---- Daily Candles ----
CREATE TABLE IF NOT EXISTS nerve.candles_1d (
    symbol LowCardinality(String),
    candle_date Date,               -- Start of the day
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume Float64,
    trade_count UInt64,
    volume_weighted_sum Float64,   -- sum(price * volume) - used to calculate VWAP
    vwap Float64,                  -- Volume Weighted Average Price (calculated)
    created_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toYYYYMM(candle_date)
ORDER BY (symbol, candle_date);


-- ================================
-- 4️⃣ MATERIALIZED VIEWS
-- ================================
-- These views automatically aggregate data from trade_data
-- and insert OHLCV candles into the respective tables.

-- ---- 1-Minute Candles ----
-- For real-time processing without delay for immediate feedback
CREATE MATERIALIZED VIEW nerve.candles_1m_mv
TO nerve.candles_1m AS
SELECT
    symbol,
    toStartOfMinute(engine_timestamp) AS candle_time,
    argMin(price, engine_timestamp) AS open,
    max(price) AS high,
    min(price) AS low,
    argMax(price, engine_timestamp) AS close,
    sum(t.volume) AS volume,
    count() AS trade_count,
    sum(price * t.volume) AS volume_weighted_sum,
    if(sum(t.volume) > 0, sum(price * t.volume) / sum(t.volume), 0) AS vwap,
    now() AS created_at
FROM nerve.trade_data t
GROUP BY symbol, candle_time;

-- ---- 5-Minute Candles ----
-- Add 6-minute delay for production to ensure completeness
CREATE MATERIALIZED VIEW nerve.candles_5m_mv
TO nerve.candles_5m AS
SELECT
    symbol,
    toDateTime(intDiv(toUnixTimestamp(engine_timestamp), 300) * 300) AS candle_time,
    argMin(price, engine_timestamp) AS open,
    max(price) AS high,
    min(price) AS low,
    argMax(price, engine_timestamp) AS close,
    sum(t.volume) AS volume,
    count() AS trade_count,
    sum(price * t.volume) AS volume_weighted_sum,
    if(sum(t.volume) > 0, sum(price * t.volume) / sum(t.volume), 0) AS vwap,
    now() AS created_at
FROM nerve.trade_data t
-- Uncomment next line for production to add delay:
-- WHERE engine_timestamp <= now() - INTERVAL 6 MINUTE
GROUP BY symbol, candle_time;

-- ---- 4-Hour Candles ----
-- Add delay for production to ensure completeness
CREATE MATERIALIZED VIEW nerve.candles_4h_mv
TO nerve.candles_4h AS
SELECT
    symbol,
    toDateTime(intDiv(toUnixTimestamp(engine_timestamp), 14400) * 14400) AS candle_time,
    argMin(price, engine_timestamp) AS open,
    max(price) AS high,
    min(price) AS low,
    argMax(price, engine_timestamp) AS close,
    sum(t.volume) AS volume,
    count() AS trade_count,
    sum(price * t.volume) AS volume_weighted_sum,
    if(sum(t.volume) > 0, sum(price * t.volume) / sum(t.volume), 0) AS vwap,
    now() AS created_at
FROM nerve.trade_data t
-- Uncomment next line for production to add delay:
-- WHERE engine_timestamp <= now() - INTERVAL 4 HOUR - INTERVAL 5 MINUTE
GROUP BY symbol, candle_time;

-- ---- Daily Candles ----
-- Process all days including today for testing
CREATE MATERIALIZED VIEW nerve.candles_1d_mv
TO nerve.candles_1d AS
SELECT
    symbol,
    toDate(engine_timestamp) AS candle_date,
    argMin(price, engine_timestamp) AS open,
    max(price) AS high,
    min(price) AS low,
    argMax(price, engine_timestamp) AS close,
    sum(t.volume) AS volume,
    count() AS trade_count,
    sum(price * t.volume) AS volume_weighted_sum,
    if(sum(t.volume) > 0, sum(price * t.volume) / sum(t.volume), 0) AS vwap,
    now() AS created_at
FROM nerve.trade_data t
-- Uncomment next line for production to only process previous days:
-- WHERE toDate(engine_timestamp) < today()
GROUP BY symbol, candle_date;


-- ================================
-- 5️⃣ USEFUL QUERIES FOR TESTING
-- ================================

-- Insert sample data for testing (remove in production)
-- INSERT INTO nerve.trade_data VALUES
--     (generateUUIDv4(), now(), now(), 'BTCUSD', 45000.0, 0.1, 'buy', 'test_user1', now()),
--     (generateUUIDv4(), now() + INTERVAL 10 SECOND, now() + INTERVAL 10 SECOND, 'BTCUSD', 45100.0, 0.2, 'sell', 'test_user2', now()),
--     (generateUUIDv4(), now() + INTERVAL 20 SECOND, now() + INTERVAL 20 SECOND, 'BTCUSD', 45200.0, 0.15, 'buy', 'test_user3', now()),
--     (generateUUIDv4(), now() + INTERVAL 70 SECOND, now() + INTERVAL 70 SECOND, 'BTCUSD', 45050.0, 0.3, 'sell', 'test_user4', now());

-- Query to check if everything is working:
-- SELECT 'Raw trades:' as table_name, count() as record_count FROM nerve.trade_data
-- UNION ALL
-- SELECT '1m candles:' as table_name, count() as record_count FROM nerve.candles_1m
-- UNION ALL  
-- SELECT '5m candles:' as table_name, count() as record_count FROM nerve.candles_5m
-- UNION ALL
-- SELECT '4h candles:' as table_name, count() as record_count FROM nerve.candles_4h
-- UNION ALL
-- SELECT '1d candles:' as table_name, count() as record_count FROM nerve.candles_1d;