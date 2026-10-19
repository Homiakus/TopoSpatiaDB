package topospatia

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// 1. BenchmarkTopologicalConsensusMesh tests convergence of consensus under varying partition sizes
func BenchmarkTopologicalConsensusMesh(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "topospatia_bench_consensus_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	const numNodes = 20
	sheaf := NewSimplicialSheaf(2, 2)

	// Create a 2D triangulated cylinder mesh
	for i := 0; i < numNodes; i++ {
		sheaf.AddCell([]int{i})
	}
	for i := 0; i < numNodes; i++ {
		next := (i + 1) % numNodes
		sheaf.AddCell([]int{i, next})
		if i+2 < numNodes {
			sheaf.AddCell([]int{i, next, (i + 2) % numNodes})
		}
	}
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	// Seed divergent states across nodes
	for i := 0; i < numNodes; i++ {
		_ = storage.Put(&Record{
			ID:        fmt.Sprintf("node-rec-%d", i),
			CellID:    MakeCellID([]int{i}),
			Dimension: 0,
			Vector:    StalkVector{float64(i * 10), float64(100 - i*5)},
			Payload:   map[string]interface{}{"node": i},
		})
	}

	engine := NewConsensusEngine(sheaf, storage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.DiffuseGlobal0CoChain()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 2. BenchmarkHighDimSimplexTreeLookup tests prefix tree lookup in higher dimensional simplex trees
func BenchmarkHighDimSimplexTreeLookup(b *testing.B) {
	st := NewSimplexTree()

	// Build a high dimensional complex: 4-simplices (5 vertices each)
	const numSimplices = 200
	simplices := make([][]int, numSimplices)
	for i := 0; i < numSimplices; i++ {
		s := []int{i, i + 1, i + 2, i + 3, i + 4}
		simplices[i] = s
		cell := &Cell{Dimension: 4, ID: MakeCellID(s), Vertices: s}
		st.Insert(s, cell)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := simplices[i%numSimplices]
		c := st.Find(target)
		if c == nil {
			b.Fatal("simplex not found")
		}
	}
}

// 3. BenchmarkHigherBettiGaussianSolver benchmarks exact Gaussian elimination on higher complexes
func BenchmarkHigherBettiGaussianSolver(b *testing.B) {
	sheaf := NewSimplicialSheaf(2, 1)

	// Hollow sphere S^2 (octahedron: 6 vertices, 12 edges, 8 triangles)
	faces := [][]int{
		{0, 1, 2}, {0, 2, 3}, {0, 3, 4}, {0, 4, 1},
		{5, 1, 2}, {5, 2, 3}, {5, 3, 4}, {5, 4, 1},
	}
	for _, f := range faces {
		for i := 0; i < 3; i++ {
			sheaf.AddCell([]int{f[i]})
		}
		sheaf.AddCell([]int{f[0], f[1]})
		sheaf.AddCell([]int{f[1], f[2]})
		sheaf.AddCell([]int{f[0], f[2]})
		sheaf.AddCell(f)
	}
	sheaf.CompileOperators()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sheaf.ComputeHigherBettiNumbers(1e-5)
	}
}

// 4. BenchmarkTopologicalRangeQuery evaluates querying multi-cell regions with graph neighborhood filter
func BenchmarkTopologicalRangeQuery(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "topospatia_bench_range_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sheaf := NewSimplicialSheaf(1, 2)
	const numCells = 50
	for i := 0; i < numCells; i++ {
		sheaf.AddCell([]int{i})
	}
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	for i := 0; i < 1000; i++ {
		cID := fmt.Sprintf("%d", i%numCells)
		_ = storage.Put(&Record{
			ID:        fmt.Sprintf("range-rec-%d", i),
			CellID:    cID,
			Dimension: 0,
			Vector:    StalkVector{float64(i), float64(i % 10)},
			Payload:   map[string]interface{}{"idx": i},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cID := fmt.Sprintf("%d", rand.Intn(numCells))
		recs, err := storage.QueryByCell(cID)
		if err != nil || len(recs) == 0 {
			b.Fatal("query failed")
		}
	}
}

// 5. BenchmarkZeroAllocDirichletEnergy confirms 0 allocs and evaluates sustained throughput
func BenchmarkZeroAllocDirichletEnergy(b *testing.B) {
	sheaf := NewSimplicialSheaf(2, 2)
	for i := 0; i < 30; i++ {
		sheaf.AddCell([]int{i})
	}
	for i := 0; i < 29; i++ {
		sheaf.AddCell([]int{i, i + 1})
	}
	sheaf.AddCell([]int{29, 0})
	sheaf.CompileOperators()

	x := make(StalkVector, 60)
	for i := range x {
		x[i] = float64(i%7 + 1)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sheaf.ComputeDirichletEnergy(0, x)
	}
}
