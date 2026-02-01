package internal

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	pbTypes "github.com/sameerkrdev/nerve/packages/proto-defs/go/generated/common"
	"google.golang.org/protobuf/proto"
)

type SymbolWAL struct {
	dirPath string
	symbol  string

	bufferWriter *bufio.Writer

	nextOffset uint64
	// commitOffset uint64

	currentSegmentFile  *os.File
	maxFileSize         int64
	currentSegmentIndex int

	shouldFsync    bool
	syncIntervalMM int

	syncTimer *time.Timer

	ctx    context.Context
	cancel context.CancelFunc

	mu sync.Mutex
}

func OpenWAL(dir string, symbol string, maxFileSize int64, enableFsync bool, syncIntervalMM int) (*SymbolWAL, error) {
	dirPath := filepath.Join(dir, symbol)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var lastSegmentIndex int

	if len(entries) > 0 {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			indexStr, exists := strings.CutSuffix(name, ".log")
			if !exists {
				continue
			}

			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, err
			}

			if index > lastSegmentIndex {
				lastSegmentIndex = index
			}
		}
	} else {
		filePath := filepath.Join(dirPath, fmt.Sprintf("%d.log", 0))
		file, err := createSegmentFile(filePath)

		if err != nil {
			return nil, err
		}

		if err := file.Close(); err != nil {
			return nil, err
		}
	}

	filePath := filepath.Join(dirPath, fmt.Sprintf("%d.log", lastSegmentIndex))
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return nil, err
	}

	// Seek to the end of the file
	if _, err = file.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())

	wal := &SymbolWAL{
		dirPath:             dirPath,
		symbol:              symbol,
		maxFileSize:         maxFileSize,
		currentSegmentFile:  file,
		currentSegmentIndex: lastSegmentIndex,
		shouldFsync:         enableFsync,
		bufferWriter:        bufio.NewWriter(file),
		syncTimer:           time.NewTimer(time.Duration(syncIntervalMM) * time.Millisecond),
		nextOffset:          0,
		syncIntervalMM:      syncIntervalMM,
		ctx:                 ctx,
		cancel:              cancel,
	}

	offset, err := wal.findLastSequenceNumber(file.Name())
	if err != nil {
		return nil, err
	}
	wal.nextOffset = offset + 1

	// go wal.keepSyncing()
	return wal, nil
}

// WriteEntry writes an entry to the WAL.
func (wal *SymbolWAL) WriteEntry(data []byte) error {
	return wal.writeEntry(data)
}

func (sw *SymbolWAL) writeEntry(data []byte) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if err := sw.rotateFile(); err != nil {
		return err
	}

	var seqBytes [8]byte
	binary.LittleEndian.PutUint64(seqBytes[:], sw.nextOffset)
	crc := crc32.ChecksumIEEE(append(data, seqBytes[:]...))

	entry := &pbTypes.WAL_Entry{
		SequenceNumber: sw.nextOffset,
		Data:           data,
		CRC:            crc,
	}

	sw.nextOffset++

	return sw.writeEntryToBuffer(entry)
}

func (sw *SymbolWAL) writeEntryToBuffer(entry *pbTypes.WAL_Entry) error {
	marshalEntry, err := proto.Marshal(entry)
	if err != nil {
		return err
	}

	size := int32(len(marshalEntry))

	if err := binary.Write(sw.bufferWriter, binary.LittleEndian, size); err != nil {
		return err
	}

	if _, err := sw.bufferWriter.Write(marshalEntry); err != nil {
		return err
	}

	return nil
}

func (sw *SymbolWAL) rotateFile() error {
	fileInfo, err := sw.currentSegmentFile.Stat()
	if err != nil {
		return err
	}

	if fileInfo.Size()+int64(sw.bufferWriter.Buffered()) >= sw.maxFileSize {
		sw.Sync()
		if err := sw.currentSegmentFile.Close(); err != nil {
			return err
		}

		filePath := filepath.Join(sw.dirPath, fmt.Sprintf("%v.log", sw.currentSegmentIndex+1))

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		sw.currentSegmentFile = file
		sw.currentSegmentIndex++
		sw.bufferWriter = bufio.NewWriter(file)
	}

	return nil
}

func (sw *SymbolWAL) Sync() error {
	if err := sw.bufferWriter.Flush(); err != nil {
		return err
	}

	if sw.shouldFsync {
		if err := sw.currentSegmentFile.Sync(); err != nil {
			return err
		}
	}

	sw.resetTimer()

	return nil
}

func (sw *SymbolWAL) resetTimer() {
	sw.syncTimer.Reset(time.Duration(sw.syncIntervalMM) * time.Millisecond)
}

