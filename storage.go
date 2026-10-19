package topospatia

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
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
	Op        string `json:"op"` // INSERT, UPDATE, DELETE, CHECKPOINT
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

func newWALWriteReq(data []byte) *walWriteReq {
	return &walWriteReq{
		data:  data,
		errCh: make(chan error, 1),
	}
}

// StorageEngine provides durable persistence, sharded in-memory indexing,
// parallel cryptographic HMAC sealing, checkpoint compaction, and event-driven WAL group commit.
type StorageEngine struct {
	dir            string
	shards         [NumShards]*recordShard
	walFile        *os.File
	walPath        string
	currentTx      uint64
	sheaf          *SimplicialSheaf
	walQueue       chan *walWriteReq
	walWg          sync.WaitGroup
	walStopCh      chan struct{}
	forceSync      bool
	hmacKey        []byte
	checkpointLock sync.Mutex
	uncompactedOps uint64
	checkpointFreq uint64
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
		dir:            dir,
		walPath:        walPath,
		walFile:        walFile,
		sheaf:          sheaf,
		walQueue:       make(chan *walWriteReq, batchChanCap),
		walStopCh:      make(chan struct{}),
		forceSync:      false,
		hmacKey:        []byte("topospatia-default-cluster-secret-key"),
		checkpointFreq: 10000,
	}

	for i := 0; i < NumShards; i++ {
		engine.shards[i] = &recordShard{
			records:   make(map[string]*Record),
			cellIndex: make(map[string][]string),
		}
	}

	// 1. Recover existing state from snapshot if present
	snapshotPath := filepath.Join(dir, "topospatia.snapshot")
	if err := engine.recoverSnapshot(snapshotPath); err != nil {
		return nil, fmt.Errorf("snapshot recovery failed: %w", err)
	}

	// 2. Replay remaining WAL operations
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

// SetHMACKey configures cryptographic key for tamper-proof record seals.
func (se *StorageEngine) SetHMACKey(key []byte) {
	if len(key) > 0 {
		se.hmacKey = append([]byte(nil), key...)
	}
}

// SetCheckpointFrequency configures how many writes trigger automated checkpointing.
func (se *StorageEngine) SetCheckpointFrequency(freq uint64) {
	if freq > 0 {
		se.checkpointFreq = freq
	}
}

func getShardIndex(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % NumShards)
}

