package topospatia

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCoboundaryComposition(t *testing.T) {
	// Create a 2D simplicial complex: vertices, edges, triangles
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

	// Simple edge between 0 and 1
	sheaf.AddCell([]int{0})
	sheaf.AddCell([]int{1})
	sheaf.AddCell([]int{0, 1})

	sheaf.CompileOperators()

	// Equal stalk values -> zero Dirichlet energy (harmonic 0-cochain)
	xHarmonic := StalkVector{5.0, 5.0}
	eHarmonic := sheaf.ComputeDirichletEnergy(0, xHarmonic)
	if math.Abs(eHarmonic) > 1e-9 {
		t.Fatalf("Expected zero Dirichlet energy for harmonic section, got %f", eHarmonic)
	}

	// Disagreeing stalk values -> non-zero Dirichlet energy (cohomological conflict)
	xConflict := StalkVector{10.0, 0.0}
	eConflict := sheaf.ComputeDirichletEnergy(0, xConflict)
	if eConflict <= 0 {
		t.Fatalf("Expected positive Dirichlet energy for conflict, got %f", eConflict)
	}

	// Test heat diffusion step reduces energy
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
