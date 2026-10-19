package topospatia

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// Cell represents an oriented k-simplex (0=vertex/node, 1=edge/link, 2=triangle/face, 3=tetrahedron).
type Cell struct {
	Dimension int    `json:"dimension"`
	ID        string `json:"id"`
	Vertices  []int  `json:"vertices"`
}

// StalkVector represents a state vector attached to a cell (Hilbert space element).
type StalkVector []float64

func CloneVector(v StalkVector) StalkVector {
	res := make(StalkVector, len(v))
	copy(res, v)
	return res
}

func Dot(a, b StalkVector) float64 {
	sum := 0.0
	n := len(a)
	i := 0
	for ; i+3 < n; i += 4 {
		sum += a[i]*b[i] + a[i+1]*b[i+1] + a[i+2]*b[i+2] + a[i+3]*b[i+3]
	}
	for ; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

// Matrix represents a real restriction operator or Laplacian.
type Matrix [][]float64

func NewMatrix(rows, cols int) Matrix {
	m := make(Matrix, rows)
	for i := range m {
		m[i] = make([]float64, cols)
	}
	return m
}

func MatVecMul(m Matrix, v StalkVector) StalkVector {
	rows := len(m)
	if rows == 0 {
		return nil
	}
	cols := len(m[0])
	res := make(StalkVector, rows)
	for r := 0; r < rows; r++ {
		row := m[r]
		sum := 0.0
		c := 0
		for ; c+3 < cols; c += 4 {
			sum += row[c]*v[c] + row[c+1]*v[c+1] + row[c+2]*v[c+2] + row[c+3]*v[c+3]
		}
		for ; c < cols; c++ {
			sum += row[c] * v[c]
		}
		res[r] = sum
	}
	return res
}

func MatTranspose(m Matrix) Matrix {
	rows := len(m)
	if rows == 0 {
		return nil
	}
	cols := len(m[0])
	res := NewMatrix(cols, rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			res[c][r] = m[r][c]
		}
	}
	return res
}

func MatMul(a, b Matrix) Matrix {
	rA := len(a)
	if rA == 0 {
		return nil
	}
	cA := len(a[0])
	cB := len(b[0])
	res := NewMatrix(rA, cB)
	for i := 0; i < rA; i++ {
		rowA := a[i]
		rowRes := res[i]
		for k := 0; k < cA; k++ {
			aik := rowA[k]
			if aik != 0.0 {
				rowB := b[k]
				j := 0
				for ; j+3 < cB; j += 4 {
					rowRes[j] += aik * rowB[j]
					rowRes[j+1] += aik * rowB[j+1]
					rowRes[j+2] += aik * rowB[j+2]
					rowRes[j+3] += aik * rowB[j+3]
				}
				for ; j < cB; j++ {
					rowRes[j] += aik * rowB[j]
				}
			}
		}
	}
	return res
}

// SparseEntry represents a non-zero element in a compressed sparse matrix row.
type SparseEntry struct {
	Col int
	Val float64
}

// SparseMatrix represents a compressed row sparse matrix for sub-microsecond Hodge operations.
type SparseMatrix struct {
	Rows int
	Cols int
	Data [][]SparseEntry
}

func NewSparseMatrix(rows, cols int) *SparseMatrix {
	return &SparseMatrix{
		Rows: rows,
		Cols: cols,
		Data: make([][]SparseEntry, rows),
	}
}

func (sm *SparseMatrix) MulVecInto(v StalkVector, res StalkVector) {
	for r := 0; r < sm.Rows; r++ {
		entries := sm.Data[r]
		m := len(entries)
		sum := 0.0
		j := 0
		for ; j+3 < m; j += 4 {
			e0, e1, e2, e3 := entries[j], entries[j+1], entries[j+2], entries[j+3]
			sum += e0.Val*v[e0.Col] + e1.Val*v[e1.Col] + e2.Val*v[e2.Col] + e3.Val*v[e3.Col]
		}
		for ; j < m; j++ {
			sum += entries[j].Val * v[entries[j].Col]
		}
		res[r] = sum
	}
}

func (sm *SparseMatrix) MulVec(v StalkVector) StalkVector {
	res := make(StalkVector, sm.Rows)
	sm.MulVecInto(v, res)
	return res
}