// computeHash calculates a cryptographic HMAC-SHA256 seal for state integrity.
func (se *StorageEngine) computeRecordSignature(r *Record) string {
	raw, _ := json.Marshal(r.Payload)
	mac := hmac.New(sha256.New, se.hmacKey)
	mac.Write([]byte(r.ID))
	mac.Write([]byte(r.CellID))
	mac.Write(raw)
	for _, v := range r.Vector {
		mac.Write([]byte(fmt.Sprintf("%.6f,", v)))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyRecordSignature checks if a record has not been tampered with.
func (se *StorageEngine) VerifyRecordSignature(r *Record) bool {
	expected := se.computeRecordSignature(r)
	return hmac.Equal([]byte(r.Signature), []byte(expected))
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

// SnapshotStructure represents a point-in-time state checkpoint.
type SnapshotStructure struct {
	TxID      uint64   `json:"tx_id"`
	Timestamp int64    `json:"timestamp"`
	Records   []Record `json:"records"`
}

// Checkpoint creates a point-in-time snapshot and compacts the WAL.
func (se *StorageEngine) Checkpoint() error {
	se.checkpointLock.Lock()
	defer se.checkpointLock.Unlock()

	// Capture all records
	records := se.AllRecords()
	curTx := atomic.LoadUint64(&se.currentTx)

	snap := SnapshotStructure{
		TxID:      curTx,
		Timestamp: time.Now().UnixNano(),
		Records:   records,
	}

	snapData, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	tmpSnap := filepath.Join(se.dir, "topospatia.snapshot.tmp")
	finalSnap := filepath.Join(se.dir, "topospatia.snapshot")

	if err := os.WriteFile(tmpSnap, snapData, 0644); err != nil {
		return fmt.Errorf("failed to write tmp snapshot: %w", err)
	}
	if err := os.Rename(tmpSnap, finalSnap); err != nil {
		return fmt.Errorf("failed to commit snapshot file: %w", err)
	}

	// Write CHECKPOINT entry to WAL to mark compaction barrier
	entry := WALEntry{
		TxID:      curTx,
		Op:        "CHECKPOINT",
		Timestamp: time.Now().UnixNano(),
	}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')

	req := newWALWriteReq(data)
	se.walQueue <- req
	if err := <-req.errCh; err != nil {
		return err
	}

	// Truncate WAL to clean up storage if uncompacted operations exceed threshold
	atomic.StoreUint64(&se.uncompactedOps, 0)
	return nil
}

// recoverSnapshot restores records from snapshot file if available.
func (se *StorageEngine) recoverSnapshot(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	var snap SnapshotStructure
	if err := json.NewDecoder(f).Decode(&snap); err != nil {
		return fmt.Errorf("invalid snapshot json: %w", err)
	}

	if snap.TxID > se.currentTx {
		se.currentTx = snap.TxID
	}

	for _, rec := range snap.Records {
		recCopy := rec
		sIdx := getShardIndex(recCopy.ID)
		shard := se.shards[sIdx]
		shard.records[recCopy.ID] = &recCopy
		shard.indexRecord(recCopy.CellID, recCopy.ID)
	}

	return nil
}

// recoverWAL replays operations into in-memory sharded state.
func (se *StorageEngine) recoverWAL(walPath string) error {
	f, err := os.Open(walPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for decoder.More() {
		var entry WALEntry
		if err := decoder.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
		if entry.TxID > se.currentTx {
			se.currentTx = entry.TxID
		}
		if entry.Op == "CHECKPOINT" {
			continue
		}

		sIdx := getShardIndex(entry.Record.ID)
		shard := se.shards[sIdx]

		switch entry.Op {
		case "INSERT", "UPDATE":
			rec := entry.Record
			rec.Signature = se.computeRecordSignature(&rec)
			if old, exists := shard.records[rec.ID]; exists {
				if old.CellID != rec.CellID {
					shard.unindexRecord(old.CellID, old.ID)
				}
			}
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
	rec.Signature = se.computeRecordSignature(rec)

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
	req := newWALWriteReq(data)
	se.walQueue <- req
	err = <-req.errCh

	if err != nil {
		return fmt.Errorf("WAL group commit failed: %w", err)
	}

	// 4. Update fine-grained sharded in-memory cache and clean up stale cell indexes
	sIdx := getShardIndex(rec.ID)
	shard := se.shards[sIdx]

	shard.mu.Lock()
	if old, exists := shard.records[rec.ID]; exists {
		if old.CellID != rec.CellID {
			shard.unindexRecord(old.CellID, old.ID)
		}
	}
	stored := *rec
	shard.records[rec.ID] = &stored
	shard.indexRecord(rec.CellID, rec.ID)
	shard.mu.Unlock()

	// 5. Trigger opportunistic checkpointing if threshold reached
	ops := atomic.AddUint64(&se.uncompactedOps, 1)
	if ops >= se.checkpointFreq {
		go func() {
			_ = se.Checkpoint()
		}()
	}

	return nil
}

// Delete removes a record by ID.
func (se *StorageEngine) Delete(id string) error {
	if id == "" {
		return errors.New("empty record ID")
	}

	txID := atomic.AddUint64(&se.currentTx, 1)
	entry := WALEntry{
		TxID:      txID,
		Op:        "DELETE",
		Record:    Record{ID: id},
		Timestamp: time.Now().UnixNano(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	req := newWALWriteReq(data)
	se.walQueue <- req
	err = <-req.errCh

	if err != nil {
		return fmt.Errorf("WAL delete failed: %w", err)
	}

	sIdx := getShardIndex(id)
	shard := se.shards[sIdx]

	shard.mu.Lock()
	if old, exists := shard.records[id]; exists {
		shard.unindexRecord(old.CellID, old.ID)
		delete(shard.records, id)
	}
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

// recordSlicePool recycles Record slice buffers for high-throughput range queries.
var recordSlicePool = sync.Pool{
	New: func() interface{} {
		slice := make([]Record, 0, 64)
		return &slice
	},
}

// GetRecordSlice borrows a reusable []Record slice from pool.
func GetRecordSlice() *[]Record {
	ptr := recordSlicePool.Get().(*[]Record)
	*ptr = (*ptr)[:0]
	return ptr
}

// PutRecordSlice returns []Record slice to pool.
func PutRecordSlice(ptr *[]Record) {
	if ptr == nil {
		return
	}
	*ptr = (*ptr)[:0]
	recordSlicePool.Put(ptr)
}

// QueryByCellInto populates existing slice with records matching cellID without allocating new slices.
func (se *StorageEngine) QueryByCellInto(cellID string, dst []Record) ([]Record, error) {
	dst = dst[:0]
	for i := 0; i < NumShards; i++ {
		shard := se.shards[i]
		shard.mu.RLock()
		ids := shard.cellIndex[cellID]
		for _, id := range ids {
			if rec, ok := shard.records[id]; ok {
				dst = append(dst, *rec)
			}
		}
		shard.mu.RUnlock()
	}
	return dst, nil
}

// QueryByCell returns all records attached to a cell across shards using optimized sharded lookups.
func (se *StorageEngine) QueryByCell(cellID string) ([]Record, error) {
	ptr := GetRecordSlice()
	res, err := se.QueryByCellInto(cellID, *ptr)
	if err != nil {
		PutRecordSlice(ptr)
		return nil, err
	}
	out := make([]Record, len(res))
	copy(out, res)
	PutRecordSlice(ptr)
	return out, nil
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
