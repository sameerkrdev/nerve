package internal

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
)

var (
	producer sarama.SyncProducer
	once     sync.Once
	initErr  error
)

func GetProducer(brokers []string) (sarama.SyncProducer, error) {
	once.Do(func() {
		config := sarama.NewConfig()
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Retry.Max = 5
		config.Producer.Return.Successes = true
		config.Producer.Idempotent = true
		config.Net.MaxOpenRequests = 1

		producer, initErr = sarama.NewSyncProducer(brokers, config)
	})

	return producer, initErr
}

type KafkaProducerWorker struct {
	producer       sarama.SyncProducer
	Symbol         string
	dirPath        string
	batchSize      int
	emitTimeMM     int
	wal            *SymbolWAL
	checkpointFile *os.File
	ctx            context.Context
}

func NewKafkaProducerWorker(symbol string, dirPath string, wal *SymbolWAL, batchSize int, emitTime int) (*KafkaProducerWorker, error) {
	brokers := []string{"localhost:19092", "localhost:19093", "localhost:19094"}
	producer, err := GetProducer(brokers)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(filepath.Join(dirPath, symbol, "checkpoint.meta"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return &KafkaProducerWorker{
		producer:       producer,
		wal:            wal,
		batchSize:      batchSize,
		emitTimeMM:     emitTime,
		dirPath:        dirPath,
		Symbol:         symbol,
		checkpointFile: file,
		ctx:            context.Background(),
	}, nil
}

func (kpw *KafkaProducerWorker) Run() {
	ticker := time.NewTicker(time.Millisecond * time.Duration(kpw.emitTimeMM))
	defer ticker.Stop()

	for {
		select {
		case <-kpw.ctx.Done():
			return

		case <-ticker.C:
			// IMPORTANT: this is blocking
			kpw.processBatch()
		}
	}
}

func (kpw *KafkaProducerWorker) processBatch() {
	startOffset := kpw.loadCheckpoint()

	events, _ := kpw.wal.ReadFromTo(
		startOffset+1,
		startOffset+uint64(kpw.batchSize),
	)

	lastOffset := uint64(0)
	if len(events) > 0 {
		lastOffset = events[len(events)-1].GetSequenceNumber()
	}

	if len(events) == 0 {
		return
	}

	if err := kpw.emitToKafka(events); err != nil {
		// DO NOT checkpoint on failure
		log.Println("kafka emit failed:", err)
		return
	}

	// Kafka ACK succeeded → safe to checkpoint
	if err := kpw.saveCheckpoint(lastOffset); err != nil {
		log.Println("checkpoint save failed:", err)
	}
}

func (kpw *KafkaProducerWorker) emitToKafka(events []*common.WAL_Entry) error {
	if len(events) == 0 {
		return nil
	}
	msgs := make([]*sarama.ProducerMessage, 0, kpw.batchSize)

	for _, event := range events {
		msg := &sarama.ProducerMessage{
			Topic: "engine-events",
			Key:   sarama.StringEncoder(kpw.Symbol),
			Value: sarama.ByteEncoder(event.GetData()),
			Headers: []sarama.RecordHeader{
				{
					Key:   []byte("sequence"),
					Value: []byte(strconv.FormatUint(event.GetSequenceNumber(), 10)),
				},
			},
		}
		msgs = append(msgs, msg)
	}

	return kpw.producer.SendMessages(msgs)
}

func (kpw *KafkaProducerWorker) loadCheckpoint() uint64 {
	// Always read from beginning
	if _, err := kpw.checkpointFile.Seek(0, 0); err != nil {
		log.Println("checkpoint seek failed:", err)
		return 0
	}

	data, err := io.ReadAll(kpw.checkpointFile)
	if err != nil {
		log.Println("checkpoint read failed:", err)
		return 0
	}

	if len(data) == 0 {
		return 0 // first run
	}

	offset, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		log.Println("invalid checkpoint value", err)
		return 0
	}

	return offset
}

func (kpw *KafkaProducerWorker) saveCheckpoint(offset uint64) error {
	data := []byte(strconv.FormatUint(offset, 10))

	if _, err := kpw.checkpointFile.Seek(0, 0); err != nil {
		return err
	}

	if err := kpw.checkpointFile.Truncate(0); err != nil {
		return err
	}

	if _, err := kpw.checkpointFile.Write(data); err != nil {
		return err
	}

	return kpw.checkpointFile.Sync()
}