func (sw *SymbolWAL) keepSyncing() {
	for range sw.syncTimer.C {

		sw.mu.Lock()
		err := sw.Sync()
		sw.mu.Unlock()

		if err != nil {
			log.Printf("Error while performing sync: %v", err)
		}
	}
}

func (sw *SymbolWAL) findLastSequenceNumber(filename string) (uint64, error) {
	entry, err := sw.findLastEntryInLog(filename)
	if err != nil {
		return 0, err
	}

	if entry != nil {
		return entry.GetSequenceNumber(), nil
	}

	return 0, nil
}

func (sw *SymbolWAL) findLastEntryInLog(filename string) (*pbTypes.WAL_Entry, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lastDataPosition int64 // Where the last entry's DATA starts
	var lastEntrySize int32    // Size of the last complete entry

	for {
		var size int32

		// WAL Entry Format: [4-byte length][protobuf message]
		// [protobuf message]: Sequence number, Data, CRC, Is Checkpoint
		if err := binary.Read(file, binary.LittleEndian, &size); err != nil {
			if err == io.EOF {
				if lastDataPosition == 0 {
					return nil, nil
				}
				// Seek to where last entry's data starts
				if _, err := file.Seek(lastDataPosition, io.SeekStart); err != nil {
					return nil, err
				}

				data := make([]byte, lastEntrySize)

				if _, err := io.ReadFull(file, data); err != nil {
					return nil, err
				}

				// Unmarshal protobuf
				entry, err := unmarshalAndVerifyEntry(data)
				if err != nil {
					return nil, err
				}

				return entry, nil
			}
			return nil, err
		}

		if size <= 0 {
			return nil, fmt.Errorf("invaild entry size %d", size)
		}

		// Save current position (where data starts) and size
		lastDataPosition, err = file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		lastEntrySize = size

		// Skip over the entry data to get to next size field
		_, err := file.Seek(int64(size), io.SeekCurrent)
		if err != nil {
			return nil, err
		}
	}
}

func (sw *SymbolWAL) ReadFromTo(from, to uint64) ([]*pbTypes.WAL_Entry, error) {
	if from > to {
		return nil, fmt.Errorf("invalid range: from > to")
	}

	entries, err := os.ReadDir(sw.dirPath)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	results := make([]*pbTypes.WAL_Entry, 0, to-from+1)

	for _, dirEntry := range entries {
		if dirEntry.IsDir() {
			continue
		}

		path := filepath.Join(sw.dirPath, dirEntry.Name())
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		for {
			var size uint32
			if err := binary.Read(file, binary.LittleEndian, &size); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}

			if size == 0 {
				return nil, fmt.Errorf("invalid WAL record size")
			}

			data := make([]byte, size)
			if _, err := io.ReadFull(file, data); err != nil {
				return nil, err
			}

			entry, err := unmarshalAndVerifyEntry(data)
			if err != nil {
				return nil, err
			}

			seq := entry.GetSequenceNumber()

			if seq < from {
				continue
			}

			if seq > to {
				return results, nil
			}

			results = append(results, entry)
		}
	}

	return results, nil
}

func (sw *SymbolWAL) ReadFromToLast(from uint64) ([]*pbTypes.WAL_Entry, error) {
	entries, err := os.ReadDir(sw.dirPath)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	results := make([]*pbTypes.WAL_Entry, 0, 1_000_000)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(sw.dirPath, entry.Name())

		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		for {
			var size uint32
			if err := binary.Read(file, binary.LittleEndian, &size); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			if size == 0 {
				return nil, fmt.Errorf("invalid WAL record size")
			}

			data := make([]byte, size)
			if _, err := io.ReadFull(file, data); err != nil {
				return nil, err
			}

			logEntry, err := unmarshalAndVerifyEntry(data)
			if err != nil {
				return nil, err
			}

			seq := logEntry.GetSequenceNumber()

			if seq < from {
				continue
			}

			results = append(results, logEntry)
		}
	}

	return results, nil
}

func unmarshalAndVerifyEntry(data []byte) (*pbTypes.WAL_Entry, error) {

	// Unmarshal protobuf
	entry := &pbTypes.WAL_Entry{}
	if err := proto.Unmarshal(data, entry); err != nil {
		panic(fmt.Sprintf("unmarshal should never fail (%v)", err))
	}

	actualChecksum := crc32.ChecksumIEEE(append(entry.GetData(), byte(entry.GetSequenceNumber())))

	if actualChecksum != entry.GetCRC() {
		return nil, fmt.Errorf("CRC mismatch: data may be corrupted")
	}

	return entry, nil
}

func createSegmentFile(filePath string) (*os.File, error) {
	file, err := os.Create(filePath)

	if err != nil {
		return nil, err
	}

	return file, nil
}
