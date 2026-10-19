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
	for i := range a {
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
		sum := 0.0
		for c := 0; c < cols; c++ {
			sum += m[r][c] * v[c]
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
		for k := 0; k < cA; k++ {
			if a[i][k] != 0.0 {
				for j := 0; j < cB; j++ {
					res[i][j] += a[i][k] * b[k][j]
				}
			}
		}
	}
	return res
}

// SimplicialSheaf represents the topological complex and restriction sheaves.
type SimplicialSheaf struct {
	mu           sync.RWMutex
	maxDim       int
	stalkDim     int
	cells        map[int][]Cell            // dim -> cells
	cellIndex    map[string]int            // cellID -> index in dimension slice
	restrictions map[string]Matrix         // "dim:lowID->highID" -> matrix (stalkDim x stalkDim)
	coboundary   map[int]Matrix            // k -> d_k matrix
	laplacians   map[int]Matrix            // k -> Delta_k matrix
	dirty        bool
}

func NewSimplicialSheaf(maxDim, stalkDim int) *SimplicialSheaf {
	return &SimplicialSheaf{
		maxDim:       maxDim,
		stalkDim:     stalkDim,
		cells:        make(map[int][]Cell),
		cellIndex:    make(map[string]int),
		restrictions: make(map[string]Matrix),
		coboundary:   make(map[int]Matrix),
		laplacians:   make(map[int]Matrix),
		dirty:        true,
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
	}

	s.dirty = false
}

// ComputeDirichletEnergy returns E(x) = x^T Delta_k x.
func (s *SimplicialSheaf) ComputeDirichletEnergy(k int, x StalkVector) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lap, ok := s.laplacians[k]
	if !ok || len(lap) != len(x) {
		return 0.0
	}
	dx := MatVecMul(lap, x)
	return Dot(x, dx)
}

// HeatDiffusionStep performs x_{t+1} = x_t - dt * Delta_k * x_t.
func (s *SimplicialSheaf) HeatDiffusionStep(k int, x StalkVector, dt float64) StalkVector {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lap, ok := s.laplacians[k]
	if !ok || len(lap) != len(x) {
		return x
	}
	grad := MatVecMul(lap, x)
	next := make(StalkVector, len(x))
	for i := range x {
		next[i] = x[i] - dt*grad[i]
	}
	return next
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