func DenseToSparse(m Matrix) *SparseMatrix {
	rows := len(m)
	if rows == 0 {
		return NewSparseMatrix(0, 0)
	}
	cols := len(m[0])
	sm := NewSparseMatrix(rows, cols)
	for r := 0; r < rows; r++ {
		row := m[r]
		for c := 0; c < cols; c++ {
			if row[c] != 0.0 {
				sm.Data[r] = append(sm.Data[r], SparseEntry{Col: c, Val: row[c]})
			}
		}
	}
	return sm
}

// SimplexTreeNode implements prefix-tree lookup for simplices.
type SimplexTreeNode struct {
	Vertex   int
	Children map[int]*SimplexTreeNode
	Cell     *Cell
}

func NewSimplexTreeNode(v int) *SimplexTreeNode {
	return &SimplexTreeNode{
		Vertex:   v,
		Children: make(map[int]*SimplexTreeNode),
	}
}

// SimplexTree maintains topological prefix hierarchy for O(k log N) boundary extraction.
type SimplexTree struct {
	Root map[int]*SimplexTreeNode
}

func NewSimplexTree() *SimplexTree {
	return &SimplexTree{
		Root: make(map[int]*SimplexTreeNode),
	}
}

func (st *SimplexTree) Insert(vertices []int, cell *Cell) {
	if len(vertices) == 0 {
		return
	}
	sorted := make([]int, len(vertices))
	copy(sorted, vertices)
	sort.Ints(sorted)

	currMap := st.Root
	var node *SimplexTreeNode
	for _, v := range sorted {
		if next, ok := currMap[v]; ok {
			node = next
		} else {
			node = NewSimplexTreeNode(v)
			currMap[v] = node
		}
		currMap = node.Children
	}
	if node != nil {
		node.Cell = cell
	}
}

func (st *SimplexTree) Find(vertices []int) *Cell {
	if len(vertices) == 0 {
		return nil
	}
	sorted := make([]int, len(vertices))
	copy(sorted, vertices)
	sort.Ints(sorted)

	currMap := st.Root
	var node *SimplexTreeNode
	for _, v := range sorted {
		if next, ok := currMap[v]; ok {
			node = next
			currMap = node.Children
		} else {
			return nil
		}
	}
	if node != nil {
		return node.Cell
	}
	return nil
}

// HodgeDecomposition represents the 3 orthogonal components of a 1-cochain (flow on edges).
type HodgeDecomposition struct {
	Gradient    StalkVector `json:"gradient"`    // d_0 alpha (acyclic / potential flow)
	Solenoidal  StalkVector `json:"solenoidal"`  // d_1^* beta (local boundary cycles / vortices)
	Harmonic    StalkVector `json:"harmonic"`    // in ker(Delta_1) (global non-bounding cycles / holes)
	CycleEnergy float64     `json:"cycle_energy"`// ||solenoidal + harmonic||^2
	TotalEnergy float64     `json:"total_energy"`// ||flow||^2
}

// SimplicialSheaf represents the topological complex and restriction sheaves.
type SimplicialSheaf struct {
	mu             sync.RWMutex
	maxDim         int
	stalkDim       int
	cells          map[int][]Cell            // dim -> cells
	cellIndex      map[string]int            // cellID -> index in dimension slice
	simplexTree    *SimplexTree              // prefix tree of simplices
	restrictions   map[string]Matrix         // "dim:lowID->highID" -> matrix (stalkDim x stalkDim)
	coboundary     map[int]Matrix            // k -> d_k matrix
	laplacians     map[int]Matrix            // k -> Delta_k matrix
	sparseLap      map[int]*SparseMatrix     // k -> sparse Delta_k matrix
	sparseCobound  map[int]*SparseMatrix     // k -> sparse d_k matrix
	dirty          bool
}

func NewSimplicialSheaf(maxDim, stalkDim int) *SimplicialSheaf {
	return &SimplicialSheaf{
		maxDim:        maxDim,
		stalkDim:      stalkDim,
		cells:         make(map[int][]Cell),
		cellIndex:     make(map[string]int),
		simplexTree:   NewSimplexTree(),
		restrictions:  make(map[string]Matrix),
		coboundary:    make(map[int]Matrix),
		laplacians:    make(map[int]Matrix),
		sparseLap:     make(map[int]*SparseMatrix),
		sparseCobound: make(map[int]*SparseMatrix),
		dirty:         true,
	}
}

