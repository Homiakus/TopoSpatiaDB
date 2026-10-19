package topospatia

import (
	"fmt"
	"os"
	"path/filepath"
)

// Options configures the embedded TopoSpatiaDB instance.
type Options struct {
	// DataDir is the path to the directory where WAL and state are persisted.
	// If empty, an in-memory/temp directory or default path can be used.
	DataDir string `json:"data_dir"`

	// MaxDim is the maximum dimension of simplices in the complex (e.g. 2 for triangles, 3 for tetrahedrons).
	MaxDim int `json:"max_dim"`

	// StalkDim is the dimension of the vector state assigned to each simplex cell (default: 2).
	StalkDim int `json:"stalk_dim"`

	// Fsync forces immediate disk sync on WAL group commit (default: false for maximum throughput).
	Fsync bool `json:"fsync"`

	// ConsensusTolerance is the Dirichlet energy threshold for consensus convergence (default: 1e-6).
	ConsensusTolerance float64 `json:"consensus_tolerance"`

	// ConsensusMaxIter is the maximum number of diffusion iterations (default: 200).
	ConsensusMaxIter int `json:"consensus_max_iter"`

	// ConsensusDt is the time step for gradient diffusion (default: 0.05).
	ConsensusDt float64 `json:"consensus_dt"`
}

// DefaultOptions returns recommended settings for embedded DBMS usage.
func DefaultOptions(dataDir string) Options {
	return Options{
		DataDir:            dataDir,
		MaxDim:             2,
		StalkDim:           2,
		Fsync:              false,
		ConsensusTolerance: 1e-6,
		ConsensusMaxIter:   200,
		ConsensusDt:        0.05,
	}
}

// DB represents a unified, production-grade embedded TopoSpatia database instance.
// Designed for seamless integration directly into Go applications without requiring
// external microservices, daemons or HTTP endpoints.
type DB struct {
	sheaf     *SimplicialSheaf
	storage   *StorageEngine
	consensus *ConsensusEngine
	opts      Options
}

// Open initializes or loads an embedded TopoSpatia database from disk with specified options.
func Open(opts Options) (*DB, error) {
	if opts.DataDir == "" {
		home, _ := os.UserHomeDir()
		opts.DataDir = filepath.Join(home, ".topospatia_data")
	}
	if opts.MaxDim <= 0 {
		opts.MaxDim = 2
	}
	if opts.StalkDim <= 0 {
		opts.StalkDim = 2
	}
	if opts.ConsensusTolerance <= 0 {
		opts.ConsensusTolerance = 1e-6
	}
	if opts.ConsensusMaxIter <= 0 {
		opts.ConsensusMaxIter = 200
	}
	if opts.ConsensusDt <= 0 {
		opts.ConsensusDt = 0.05
	}

	sheaf := NewSimplicialSheaf(opts.MaxDim, opts.StalkDim)
	storage, err := OpenStorage(opts.DataDir, sheaf)
	if err != nil {
		return nil, fmt.Errorf("topospatia: failed to open storage: %w", err)
	}
	storage.SetFsync(opts.Fsync)

	consensus := NewConsensusEngine(sheaf, storage)
	consensus.tolerance = opts.ConsensusTolerance
	consensus.maxIter = opts.ConsensusMaxIter
	consensus.dt = opts.ConsensusDt

	return &DB{
		sheaf:     sheaf,
		storage:   storage,
		consensus: consensus,
		opts:      opts,
	}, nil
}

// OpenDefault opens an embedded DB with standard high-performance options.
func OpenDefault(dataDir string) (*DB, error) {
	return Open(DefaultOptions(dataDir))
}

// Close gracefully flushes the WAL buffer, stops background commit workers, and closes files.
func (db *DB) Close() error {
	return db.storage.Close()
}

// --- Sheaf & Topology Operations ---

// AddCell inserts a simplex cell (e.g. vertex [0], edge [0, 1], triangle [0, 1, 2]).
func (db *DB) AddCell(vertices []int) (Cell, error) {
	return db.sheaf.AddCell(vertices)
}

// CompileOperators compiles coboundary operators and Hodge Laplacians.
func (db *DB) CompileOperators() {
	db.sheaf.CompileOperators()
}

// ComputeBettiNumbers returns Betti numbers (beta_0, beta_1).
func (db *DB) ComputeBettiNumbers(tol float64) (int, int) {
	return db.sheaf.ComputeBettiNumbers(tol)
}

// ComputeHigherBettiNumbers computes exact Betti numbers H^0, H^1, ..., H^maxDim.
func (db *DB) ComputeHigherBettiNumbers(tol float64) map[int]int {
	return db.sheaf.ComputeHigherBettiNumbers(tol)
}

// ComputeDirichletEnergy returns E(x) = x^T Delta_k x.
func (db *DB) ComputeDirichletEnergy(k int, x StalkVector) float64 {
	return db.sheaf.ComputeDirichletEnergy(k, x)
}

// DecomposeFlow1 performs Hodge-Helmholtz decomposition (Gradient + Solenoidal + Harmonic).
func (db *DB) DecomposeFlow1(flow StalkVector, maxSteps int, dt float64) HodgeDecomposition {
	return db.sheaf.DecomposeFlow1(flow, maxSteps, dt)
}

// SimplicialRAGDiffusion spreads query activation for LLM context retrieval.
func (db *DB) SimplicialRAGDiffusion(seedNodeIDs []int, steps int, alpha float64) map[string]float64 {
	return db.sheaf.SimplicialRAGDiffusion(seedNodeIDs, steps, alpha)
}

// --- Storage & CRUD Operations ---

// Put writes or updates a document in a cell stalk with ACID guarantee and SHA-256 seal.
func (db *DB) Put(rec *Record) error {
	return db.storage.Put(rec)
}

// Get retrieves a document by ID.
func (db *DB) Get(id string) (*Record, error) {
	return db.storage.Get(id)
}

// QueryByCell retrieves all records attached to a given cell ID.
func (db *DB) QueryByCell(cellID string) ([]Record, error) {
	return db.storage.QueryByCell(cellID)
}

// QueryByCellInto fills an existing record slice without heap allocations.
func (db *DB) QueryByCellInto(cellID string, dst []Record) ([]Record, error) {
	return db.storage.QueryByCellInto(cellID, dst)
}

// AllRecords returns a snapshot of all records in storage.
func (db *DB) AllRecords() []Record {
	return db.storage.AllRecords()
}

// --- Consensus Operations ---

// Diffuse executes topological consensus diffusion to resolve replica conflicts.
func (db *DB) Diffuse() (*ConsensusReport, error) {
	return db.consensus.DiffuseGlobal0CoChain()
}

// --- Component Accessors ---

// Sheaf returns the underlying SimplicialSheaf.
func (db *DB) Sheaf() *SimplicialSheaf {
	return db.sheaf
}

// Storage returns the underlying StorageEngine.
func (db *DB) Storage() *StorageEngine {
	return db.storage
}

// Consensus returns the underlying ConsensusEngine.
func (db *DB) Consensus() *ConsensusEngine {
	return db.consensus
}
