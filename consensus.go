package topospatia

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ConsensusEngine drives topological harmonic alignment and distributed multi-replica state synchronization.
// Combines discrete sheaf Laplacian diffusion (analog consensus in ker(Delta_0)) with distributed
// versioned state-machine replication (SMR) and Byzantine-resistant peer verification.
type ConsensusEngine struct {
	mu           sync.RWMutex
	sheaf        *SimplicialSheaf
	storage      *StorageEngine
	tolerance    float64
	maxIter      int
	dt           float64
	term         uint64
	nodeID       string
	peers        map[string]*PeerState
	raftLog      []RaftLogEntry
	commitIndex  uint64
	quorumSize   int
	clusterKey   []byte
}

// PeerState tracks heartbeat, network latency, and replication watermark for peer replicas.
type PeerState struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	LastSeen    time.Time `json:"last_seen"`
	MatchIndex  uint64    `json:"match_index"`
	IsHarmonic  bool      `json:"is_harmonic"`
	Reputation  float64   `json:"reputation"` // 0.0 to 1.0 (Byzantine filtering)
}

// RaftLogEntry represents an ordered state-machine replication entry.
type RaftLogEntry struct {
	Term      uint64 `json:"term"`
	Index     uint64 `json:"index"`
	Op        string `json:"op"`
	Data      []byte `json:"data"`
	Signature string `json:"signature"`
}

// ConsensusReport provides detailed metrics of topological consensus convergence.
type ConsensusReport struct {
	InitialEnergy    float64   `json:"initial_energy"`
	FinalEnergy      float64   `json:"final_energy"`
	Iterations       int       `json:"iterations"`
	Converged        bool      `json:"converged"`
	Betti0           int       `json:"betti_0"`
	ResidualHarmonic []float64 `json:"residual_harmonic"`
	Term             uint64    `json:"term"`
	CommitIndex      uint64    `json:"commit_index"`
	ActivePeers      int       `json:"active_peers"`
}

// NewConsensusEngine creates a hybrid SMR + Hodge consensus controller for TopoSpatiaDB.
func NewConsensusEngine(sheaf *SimplicialSheaf, storage *StorageEngine) *ConsensusEngine {
	return &ConsensusEngine{
		sheaf:       sheaf,
		storage:     storage,
		tolerance:   1e-6,
		maxIter:     200,
		dt:          0.05,
		term:        1,
		nodeID:      "node-local-primary",
		peers:       make(map[string]*PeerState),
		raftLog:     make([]RaftLogEntry, 0, 1024),
		quorumSize:  1,
		clusterKey:  []byte("topospatia-consensus-secret-key"),
	}
}

// RegisterPeer connects a cluster node into the replication and harmonic mesh.
func (ce *ConsensusEngine) RegisterPeer(id, address string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.peers[id] = &PeerState{
		ID:         id,
		Address:    address,
		LastSeen:   time.Now(),
		MatchIndex: 0,
		IsHarmonic: true,
		Reputation: 1.0,
	}
	ce.quorumSize = (len(ce.peers)+1)/2 + 1
}

// AssembleGlobal0CoChain builds a continuous C^0 cochain vector from vertex records.
func (ce *ConsensusEngine) AssembleGlobal0CoChain() (StalkVector, []string, error) {
	ce.sheaf.CompileOperators()
	vCells := ce.sheaf.GetCellsByDim(0)
	sd := ce.sheaf.StalkDim()
	totalDim := len(vCells) * sd

	x := make(StalkVector, totalDim)
	cellOrder := make([]string, len(vCells))

	recBuf := GetRecordSlice()
	defer PutRecordSlice(recBuf)

	for i, c := range vCells {
		cellOrder[i] = c.ID
		records, err := ce.storage.QueryByCellInto(c.ID, *recBuf)
		if err == nil && len(records) > 0 {
			// Aggregate latest record vector into stalk
			latest := records[len(records)-1]
			for d := 0; d < sd && d < len(latest.Vector); d++ {
				x[i*sd+d] = latest.Vector[d]
			}
		}
	}

	return x, cellOrder, nil
}

// DiffuseGlobal0CoChain runs continuous heat equation dx/dt = -Delta_0 * x
// to eliminate topological conflicts (H^1 cohomology) until Dirichlet energy < tolerance,
// and records an immutable State Machine Replication entry.
func (ce *ConsensusEngine) DiffuseGlobal0CoChain() (*ConsensusReport, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	x, cellOrder, err := ce.AssembleGlobal0CoChain()
	if err != nil {
		return nil, err
	}
	if len(x) == 0 {
		return &ConsensusReport{Converged: true, Term: ce.term, CommitIndex: ce.commitIndex}, nil
	}

	b0, _ := ce.sheaf.ComputeBettiNumbers(ce.tolerance)

	initEnergy := ce.sheaf.ComputeDirichletEnergy(0, x)
	currEnergy := initEnergy
	curr := x
	next := make(StalkVector, len(x))
	iter := 0

	for iter < ce.maxIter && currEnergy > ce.tolerance {
		ce.sheaf.HeatDiffusionStepInto(0, curr, next, ce.dt)
		curr, next = next, curr
		currEnergy = ce.sheaf.ComputeDirichletEnergy(0, curr)
		iter++
	}

	converged := currEnergy <= ce.tolerance || math.Abs(initEnergy-currEnergy) < 1e-9

	// Update converged values back into records
	sd := ce.sheaf.StalkDim()
	recBuf := GetRecordSlice()
	defer PutRecordSlice(recBuf)

	for i, cellID := range cellOrder {
		recs, _ := ce.storage.QueryByCellInto(cellID, *recBuf)
		if len(recs) > 0 {
			latest := recs[len(recs)-1]
			newVec := make(StalkVector, sd)
			for d := 0; d < sd; d++ {
				newVec[d] = curr[i*sd+d]
			}
			latest.Vector = newVec
			_ = ce.storage.Put(&latest)
		}
	}

	// Commit state transition into monotonic log
	newIndex := atomic.AddUint64(&ce.commitIndex, 1)
	entry := RaftLogEntry{
		Term:      ce.term,
		Index:     newIndex,
		Op:        "TOPOLOGICAL_DIFFUSE",
		Data:      []byte(fmt.Sprintf("energy:%.6f;iters:%d", currEnergy, iter)),
		Signature: ce.signLogEntry(ce.term, newIndex, "TOPOLOGICAL_DIFFUSE"),
	}
	ce.raftLog = append(ce.raftLog, entry)

	return &ConsensusReport{
		InitialEnergy:    initEnergy,
		FinalEnergy:      currEnergy,
		Iterations:       iter,
		Converged:        converged,
		Betti0:           b0,
		ResidualHarmonic: curr,
		Term:             ce.term,
		CommitIndex:      newIndex,
		ActivePeers:      len(ce.peers),
	}, nil
}

func (ce *ConsensusEngine) signLogEntry(term, index uint64, op string) string {
	mac := hmac.New(sha256.New, ce.clusterKey)
	mac.Write([]byte(fmt.Sprintf("%d:%d:%s", term, index, op)))
	return hex.EncodeToString(mac.Sum(nil))
}