func MakeCellID(vertices []int) string {
	sorted := make([]int, len(vertices))
	copy(sorted, vertices)
	sort.Ints(sorted)
	str := ""
	for i, v := range sorted {
		if i > 0 {
			str += "_"
		}
		str += fmt.Sprintf("%d", v)
	}
	return str
}

// AddCell adds a simplex to the complex.
func (s *SimplicialSheaf) AddCell(vertices []int) (Cell, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dim := len(vertices) - 1
	if dim > s.maxDim {
		return Cell{}, fmt.Errorf("dimension %d exceeds max %d", dim, s.maxDim)
	}

	sorted := make([]int, len(vertices))
	copy(sorted, vertices)
	sort.Ints(sorted)
	id := MakeCellID(sorted)

	if idx, exists := s.cellIndex[id]; exists {
		return s.cells[dim][idx], nil
	}

	c := Cell{Dimension: dim, ID: id, Vertices: sorted}
	s.cells[dim] = append(s.cells[dim], c)
	s.cellIndex[id] = len(s.cells[dim]) - 1
	s.simplexTree.Insert(sorted, &c)
	s.dirty = true
	return c, nil
}

// GetCellsByDim returns all simplices in dimension k.
func (s *SimplicialSheaf) GetCellsByDim(k int) []Cell {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]Cell, len(s.cells[k]))
	copy(res, s.cells[k])
	return res
}

// SetRestriction assigns a restriction operator rho: F(low) -> F(high).
func (s *SimplicialSheaf) SetRestriction(lowID, highID string, dim int, rho Matrix) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%d:%s->%s", dim, lowID, highID)
	s.restrictions[key] = rho
	s.dirty = true
}

// CompileOperators constructs coboundary operators d_k and Hodge Laplacians Delta_k.
func (s *SimplicialSheaf) CompileOperators() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return
	}

	sd := s.stalkDim

	for k := 0; k < s.maxDim; k++ {
		kCells := s.cells[k]
		k1Cells := s.cells[k+1]
		if len(kCells) == 0 || len(k1Cells) == 0 {
			continue
		}

		rows := len(k1Cells) * sd
		cols := len(kCells) * sd
		dK := NewMatrix(rows, cols)

		kCellMap := make(map[string]int)
		for idx, c := range kCells {
			kCellMap[c.ID] = idx
		}

		for highIdx, highCell := range k1Cells {
			for i := 0; i < len(highCell.Vertices); i++ {
				faceVerts := make([]int, 0, len(highCell.Vertices)-1)
				faceVerts = append(faceVerts, highCell.Vertices[:i]...)
				faceVerts = append(faceVerts, highCell.Vertices[i+1:]...)
				faceID := MakeCellID(faceVerts)

				lowIdx, exists := kCellMap[faceID]
				if !exists {
					continue
				}

				sign := 1.0
				if i%2 != 0 {
					sign = -1.0
				}

				key := fmt.Sprintf("%d:%s->%s", k, faceID, highCell.ID)
				rho, hasRho := s.restrictions[key]
				if !hasRho {
					// Default identity operator
					rho = NewMatrix(sd, sd)
					for d := 0; d < sd; d++ {
						rho[d][d] = 1.0
					}
				}

				rowOffset := highIdx * sd
				colOffset := lowIdx * sd
				for r := 0; r < sd; r++ {
					for c := 0; c < sd; c++ {
						dK[rowOffset+r][colOffset+c] += sign * rho[r][c]
					}
				}
			}
		}
		s.coboundary[k] = dK
		s.sparseCobound[k] = DenseToSparse(dK)
	}

	// Compute Hodge Laplacians: Delta_k = d_{k-1} d_{k-1}^* + d_k^* d_k
	for k := 0; k <= s.maxDim; k++ {
		kCells := s.cells[k]
		if len(kCells) == 0 {
			continue
		}
		totalDim := len(kCells) * sd
		delta := NewMatrix(totalDim, totalDim)

		// Downward part: d_{k-1} d_{k-1}^*
		if k > 0 {
			if dPrev, ok := s.coboundary[k-1]; ok && len(dPrev) == totalDim {
				dPrevT := MatTranspose(dPrev)
				down := MatMul(dPrev, dPrevT)
				for r := 0; r < totalDim; r++ {
					for c := 0; c < totalDim; c++ {
						delta[r][c] += down[r][c]
					}
				}
			}
		}

		// Upward part: d_k^* d_k
		if k < s.maxDim {
			if dCurr, ok := s.coboundary[k]; ok && len(dCurr[0]) == totalDim {
				dCurrT := MatTranspose(dCurr)
				up := MatMul(dCurrT, dCurr)
				for r := 0; r < totalDim; r++ {
					for c := 0; c < totalDim; c++ {
						delta[r][c] += up[r][c]
					}
				}
			}
		}
		s.laplacians[k] = delta
		s.sparseLap[k] = DenseToSparse(delta)
	}

	s.dirty = false
}

