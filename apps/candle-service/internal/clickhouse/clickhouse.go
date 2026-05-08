package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	ClickhouseClient driver.Conn
	once             sync.Once
	initErr          error
)

func NewClickhouseClient(ctx context.Context) (driver.Conn, error) {
	once.Do(func() {
		var conn driver.Conn

		conn, initErr = clickhouse.Open(&clickhouse.Options{
			Addr:     []string{os.Getenv("CLICKHOUSE_ADDR")},
			Protocol: clickhouse.Native,
			TLS: &tls.Config{
				InsecureSkipVerify: true,
			},
			Auth: clickhouse.Auth{
				Username: os.Getenv("CLICKHOUSE_USER"),
				Password: os.Getenv("CLICKHOUSE_PASSWORD"),
			},
		})

		if initErr != nil {
			return
		}

		if err := conn.Ping(ctx); err != nil {
			if exception, ok := err.(*clickhouse.Exception); ok {
				fmt.Printf("Exception [%d] %s\n%s\n", exception.Code, exception.Message, exception.StackTrace)
			}
			initErr = err
			return
		}

		ClickhouseClient = conn
	})
	return ClickhouseClient, initErr
}

func InsertRawTrade() {

}
