package topospatia

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// Single-threaded benchmarks
func BenchmarkStorageEnginePut(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "topospatia_bench_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sheaf := NewSimplicialSheaf(1, 2)
	v0, _ := sheaf.AddCell([]int{0})
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := &Record{
			ID:        fmt.Sprintf("bench-rec-%d", i),
			CellID:    v0.ID,
			Dimension: 0,
			Vector:    StalkVector{float64(i), float64(i * 2)},
			Payload:   map[string]interface{}{"iter": i, "val": "benchmark payload"},
		}
		if err := storage.Put(rec); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStorageEngineGet(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "topospatia_bench_get_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sheaf := NewSimplicialSheaf(1, 2)
	v0, _ := sheaf.AddCell([]int{0})
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	const numKeys = 1000
	for i := 0; i < numKeys; i++ {
		_ = storage.Put(&Record{
			ID:        fmt.Sprintf("key-%d", i),
			CellID:    v0.ID,
			Dimension: 0,
			Vector:    StalkVector{float64(i), 1.0},
			Payload:   map[string]interface{}{"v": i},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := rand.Intn(numKeys)
		_, err := storage.Get(fmt.Sprintf("key-%d", k))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSheafHodgeDiffusion(b *testing.B) {
	sheaf := NewSimplicialSheaf(2, 2)
	for i := 0; i < 10; i++ {
		sheaf.AddCell([]int{i})
	}
	for i := 0; i < 9; i++ {
		sheaf.AddCell([]int{i, i + 1})
	}
	sheaf.AddCell([]int{9, 0})
	sheaf.CompileOperators()

	x := make(StalkVector, 20)
	for i := range x {
		x[i] = float64(i%5 + 1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x = sheaf.HeatDiffusionStep(0, x, 0.05)
	}
}

func BenchmarkSheafDirichletEnergy(b *testing.B) {
	sheaf := NewSimplicialSheaf(2, 2)
	for i := 0; i < 10; i++ {
		sheaf.AddCell([]int{i})
	}
	for i := 0; i < 9; i++ {
		sheaf.AddCell([]int{i, i + 1})
	}
	sheaf.AddCell([]int{9, 0})
	sheaf.CompileOperators()

	x := make(StalkVector, 20)
	for i := range x {
		x[i] = float64(i*3 + 1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sheaf.ComputeDirichletEnergy(0, x)
	}
}

// -------------------------------------------------------------
// MULTI-THREADED / CONCURRENT BENCHMARKS (Parallel sysbench style)
// -------------------------------------------------------------

func BenchmarkParallelStorageEngineGet(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "topospatia_par_get_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sheaf := NewSimplicialSheaf(1, 2)
	v0, _ := sheaf.AddCell([]int{0})
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	const numKeys = 2000
	for i := 0; i < numKeys; i++ {
		_ = storage.Put(&Record{
			ID:        fmt.Sprintf("pkey-%d", i),
			CellID:    v0.ID,
			Dimension: 0,
			Vector:    StalkVector{float64(i), 3.14},
			Payload:   map[string]interface{}{"val": i, "tag": "bench"},
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			k := (idx * 37) % numKeys
			_, err := storage.Get(fmt.Sprintf("pkey-%d", k))
			if err != nil {
				b.Fatal(err)
			}
			idx++
		}
	})
}

func BenchmarkParallelStorageEnginePut(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "topospatia_par_put_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sheaf := NewSimplicialSheaf(1, 2)
	v0, _ := sheaf.AddCell([]int{0})
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	var counter uint64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			val := atomic.AddUint64(&counter, 1)
			rec := &Record{
				ID:        fmt.Sprintf("par-rec-%d", val),
				CellID:    v0.ID,
				Dimension: 0,
				Vector:    StalkVector{float64(val), float64(val % 10)},
				Payload:   map[string]interface{}{"seq": val},
			}
			if err := storage.Put(rec); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParallelReadWriteMixed(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "topospatia_par_mixed_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sheaf := NewSimplicialSheaf(1, 2)
	v0, _ := sheaf.AddCell([]int{0})
	sheaf.CompileOperators()

	storage, err := OpenStorage(filepath.Join(tmpDir, "data"), sheaf)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	const seedKeys = 1000
	for i := 0; i < seedKeys; i++ {
		_ = storage.Put(&Record{
			ID:        fmt.Sprintf("mkey-%d", i),
			CellID:    v0.ID,
			Dimension: 0,
			Vector:    StalkVector{float64(i), 1.0},
			Payload:   map[string]interface{}{"v": i},
		})
	}

	var writeCounter uint64

	b.ResetTimer()
	// 80% Read / 20% Write (standard OLTP profile)
	b.RunParallel(func(pb *testing.PB) {
		step := 0
		for pb.Next() {
			step++
			if step%5 == 0 {
				wID := atomic.AddUint64(&writeCounter, 1)
				rec := &Record{
					ID:        fmt.Sprintf("new-mkey-%d", wID),
					CellID:    v0.ID,
					Dimension: 0,
					Vector:    StalkVector{float64(wID), float64(wID)},
					Payload:   map[string]interface{}{"w": wID},
				}
				_ = storage.Put(rec)
			} else {
				k := (step * 31) % seedKeys
				_, _ = storage.Get(fmt.Sprintf("mkey-%d", k))
			}
		}
	})
}

func BenchmarkParallelSheafHodgeDiffusion(b *testing.B) {
	sheaf := NewSimplicialSheaf(2, 2)
	for i := 0; i < 16; i++ {
		sheaf.AddCell([]int{i})
	}
	for i := 0; i < 15; i++ {
		sheaf.AddCell([]int{i, i + 1})
	}
	sheaf.AddCell([]int{15, 0})
	sheaf.CompileOperators()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		localX := make(StalkVector, 32)
		for i := range localX {
			localX[i] = float64(i*2 + 1)
		}
		for pb.Next() {
			localX = sheaf.HeatDiffusionStep(0, localX, 0.05)
		}
	})
}