// ComputeDirichletEnergy returns E(x) = x^T Delta_k x with SIMD auto-vectorization.
func (s *SimplicialSheaf) ComputeDirichletEnergy(k int, x StalkVector) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sparse, ok := s.sparseLap[k]
	if ok && sparse.Rows == len(x) {
		sum := 0.0
		n := len(x)
		for r := 0; r < n; r++ {
			rowVal := 0.0
			for _, e := range sparse.Data[r] {
				rowVal += e.Val * x[e.Col]
			}
			sum += x[r] * rowVal
		}
		return sum
	}

	lap, ok2 := s.laplacians[k]
	if !ok2 || len(lap) != len(x) {
		return 0.0
	}
	dx := MatVecMul(lap, x)
	return Dot(x, dx)
}

// HeatDiffusionStepInto performs x_{t+1} = x_t - dt * Delta_k * x_t directly into out without allocation.
func (s *SimplicialSheaf) HeatDiffusionStepInto(k int, x, out StalkVector, dt float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(x)
	if len(out) < n {
		return
	}

	if sparse, ok := s.sparseLap[k]; ok && sparse.Rows == n {
		for r := 0; r < n; r++ {
			grad := 0.0
			for _, e := range sparse.Data[r] {
				grad += e.Val * x[e.Col]
			}
			out[r] = x[r] - dt*grad
		}
		return
	}

	lap, ok := s.laplacians[k]
	if !ok || len(lap) != n {
		copy(out, x)
		return
	}
	grad := MatVecMul(lap, x)
	i := 0
	for ; i+3 < n; i += 4 {
		out[i] = x[i] - dt*grad[i]
		out[i+1] = x[i+1] - dt*grad[i+1]
		out[i+2] = x[i+2] - dt*grad[i+2]
		out[i+3] = x[i+3] - dt*grad[i+3]
	}
	for ; i < n; i++ {
		out[i] = x[i] - dt*grad[i]
	}
}

// HeatDiffusionStep performs x_{t+1} = x_t - dt * Delta_k * x_t with sparse vector acceleration.
func (s *SimplicialSheaf) HeatDiffusionStep(k int, x StalkVector, dt float64) StalkVector {
	n := len(x)
	next := make(StalkVector, n)
	s.HeatDiffusionStepInto(k, x, next, dt)
	return next
}

