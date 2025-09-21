-- RECOMMENDED APPROACH: Single Table for All Trade Data
-- This is more efficient and follows industry best practices
-- 1. SINGLE TRADE DATA TABLE(RECOMMENDED)
CREATE TABLE IF NOT EXISTS
    trade_data(
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
PARTITION BY
    toYYYYMMDD(engine_timestamp)
ORDER BY
    (symbol, engine_timestamp) SETTINGS index_granularity = 8192;

-- Index for faster price-based queries
ALTER TABLE trade_data ADD INDEX idx_price price TYPE minmax GRANULARITY 4;

-- 2. CANDLE GENERATION FROM TRADE DATA
-- 1-Minute Candles
CREATE TABLE IF NOT EXISTS
    candles_1m(
        symbol LowCardinality(String),
        candle_time DateTime,
        open Float64,
        high Float64,
        low Float64,
        close Float64,
        volume Float64,
        trade_count UInt64,
        vwap Float64, -- Volume Weighted Average Price
        created_at DateTime DEFAULT now()
    ) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY
    toYYYYMM(candle_time)
ORDER BY
    (symbol, candle_time);

-- 5-Minute Candles
CREATE TABLE IF NOT EXISTS
    candles_5m(
        symbol LowCardinality(String),
        candle_time DateTime,
        open Float64,
        high Float64,
        low Float64,
        close Float64,
        volume Float64,
        trade_count UInt64,
        vwap Float64,
        created_at DateTime DEFAULT now()
    ) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY
    toYYYYMM(candle_time)
ORDER BY
    (symbol, candle_time);

-- 4-Hours Candles
CREATE TABLE IF NOT EXISTS 
    candle_4h(
        symbol LowCardinality(String),
        candle_time DateTime,
        open Float64,
        high Float64,
        low Float64,
        close Float64,
        volume Float64,
        trade_count UInt64,
        vwap Float64,
        created_at DateTime DEFAULT now()
    ) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY
    toYYYYMMDD(candle_time)
ORDER BY
    (symbol, candle_time);

-- Daily Candles
CREATE TABLE IF NOT EXISTS
    candles_1d(
        symbol LowCardinality(String),
        candle_date Date,
        open Float64,
        high Float64,
        low Float64,
        close Float64,
        volume Float64,
        trade_count UInt64,
        vwap Float64,
        created_at DateTime DEFAULT now()
    ) ENGINE = ReplacingMergeTree(created_at)
PARTITION BY
    toYYYYMM(candle_date)
ORDER BY
     (symbol, candle_date);

-- 3. MATERIALIZED VIEWS FOR REAL-TIME CANDLE GENERATION
-- 1-Minute Candles(with 70-second delay to ensure completion)
CREATE MATERIALIZED VIEW candles_1m_mv TO candles_1m AS
SELECT
    symbol,
    toStartOfMinute(timestamp) as candle_time,
    argMin(price, timestamp) as open,
    max(price) as high,
    min(price) as low,
    argMax(price, timestamp) as close,
    sum(volume) as volume,
    count() as trade_count,
    sum(price * volume) / sum(volume) as vwap, -- Volume Weighted Average Price
    now() as created_at
FROM
    trade_data
WHERE
    timestamp <= now() - INTERVAL 70 SECOND
GROUP BY
    symbol,
    candle_time;

-- 5-Minute Candles(with 6-minute delay)
CREATE MATERIALIZED VIEW candles_5m_mv TO candles_5m AS
SELECT
    symbol,
    toDateTime(intDiv(toUnixTimestamp(timestamp), 300) * 300) as candle_time,
    argMin(price, timestamp) as open,
    max(price) as high,
    min(price) as low,
    argMax(price, timestamp) as close,
    sum(volume) as volume,
    count() as trade_count,
    sum(price * volume) / sum(volume) as vwap,
    now() as created_at
FROM
    trade_data
WHERE
    timestamp <= now() - INTERVAL 6 MINUTE
GROUP BY
    symbol,
    candle_time;


-- 4-Hours Candles(with 4-hour 5-minute delay)
CREATE MATERIALIZED VIEW candle_4h_mv TO candle_4h AS
SELECT 
    symbol,
    toDateTime(intDiv(toUnixTimestamp(timestamp), 14400) * 14400) as candle_time,
    argMin(price, timestamp) as open,
    max(price) as high,
    min(price) as low,
    argMax(price, timestamp) as close,
    sum(volume) as volume,
    count() as trade_count,
    sum(price * volume) / sum(volume) as vwap,
    now() as created_at
FROM
    trade_data
WHERE
    timestamp <= now() - INTERVAL 4 HOUR - INTERVAL 5 MINUTE
GROUP BY
    symbol,
    candle_time;


-- Daily Candles(previous day only)
CREATE MATERIALIZED VIEW candles_1d_mv TO candles_1d AS
SELECT
    symbol,
    toDate(timestamp) as candle_time,
    argMin(price, timestamp) as open,
    max(price) as high,
    min(price) as low,
    argMax(price, timestamp) as close,
    sum(volume) as volume,
    count() as trade_count,
    sum(price * volume) / sum(volume) as vwap,
    now() as created_at
FROM
    trade_data
WHERE
    toDate(timestamp) < today()
GROUP BY
    symbol,
    candle_time;

