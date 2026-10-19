package topospatia

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCoboundaryComposition(t *testing.T) {
	sheaf := NewSimplicialSheaf(2, 2)

	// Triangle {0, 1, 2}
	_, err := sheaf.AddCell([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	sheaf.AddCell([]int{1})
	sheaf.AddCell([]int{2})
	sheaf.AddCell([]int{0, 1})
	sheaf.AddCell([]int{1, 2})
	sheaf.AddCell([]int{0, 2})
	sheaf.AddCell([]int{0, 1, 2})

	sheaf.CompileOperators()

	// Verify coboundary nilpotency d_{k+1} o d_k == 0
	errCoboundary := sheaf.CheckCoboundaryComposition(0)
	if errCoboundary > 1e-9 {
		t.Fatalf("Coboundary composition d_1 o d_0 != 0, residual: %e", errCoboundary)
	}
}

func TestHodgeLaplacianDirichletEnergy(t *testing.T) {
	sheaf := NewSimplicialSheaf(2, 1)

	sheaf.AddCell([]int{0})
	sheaf.AddCell([]int{1})
	sheaf.AddCell([]int{0, 1})

	sheaf.CompileOperators()

	xHarmonic := StalkVector{5.0, 5.0}
	eHarmonic := sheaf.ComputeDirichletEnergy(0, xHarmonic)
	if math.Abs(eHarmonic) > 1e-9 {
		t.Fatalf("Expected zero Dirichlet energy for harmonic section, got %f", eHarmonic)
	}

	xConflict := StalkVector{10.0, 0.0}
	eConflict := sheaf.ComputeDirichletEnergy(0, xConflict)
	if eConflict <= 0 {
		t.Fatalf("Expected positive Dirichlet energy for conflict, got %f", eConflict)
	}

	diffused := sheaf.HeatDiffusionStep(0, xConflict, 0.1)
	eDiffused := sheaf.ComputeDirichletEnergy(0, diffused)
	if eDiffused >= eConflict {
		t.Fatalf("Heat diffusion did not reduce Dirichlet energy: before=%f, after=%f", eConflict, eDiffused)
	}
}

func TestStorageAndConsensus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "topospatia_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sheaf := NewSimplicialSheaf(2, 2)
	v0, _ := sheaf.AddCell([]int{0})
	v1, _ := sheaf.AddCell([]int{1})
	sheaf.AddCell([]int{0, 1})
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	rec0 := &Record{
		ID:        "rec-0",
		CellID:    v0.ID,
		Dimension: 0,
		Vector:    StalkVector{10.0, 2.0},
		Payload:   map[string]interface{}{"title": "Alpha"},
	}
	rec1 := &Record{
		ID:        "rec-1",
		CellID:    v1.ID,
		Dimension: 0,
		Vector:    StalkVector{2.0, 10.0},
		Payload:   map[string]interface{}{"title": "Beta"},
	}

	if err := storage.Put(rec0); err != nil {
		t.Fatal(err)
	}
	if err := storage.Put(rec1); err != nil {
		t.Fatal(err)
	}

	consensus := NewConsensusEngine(sheaf, storage)
	report, err := consensus.DiffuseGlobal0CoChain()
	if err != nil {
		t.Fatal(err)
	}

	if !report.Converged {
		t.Fatalf("Consensus did not converge: %+v", report)
	}

	if report.FinalEnergy >= report.InitialEnergy {
		t.Fatalf("Consensus failed to lower energy: init=%f, final=%f", report.InitialEnergy, report.FinalEnergy)
	}
}

func TestHodgeHelmholtzDecomposition(t *testing.T) {
	sheaf := NewSimplicialSheaf(2, 1)

	// Triangle {0, 1, 2} with 3 edges
	sheaf.AddCell([]int{0})
	sheaf.AddCell([]int{1})
	sheaf.AddCell([]int{2})
	sheaf.AddCell([]int{0, 1})   // edge 0
	sheaf.AddCell([]int{0, 2})   // edge 1
	sheaf.AddCell([]int{1, 2})   // edge 2
	sheaf.AddCell([]int{0, 1, 2}) // face
	sheaf.CompileOperators()

	// Circular flow on edges: 1 -> 2 -> 0
	flow := StalkVector{1.0, -1.0, 1.0}
	decomp := sheaf.DecomposeFlow1(flow, 100, 0.05)

	if decomp.TotalEnergy <= 0 {
		t.Fatalf("Invalid total energy: %f", decomp.TotalEnergy)
	}
	if len(decomp.Gradient) != 3 || len(decomp.Solenoidal) != 3 || len(decomp.Harmonic) != 3 {
		t.Fatalf("Decomposition dimension mismatch: %+v", decomp)
	}
}

func TestSimplicialRAGDiffusion(t *testing.T) {
	sheaf := NewSimplicialSheaf(2, 1)

	// Path 0 - 1 - 2 - 3
	for i := 0; i < 4; i++ {
		sheaf.AddCell([]int{i})
	}
	sheaf.AddCell([]int{0, 1})
	sheaf.AddCell([]int{1, 2})
	sheaf.AddCell([]int{2, 3})
	sheaf.CompileOperators()

	// Query starts at node 0
	rankings := sheaf.SimplicialRAGDiffusion([]int{0}, 3, 0.1)
	if len(rankings) != 4 {
		t.Fatalf("Expected 4 node rankings, got %d", len(rankings))
	}
	// Node 0 should have highest activation, decreasing along path
	if rankings["0"] <= rankings["3"] {
		t.Fatalf("Expected diffusion decay from node 0 to 3: %+v", rankings)
	}
}

func TestHigherBettiSphereAndSimplexTree(t *testing.T) {
	// Octahedron S^2 hollow sphere (6 vertices, 12 edges, 8 triangles)
	sheaf := NewSimplicialSheaf(2, 1)
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

	// Verify SimplexTree prefix lookup
	for _, f := range faces {
		cell := sheaf.simplexTree.Find(f)
		if cell == nil {
			t.Fatalf("SimplexTree failed to find face %+v", f)
		}
	}

	// Verify Higher Betti numbers
	bettiMap := sheaf.ComputeHigherBettiNumbers(1e-5)
	if bettiMap[0] != 1 {
		t.Fatalf("Expected Betti_0 = 1 for S^2, got %d", bettiMap[0])
	}
	if bettiMap[1] != 0 {
		t.Fatalf("Expected Betti_1 = 0 for S^2, got %d", bettiMap[1])
	}
	if bettiMap[2] != 1 {
		t.Fatalf("Expected Betti_2 = 1 for S^2 cavity, got %d", bettiMap[2])
	}
}

func TestEmbeddedDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "topospatia_embed_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := OpenDefault(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open embedded DB: %v", err)
	}
	defer db.Close()

	// 1. Add topology
	v0, err := db.AddCell([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := db.AddCell([]int{1})
	_, _ = db.AddCell([]int{0, 1})
	db.CompileOperators()

	// 2. Put records
	err = db.Put(&Record{
		ID:        "node-0",
		CellID:    v0.ID,
		Dimension: 0,
		Vector:    StalkVector{10.0, 5.0},
		Payload:   map[string]interface{}{"role": "leader"},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	err = db.Put(&Record{
		ID:        "node-1",
		CellID:    v1.ID,
		Dimension: 0,
		Vector:    StalkVector{2.0, 8.0},
		Payload:   map[string]interface{}{"role": "follower"},
	})
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 3. Get & Query
	rec, err := db.Get("node-0")
	if err != nil || rec.ID != "node-0" {
		t.Fatalf("Get failed: %v, got %+v", err, rec)
	}

	recs, err := db.QueryByCell(v0.ID)
	if err != nil || len(recs) != 1 {
		t.Fatalf("QueryByCell failed: %v, count=%d", err, len(recs))
	}

	// 4. Run consensus
	report, err := db.Diffuse()
	if err != nil {
		t.Fatalf("Diffuse failed: %v", err)
	}
	if !report.Converged {
		t.Fatalf("Expected convergence, got %+v", report)
	}

	// 5. Test Higher Betti numbers (stalkDim=2, connected segment -> dim ker(Delta_0) = 1 * 2 = 2)
	betti := db.ComputeHigherBettiNumbers(1e-5)
	if betti[0] != 2 {
		t.Fatalf("Expected Betti 0 = 2 for connected segment with stalkDim=2, got %d", betti[0])
	}
}
