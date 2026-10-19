package topospatia

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Record represents a topological document or entity stored inside a Cell stalk.
type Record struct {
	ID        string                 `json:"id"`
	CellID    string                 `json:"cell_id"`
	Dimension int                    `json:"dimension"`
	Vector    StalkVector            `json:"vector"`
	Payload   map[string]interface{} `json:"payload"`
	Version   uint64                 `json:"version"`
	UpdatedAt int64                  `json:"updated_at"`
	Signature string                 `json:"signature"`
}

// WALEntry represents an append-only write-ahead log item for ACID transactions.
type WALEntry struct {
	TxID      uint64 `json:"tx_id"`
	Op        string `json:"op"` // INSERT, UPDATE, DELETE, DIFFUSION, RESTRICTION
	Record    Record `json:"record,omitempty"`
	CellID    string `json:"cell_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

const (
	NumShards    = 32
	batchChanCap = 16384
	maxBatchSize = 512
)

type recordShard struct {
	mu        sync.RWMutex
	records   map[string]*Record
	cellIndex map[string][]string
}

type walWriteReq struct {
	data  []byte
	errCh chan error
}

var reqPool = sync.Pool{
	New: func() interface{} {
		return &walWriteReq{
			errCh: make(chan error, 1),
		}
	},
}

// StorageEngine provides durable persistence, sharded in-memory indexing,
// parallel cryptographic hashing, and non-delayed event-driven WAL group commit.
type StorageEngine struct {
	dir         string
	shards      [NumShards]*recordShard
	walFile     *os.File
	currentTx   uint64
	sheaf       *SimplicialSheaf
	walQueue    chan *walWriteReq
	walWg       sync.WaitGroup
	walStopCh   chan struct{}
	forceSync   bool // configurable fsync policy
}

// OpenStorage initializes or loads the storage engine from disk.
func OpenStorage(dir string, sheaf *SimplicialSheaf) (*StorageEngine, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	walPath := filepath.Join(dir, "topospatia.wal")
	walFile, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL: %w", err)
	}

	engine := &StorageEngine{
		dir:       dir,
		walFile:   walFile,
		sheaf:     sheaf,
		walQueue:  make(chan *walWriteReq, batchChanCap),
		walStopCh: make(chan struct{}),
		forceSync: false,
	}

	for i := 0; i < NumShards; i++ {
		engine.shards[i] = &recordShard{
			records:   make(map[string]*Record),
			cellIndex: make(map[string][]string),
		}
	}

	if err := engine.recoverWAL(walPath); err != nil {
		return nil, fmt.Errorf("WAL recovery failed: %w", err)
	}

	// Start event-driven non-delayed group commit worker
	engine.walWg.Add(1)
	go engine.walGroupCommitWorker()

	return engine, nil
}

// SetFsync controls whether fsync is called on group commit batches.
func (se *StorageEngine) SetFsync(force bool) {
	se.forceSync = force
}

func getShardIndex(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % NumShards)
}

// computeHash calculates a cryptographic seal for state integrity.
func (r *Record) computeHash() string {
	raw, _ := json.Marshal(r.Payload)
	h := sha256.New()
	h.Write([]byte(r.ID))
	h.Write([]byte(r.CellID))
	h.Write(raw)
	for _, v := range r.Vector {
		h.Write([]byte(fmt.Sprintf("%.6f,", v)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// walGroupCommitWorker executes opportunistic event-driven batching WITHOUT timer delays.
func (se *StorageEngine) walGroupCommitWorker() {
	defer se.walWg.Done()

	batch := make([]*walWriteReq, 0, maxBatchSize)
	var buf bytes.Buffer
	buf.Grow(128 * 1024)

	for {
		// 1. Block only when queue is completely empty (0 us delay on first item)
		select {
		case req, ok := <-se.walQueue:
			if !ok {
				return
			}
			batch = append(batch, req)
		case <-se.walStopCh:
			// Drain remaining on shutdown
			for {
				select {
				case req := <-se.walQueue:
					batch = append(batch, req)
				default:
					if len(batch) > 0 {
						buf.Reset()
						for _, r := range batch {
							buf.Write(r.data)
						}
						_, err := se.walFile.Write(buf.Bytes())
						if err == nil && se.forceSync {
							_ = se.walFile.Sync()
						}
						for _, r := range batch {
							r.errCh <- err
						}
					}
					return
				}
			}
		}

		// 2. Opportunistically drain whatever has already arrived in the buffer right now
	drainLoop:
		for len(batch) < maxBatchSize && buf.Len() < 64*1024 {
			select {
			case req := <-se.walQueue:
				batch = append(batch, req)
			default:
				// Queue empty right now: DO NOT wait for timer, write immediately!
				break drainLoop
			}
		}

		// 3. Serialize batch into preallocated buffer and execute single write()
		buf.Reset()
		for _, req := range batch {
			buf.Write(req.data)
		}

		_, err := se.walFile.Write(buf.Bytes())
		if err == nil && se.forceSync {
			err = se.walFile.Sync()
		}

		// 4. Notify waiting callers immediately
		for _, req := range batch {
			req.errCh <- err
		}
		batch = batch[:0]
	}
}

// Close flushes data and closes WAL worker.
func (se *StorageEngine) Close() error {
	close(se.walStopCh)
	se.walWg.Wait()
	if se.walFile != nil {
		_ = se.walFile.Sync()
		return se.walFile.Close()
	}
	return nil
}

// recoverWAL replays operations into in-memory sharded state.
func (se *StorageEngine) recoverWAL(walPath string) error {
	f, err := os.Open(walPath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for decoder.More() {
		var entry WALEntry
		if err := decoder.Decode(&entry); err != nil {
			break
		}
		if entry.TxID > se.currentTx {
			se.currentTx = entry.TxID
		}
		sIdx := getShardIndex(entry.Record.ID)
		shard := se.shards[sIdx]

		switch entry.Op {
		case "INSERT", "UPDATE":
			rec := entry.Record
			rec.Signature = rec.computeHash()
			shard.records[rec.ID] = &rec
			shard.indexRecord(rec.CellID, rec.ID)
		case "DELETE":
			if old, exists := shard.records[entry.Record.ID]; exists {
				shard.unindexRecord(old.CellID, old.ID)
				delete(shard.records, old.ID)
			}
		}
	}
	return nil
}

func (shard *recordShard) indexRecord(cellID, recID string) {
	for _, id := range shard.cellIndex[cellID] {
		if id == recID {
			return
		}
	}
	shard.cellIndex[cellID] = append(shard.cellIndex[cellID], recID)
}

func (shard *recordShard) unindexRecord(cellID, recID string) {
	slice := shard.cellIndex[cellID]
	for i, id := range slice {
		if id == recID {
			shard.cellIndex[cellID] = append(slice[:i], slice[i+1:]...)
			return
		}
	}
}

// Put writes or updates a record inside a specific cell stalk with sharded lock & event-driven group commit.
func (se *StorageEngine) Put(rec *Record) error {
	if rec.ID == "" {
		return errors.New("empty record ID")
	}
	if rec.CellID == "" {
		return errors.New("empty cell ID")
	}

	// 1. Compute heavy cryptography OUTSIDE critical locks
	rec.Version++
	rec.UpdatedAt = time.Now().UnixNano()
	rec.Signature = rec.computeHash()

	// 2. Monotonic transaction assignment
	txID := atomic.AddUint64(&se.currentTx, 1)

	entry := WALEntry{
		TxID:      txID,
		Op:        "INSERT",
		Record:    *rec,
		CellID:    rec.CellID,
		Timestamp: rec.UpdatedAt,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// 3. Dispatch to Non-Delayed Event-Driven Group Commit queue
	req := reqPool.Get().(*walWriteReq)
	req.data = data
	// Drain any residual error if channel had leftover
	select {
	case <-req.errCh:
	default:
	}

	se.walQueue <- req
	err = <-req.errCh
	reqPool.Put(req)

	if err != nil {
		return fmt.Errorf("WAL group commit failed: %w", err)
	}

	// 4. Update fine-grained sharded in-memory cache
	sIdx := getShardIndex(rec.ID)
	shard := se.shards[sIdx]

	shard.mu.Lock()
	stored := *rec
	shard.records[rec.ID] = &stored
	shard.indexRecord(rec.CellID, rec.ID)
	shard.mu.Unlock()

	return nil
}

// Get retrieves a record by ID using sharded lock.
func (se *StorageEngine) Get(id string) (*Record, error) {
	sIdx := getShardIndex(id)
	shard := se.shards[sIdx]

	shard.mu.RLock()
	rec, exists := shard.records[id]
	if !exists {
		shard.mu.RUnlock()
		return nil, fmt.Errorf("record %s not found", id)
	}
	copied := *rec
	copied.Vector = CloneVector(rec.Vector)
	shard.mu.RUnlock()

	return &copied, nil
}

// QueryByCell returns all records attached to a cell across shards.
func (se *StorageEngine) QueryByCell(cellID string) ([]Record, error) {
	var results []Record

	for i := 0; i < NumShards; i++ {
		shard := se.shards[i]
		shard.mu.RLock()
		ids := shard.cellIndex[cellID]
		for _, id := range ids {
			if rec, ok := shard.records[id]; ok {
				copied := *rec
				copied.Vector = CloneVector(rec.Vector)
				results = append(results, copied)
			}
		}
		shard.mu.RUnlock()
	}

	return results, nil
}

// AllRecords returns all stored records across all shards.
func (se *StorageEngine) AllRecords() []Record {
	var list []Record

	for i := 0; i < NumShards; i++ {
		shard := se.shards[i]
		shard.mu.RLock()
		for _, r := range shard.records {
			cp := *r
			cp.Vector = CloneVector(r.Vector)
			list = append(list, cp)
		}
		shard.mu.RUnlock()
	}

	return list
}