// DecomposeFlow1 performs Hodge-Helmholtz decomposition of edge flow C^1:
// Flow = Gradient (d_0 alpha) + Solenoidal (d_1^* beta) + Harmonic (ker Delta_1).
func (s *SimplicialSheaf) DecomposeFlow1(flow StalkVector, maxSteps int, dt float64) HodgeDecomposition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	edges := s.cells[1]
	total1Dim := len(edges) * s.stalkDim
	if len(flow) != total1Dim || total1Dim == 0 {
		return HodgeDecomposition{Gradient: flow}
	}

	// 1. Extract harmonic component via diffusion exp(-t * Delta_1)
	harmonic := CloneVector(flow)
	sparse1, hasSparse1 := s.sparseLap[1]
	if hasSparse1 && sparse1.Rows == total1Dim {
		for step := 0; step < maxSteps; step++ {
			for r := 0; r < total1Dim; r++ {
				grad := 0.0
				for _, e := range sparse1.Data[r] {
					grad += e.Val * harmonic[e.Col]
				}
				harmonic[r] -= dt * grad
			}
		}
	} else if lap1, hasLap1 := s.laplacians[1]; hasLap1 && len(lap1) == total1Dim {
		for step := 0; step < maxSteps; step++ {
			grad := MatVecMul(lap1, harmonic)
			for i := range harmonic {
				harmonic[i] -= dt * grad[i]
			}
		}
	}

	// Non-harmonic part: w = flow - harmonic = d_0 alpha + d_1^* beta
	w := make(StalkVector, total1Dim)
	for i := range flow {
		w[i] = flow[i] - harmonic[i]
	}

	// 2. Extract gradient component: project onto im(d_0)
	// d_0: C^0 -> C^1. d_0^*: C^1 -> C^0. Delta_0_down = d_0^* d_0.
	d0, hasD0 := s.coboundary[0]
	gradient := make(StalkVector, total1Dim)
	if hasD0 {
		d0T := MatTranspose(d0) // (C^0 x C^1)
		// RHS = d_0^* w
		rhs := MatVecMul(d0T, w)
		// Solve Delta_0 alpha = rhs via gradient descent
		delta0 := s.laplacians[0]
		if len(delta0) == len(rhs) {
			alpha := make(StalkVector, len(rhs))
			sparse0, hasSparse0 := s.sparseLap[0]
			for it := 0; it < maxSteps; it++ {
				if hasSparse0 && sparse0.Rows == len(alpha) {
					for r := range alpha {
						cur := 0.0
						for _, e := range sparse0.Data[r] {
							cur += e.Val * alpha[e.Col]
						}
						alpha[r] += dt * (rhs[r] - cur)
					}
				} else {
					cur := MatVecMul(delta0, alpha)
					for i := range alpha {
						alpha[i] += dt * (rhs[i] - cur[i])
					}
				}
			}
			gradient = MatVecMul(d0, alpha)
		}
	}

	// 3. Solenoidal component: solenoidal = w - gradient
	solenoidal := make(StalkVector, total1Dim)
	for i := range total1Dim {
		solenoidal[i] = w[i] - gradient[i]
	}

	totalE := Dot(flow, flow)
	cycleVec := make(StalkVector, total1Dim)
	for i := range total1Dim {
		cycleVec[i] = solenoidal[i] + harmonic[i]
	}
	cycleE := Dot(cycleVec, cycleVec)

	return HodgeDecomposition{
		Gradient:    gradient,
		Solenoidal:  solenoidal,
		Harmonic:    harmonic,
		CycleEnergy: cycleE,
		TotalEnergy: totalE,
	}
}

// ComputeBettiNumbers estimates topological Betti numbers (dim ker Delta_k).
func (s *SimplicialSheaf) ComputeBettiNumbers(tol float64) (betti0 int, betti1 int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if l0, ok := s.laplacians[0]; ok && len(l0) > 0 {
		betti0 = countZeroSingularModes(l0, tol)
	}
	if l1, ok := s.laplacians[1]; ok && len(l1) > 0 {
		betti1 = countZeroSingularModes(l1, tol)
	}
	return betti0, betti1
}

// gaussianBuffer encapsulates reusable memory buffers for Gaussian elimination rank computation.
type gaussianBuffer struct {
	matrix []float64
	rows   [][]float64
}

var gaussianPool = sync.Pool{
	New: func() interface{} {
		return &gaussianBuffer{
			matrix: make([]float64, 0, 1024),
			rows:   make([][]float64, 0, 64),
		}
	},
}

// ComputeHigherBettiNumbersInto computes exact or bounded Betti numbers H^0, H^1, ..., H^maxDim
// writing results into out slice without allocations.
func (s *SimplicialSheaf) ComputeHigherBettiNumbersInto(out []int, tol float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k := 0; k <= s.maxDim && k < len(out); k++ {
		if lap, ok := s.laplacians[k]; ok && len(lap) > 0 {
			out[k] = countZeroSingularModes(lap, tol)
		} else {
			out[k] = 0
		}
	}
}

// ComputeHigherBettiNumbers computes exact or bounded Betti numbers H^0, H^1, ..., H^maxDim.
func (s *SimplicialSheaf) ComputeHigherBettiNumbers(tol float64) map[int]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[int]int, s.maxDim+1)
	for k := 0; k <= s.maxDim; k++ {
		if lap, ok := s.laplacians[k]; ok && len(lap) > 0 {
			res[k] = countZeroSingularModes(lap, tol)
		} else {
			res[k] = 0
		}
	}
	return res
}

