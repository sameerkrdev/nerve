package clickhouse

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	pb "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/aggeration/v1"
)

// FetchCandles reads from the nerve.candles view which is pre-computed by
// ClickHouse materialized views from the trades table.
// tfSeconds matches the Timeframe enum value (e.g. 60 for 1m, 3600 for 1h).
func FetchCandles(ctx context.Context, conn driver.Conn, symbol string, tfSeconds int32, limit int64) ([]*pb.Candle, error) {
	rows, err := conn.Query(ctx, `
		SELECT open_time, o, h, l, c, v
		FROM candles
		WHERE symbol = ? AND timeframe_secs = ?
		ORDER BY open_time DESC
		LIMIT ?
	`, symbol, uint32(tfSeconds), limit)
	if err != nil {
		return nil, fmt.Errorf("clickhouse query: %w", err)
	}
	defer rows.Close()

	var candles []*pb.Candle
	for rows.Next() {
		var (
			openTime   int64
			o, h, l, c float64
			v          int64
		)
		if err := rows.Scan(&openTime, &o, &h, &l, &c, &v); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		candles = append(candles, &pb.Candle{
			OpenTime: openTime,
			O: o, H: h, L: l, C: c,
			V:        v,
			IsClosed: true,
		})
	}
	return candles, rows.Err()
}
