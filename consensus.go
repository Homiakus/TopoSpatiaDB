package topospatia

import (
	"math"
	"sync"
)

// ConsensusEngine drives topological diffusion over the simplicial complex.
// Resolves H^1 replica disagreements by projecting states onto harmonic kernels ker(Delta_0).
type ConsensusEngine struct {
	mu        sync.RWMutex
	sheaf     *SimplicialSheaf
	storage   *StorageEngine
	tolerance float64
	maxIter   int
	dt        float64
}

// ConsensusReport provides detailed metrics of topological consensus convergence.
type ConsensusReport struct {
	InitialEnergy   float64   `json:"initial_energy"`
	FinalEnergy     float64   `json:"final_energy"`
	Iterations      int       `json:"iterations"`
	Converged       bool      `json:"converged"`
	ResidualHarmonic []float64 `json:"residual_harmonic"`
}

// NewConsensusEngine creates a consensus controller for TopoSpatiaDB.
func NewConsensusEngine(sheaf *SimplicialSheaf, storage *StorageEngine) *ConsensusEngine {
	return &ConsensusEngine{
		sheaf:     sheaf,
		storage:   storage,
		tolerance: 1e-6,
		maxIter:   200,
		dt:        0.05,
	}
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
// to eliminate topological conflicts (H^1 cohomology) until Dirichlet energy < tolerance.
func (ce *ConsensusEngine) DiffuseGlobal0CoChain() (*ConsensusReport, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	x, cellOrder, err := ce.AssembleGlobal0CoChain()
	if err != nil {
		return nil, err
	}
	if len(x) == 0 {
		return &ConsensusReport{Converged: true}, nil
	}

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

	return &ConsensusReport{
		InitialEnergy:   initEnergy,
		FinalEnergy:     currEnergy,
		Iterations:      iter,
		Converged:       converged,
		ResidualHarmonic: curr,
	}, nil
}