func countZeroSingularModes(m Matrix, tol float64) int {
	n := len(m)
	if n == 0 {
		return 0
	}
	cols := len(m[0])
	totalElements := n * cols

	buf := gaussianPool.Get().(*gaussianBuffer)
	if cap(buf.matrix) < totalElements {
		buf.matrix = make([]float64, totalElements)
	} else {
		buf.matrix = buf.matrix[:totalElements]
	}
	if cap(buf.rows) < n {
		buf.rows = make([][]float64, n)
	} else {
		buf.rows = buf.rows[:n]
	}

	A := buf.rows
	flat := buf.matrix
	for i := 0; i < n; i++ {
		start := i * cols
		rowSlice := flat[start : start+cols]
		copy(rowSlice, m[i])
		A[i] = rowSlice
	}

	rank := 0
	for col := 0; col < cols && rank < n; col++ {
		// Find pivot
		pivotRow := rank
		maxVal := math.Abs(A[pivotRow][col])
		for r := rank + 1; r < n; r++ {
			if val := math.Abs(A[r][col]); val > maxVal {
				maxVal = val
				pivotRow = r
			}
		}

		if maxVal < tol {
			continue
		}

		// Swap rows
		if pivotRow != rank {
			A[pivotRow], A[rank] = A[rank], A[pivotRow]
		}

		// Eliminate below
		pivot := A[rank][col]
		pivotRowSlice := A[rank]
		for r := rank + 1; r < n; r++ {
			curRow := A[r]
			factor := curRow[col] / pivot
			if factor != 0.0 {
				c := col
				for ; c+3 < cols; c += 4 {
					curRow[c] -= factor * pivotRowSlice[c]
					curRow[c+1] -= factor * pivotRowSlice[c+1]
					curRow[c+2] -= factor * pivotRowSlice[c+2]
					curRow[c+3] -= factor * pivotRowSlice[c+3]
				}
				for ; c < cols; c++ {
					curRow[c] -= factor * pivotRowSlice[c]
				}
			}
		}
		rank++
	}

	gaussianPool.Put(buf)

	kernelDim := cols - rank
	if kernelDim < 0 {
		return 0
	}
	return kernelDim
}

// SimplicialRAGDiffusion spreads query activation across simplices in k steps
// returning ranked node activations for LLM context retrieval.
func (s *SimplicialSheaf) SimplicialRAGDiffusion(seedNodeIDs []int, steps int, alpha float64) map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vCells := s.cells[0]
	if len(vCells) == 0 {
		return nil
	}

	sd := s.stalkDim
	totalDim := len(vCells) * sd
	state := make(StalkVector, totalDim)

	// Seed activation in initial nodes
	seedSet := make(map[int]bool)
	for _, id := range seedNodeIDs {
		seedSet[id] = true
	}

	for idx, c := range vCells {
		if len(c.Vertices) > 0 && seedSet[c.Vertices[0]] {
			for d := 0; d < sd; d++ {
				state[idx*sd+d] = 1.0
			}
		}
	}

	sparse0, hasSparse0 := s.sparseLap[0]
	if hasSparse0 && sparse0.Rows == totalDim {
		for st := 0; st < steps; st++ {
			for r := 0; r < totalDim; r++ {
				grad := 0.0
				for _, e := range sparse0.Data[r] {
					grad += e.Val * state[e.Col]
				}
				state[r] = math.Max(0.0, state[r]-alpha*grad)
			}
		}
	} else if lap0, hasLap0 := s.laplacians[0]; hasLap0 && len(lap0) == totalDim {
		for st := 0; st < steps; st++ {
			grad := MatVecMul(lap0, state)
			for i := range state {
				state[i] = math.Max(0.0, state[i]-alpha*grad[i])
			}
		}
	}

	// Aggregate activation per cell
	scores := make(map[string]float64)
	for idx, c := range vCells {
		mag := 0.0
		for d := 0; d < sd; d++ {
			val := state[idx*sd+d]
			mag += val * val
		}
		scores[c.ID] = math.Sqrt(mag)
	}

	return scores
}

// CheckCoboundaryComposition verifies d_{k+1} o d_k == 0.
func (s *SimplicialSheaf) CheckCoboundaryComposition(k int) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d0, ok0 := s.coboundary[k]
	d1, ok1 := s.coboundary[k+1]
	if !ok0 || !ok1 {
		return 0.0
	}
	comp := MatMul(d1, d0)
	maxErr := 0.0
	for r := range comp {
		for c := range comp[r] {
			if abs := math.Abs(comp[r][c]); abs > maxErr {
				maxErr = abs
			}
		}
	}
	return maxErr
}

// StalkDim returns stalk vector dimension.
func (s *SimplicialSheaf) StalkDim() int {
	return s.stalkDim
}

// MaxDim returns maximum simplex dimension.
func (s *SimplicialSheaf) MaxDim() int {
	return s.maxDim
}
