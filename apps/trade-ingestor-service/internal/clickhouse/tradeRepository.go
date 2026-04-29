package clickhouse

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/IBM/sarama"
	pbEngine "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/engine"
)

type BatchItem struct {
	msg   *sarama.ConsumerMessage
	trade *pbEngine.TradeEvent
}

type TradeBatcher struct {
	ch     chan BatchItem
	buffer []BatchItem

	maxSize   int
	flushTime time.Duration

	kafkaClient      *sarama.ConsumerGroupSession
	clickhouseClient *driver.Conn
}

func Insert() {

}
