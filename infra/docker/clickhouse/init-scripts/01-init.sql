-- ================================
-- 1️⃣ CREATE DATABASE
-- ================================
CREATE DATABASE IF NOT EXISTS nerve;


-- ================================
-- 2️⃣ RAW TRADE DATA TABLE
-- ================================
CREATE TABLE IF NOT EXISTS nerve.trade_data (
    id UUID,
    client_timestamp DateTime64(3),
    engine_timestamp DateTime64(3),
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

ALTER TABLE nerve.trade_data ADD INDEX IF NOT EXISTS idx_price  price   TYPE minmax GRANULARITY 4;
ALTER TABLE nerve.trade_data ADD INDEX IF NOT EXISTS idx_userId user_id TYPE minmax GRANULARITY 4;
ALTER TABLE nerve.trade_data ADD INDEX IF NOT EXISTS idx_id     id      TYPE minmax GRANULARITY 4;


-- ================================
-- 3️⃣ CANDLE STATE TABLES (Using AggregatingMergeTree)
-- ================================
-- These tables store intermediate aggregation states that can be merged

-- ---- 1-Minute Candles State ----
CREATE TABLE IF NOT EXISTS nerve.candles_1m_state (
    symbol LowCardinality(String),
    candle_time DateTime,
    open_state AggregateFunction(argMin, Float64, DateTime64(3)),
    high_state AggregateFunction(max, Float64),
    low_state AggregateFunction(min, Float64),
    close_state AggregateFunction(argMax, Float64, DateTime64(3)),
    volume_state AggregateFunction(sum, Float64),
    trade_count_state AggregateFunction(count, UInt64),
    volume_weighted_sum_state AggregateFunction(sum, Float64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(candle_time)
ORDER BY (symbol, candle_time);

-- ---- 5-Minute Candles State ----
CREATE TABLE IF NOT EXISTS nerve.candles_5m_state (
    symbol LowCardinality(String),
    candle_time DateTime,
    open_state AggregateFunction(argMin, Float64, DateTime64(3)),
    high_state AggregateFunction(max, Float64),
    low_state AggregateFunction(min, Float64),
    close_state AggregateFunction(argMax, Float64, DateTime64(3)),
    volume_state AggregateFunction(sum, Float64),
    trade_count_state AggregateFunction(count, UInt64),
    volume_weighted_sum_state AggregateFunction(sum, Float64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(candle_time)
ORDER BY (symbol, candle_time);

-- ---- 4-Hour Candles State ----
CREATE TABLE IF NOT EXISTS nerve.candles_4h_state (
    symbol LowCardinality(String),
    candle_time DateTime,
    open_state AggregateFunction(argMin, Float64, DateTime64(3)),
    high_state AggregateFunction(max, Float64),
    low_state AggregateFunction(min, Float64),
    close_state AggregateFunction(argMax, Float64, DateTime64(3)),
    volume_state AggregateFunction(sum, Float64),
    trade_count_state AggregateFunction(count, UInt64),
    volume_weighted_sum_state AggregateFunction(sum, Float64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMMDD(candle_time)
ORDER BY (symbol, candle_time);

-- ---- Daily Candles State ----
CREATE TABLE IF NOT EXISTS nerve.candles_1d_state (
    symbol LowCardinality(String),
    candle_date Date,
    open_state AggregateFunction(argMin, Float64, DateTime64(3)),
    high_state AggregateFunction(max, Float64),
    low_state AggregateFunction(min, Float64),
    close_state AggregateFunction(argMax, Float64, DateTime64(3)),
    volume_state AggregateFunction(sum, Float64),
    trade_count_state AggregateFunction(count, UInt64),
    volume_weighted_sum_state AggregateFunction(sum, Float64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(candle_date)
ORDER BY (symbol, candle_date);


-- ================================
-- 4️⃣ MATERIALIZED VIEWS (Writing to State Tables)
-- ================================

-- ---- 1-Minute Candles MV ----
CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_1m_mv
TO nerve.candles_1m_state AS
SELECT
    symbol,
    toStartOfMinute(engine_timestamp) AS candle_time,
    argMinState(price, engine_timestamp) AS open_state,
    maxState(price) AS high_state,
    minState(price) AS low_state,
    argMaxState(price, engine_timestamp) AS close_state,
    sumState(volume) AS volume_state,
    countState() AS trade_count_state,
    sumState(price * volume) AS volume_weighted_sum_state
FROM nerve.trade_data
GROUP BY symbol, candle_time;

-- ---- 5-Minute Candles MV ----
CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_5m_mv
TO nerve.candles_5m_state AS
SELECT
    symbol,
    toDateTime(intDiv(toUnixTimestamp(engine_timestamp), 300) * 300) AS candle_time,
    argMinState(price, engine_timestamp) AS open_state,
    maxState(price) AS high_state,
    minState(price) AS low_state,
    argMaxState(price, engine_timestamp) AS close_state,
    sumState(volume) AS volume_state,
    countState() AS trade_count_state,
    sumState(price * volume) AS volume_weighted_sum_state
FROM nerve.trade_data
GROUP BY symbol, candle_time;

-- ---- 4-Hour Candles MV ----
CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_4h_mv
TO nerve.candles_4h_state AS
SELECT
    symbol,
    toDateTime(intDiv(toUnixTimestamp(engine_timestamp), 14400) * 14400) AS candle_time,
    argMinState(price, engine_timestamp) AS open_state,
    maxState(price) AS high_state,
    minState(price) AS low_state,
    argMaxState(price, engine_timestamp) AS close_state,
    sumState(volume) AS volume_state,
    countState() AS trade_count_state,
    sumState(price * volume) AS volume_weighted_sum_state
FROM nerve.trade_data
GROUP BY symbol, candle_time;

-- ---- Daily Candles MV ----
CREATE MATERIALIZED VIEW IF NOT EXISTS nerve.candles_1d_mv
TO nerve.candles_1d_state AS
SELECT
    symbol,
    toDate(engine_timestamp) AS candle_date,
    argMinState(price, engine_timestamp) AS open_state,
    maxState(price) AS high_state,
    minState(price) AS low_state,
    argMaxState(price, engine_timestamp) AS close_state,
    sumState(volume) AS volume_state,
    countState() AS trade_count_state,
    sumState(price * volume) AS volume_weighted_sum_state
FROM nerve.trade_data
GROUP BY symbol, candle_date;


-- ================================
-- 5️⃣ QUERY VIEWS (For Reading Final Candles)
-- ================================
-- These views merge the states and present final OHLCV data

-- ---- 1-Minute Candles Query View ----
CREATE OR REPLACE VIEW nerve.candles_1m AS
SELECT
    symbol,
    candle_time,
    argMinMerge(open_state) AS open,
    maxMerge(high_state) AS high,
    minMerge(low_state) AS low,
    argMaxMerge(close_state) AS close,
    sumMerge(volume_state) AS volume,
    countMerge(trade_count_state) AS trade_count,
    sumMerge(volume_weighted_sum_state) AS volume_weighted_sum,
    if(sumMerge(volume_state) > 0, 
       sumMerge(volume_weighted_sum_state) / sumMerge(volume_state), 
       0) AS vwap
FROM nerve.candles_1m_state
GROUP BY symbol, candle_time
ORDER BY symbol, candle_time;

-- ---- 5-Minute Candles Query View ----
CREATE OR REPLACE VIEW nerve.candles_5m AS
SELECT
    symbol,
    candle_time,
    argMinMerge(open_state) AS open,
    maxMerge(high_state) AS high,
    minMerge(low_state) AS low,
    argMaxMerge(close_state) AS close,
    sumMerge(volume_state) AS volume,
    countMerge(trade_count_state) AS trade_count,
    sumMerge(volume_weighted_sum_state) AS volume_weighted_sum,
    if(sumMerge(volume_state) > 0, 
       sumMerge(volume_weighted_sum_state) / sumMerge(volume_state), 
       0) AS vwap
FROM nerve.candles_5m_state
GROUP BY symbol, candle_time
ORDER BY symbol, candle_time;

-- ---- 4-Hour Candles Query View ----
CREATE OR REPLACE VIEW nerve.candles_4h AS
SELECT
    symbol,
    candle_time,
    argMinMerge(open_state) AS open,
    maxMerge(high_state) AS high,
    minMerge(low_state) AS low,
    argMaxMerge(close_state) AS close,
    sumMerge(volume_state) AS volume,
    countMerge(trade_count_state) AS trade_count,
    sumMerge(volume_weighted_sum_state) AS volume_weighted_sum,
    if(sumMerge(volume_state) > 0, 
       sumMerge(volume_weighted_sum_state) / sumMerge(volume_state), 
       0) AS vwap
FROM nerve.candles_4h_state
GROUP BY symbol, candle_time
ORDER BY symbol, candle_time;

-- ---- Daily Candles Query View ----
CREATE OR REPLACE VIEW nerve.candles_1d AS
SELECT
    symbol,
    candle_date,
    argMinMerge(open_state) AS open,
    maxMerge(high_state) AS high,
    minMerge(low_state) AS low,
    argMaxMerge(close_state) AS close,
    sumMerge(volume_state) AS volume,
    countMerge(trade_count_state) AS trade_count,
    sumMerge(volume_weighted_sum_state) AS volume_weighted_sum,
    if(sumMerge(volume_state) > 0, 
       sumMerge(volume_weighted_sum_state) / sumMerge(volume_state), 
       0) AS vwap
FROM nerve.candles_1d_state
GROUP BY symbol, candle_date
ORDER BY symbol, candle_date;


-- ================================
-- 6️⃣ TESTING QUERIES
-- ================================

-- Query to verify no duplicates (should show 1 row per symbol+candle_time):
-- SELECT symbol, candle_time, count(*) as cnt 
-- FROM nerve.candles_1m 
-- GROUP BY symbol, candle_time 
-- HAVING cnt > 1;

-- Check record counts:
-- SELECT 'Raw trades' as table_name, count() as count FROM nerve.trade_data
-- UNION ALL
-- SELECT '1m candles', count() FROM nerve.candles_1m
-- UNION ALL  
-- SELECT '5m candles', count() FROM nerve.candles_5m
-- UNION ALL
-- SELECT '4h candles', count() FROM nerve.candles_4h
-- UNION ALL
-- SELECT '1d candles', count() FROM nerve.candles_1d;