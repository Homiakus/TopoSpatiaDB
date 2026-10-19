package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"topospatia"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8089"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".topospatia_data")
	}

	log.Printf("Initializing TopoSpatiaDB (Multidimensional Sheaf Holographic DBMS)...")

	// Create 2-simplex sheaf (vertices, edges, triangles) with 2D stalks
	sheaf := topospatia.NewSimplicialSheaf(2, 2)

	// Seed an initial topological simplicial network: triangle 0-1-2
	v0, _ := sheaf.AddCell([]int{0})
	v1, _ := sheaf.AddCell([]int{1})
	v2, _ := sheaf.AddCell([]int{2})
	sheaf.AddCell([]int{0, 1})
	sheaf.AddCell([]int{1, 2})
	sheaf.AddCell([]int{0, 2})
	sheaf.AddCell([]int{0, 1, 2})

	sheaf.CompileOperators()

	storage, err := topospatia.OpenStorage(dataDir, sheaf)
	if err != nil {
		log.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()

	// Seed initial records if empty
	if len(storage.AllRecords()) == 0 {
		_ = storage.Put(&topospatia.Record{
			ID:        "rec-v0",
			CellID:    v0.ID,
			Dimension: 0,
			Vector:    topospatia.StalkVector{10.0, 5.0},
			Payload:   map[string]interface{}{"entity": "Node Alpha", "status": "primary"},
		})
		_ = storage.Put(&topospatia.Record{
			ID:        "rec-v1",
			CellID:    v1.ID,
			Dimension: 0,
			Vector:    topospatia.StalkVector{8.0, 7.0},
			Payload:   map[string]interface{}{"entity": "Node Beta", "status": "replica"},
		})
		_ = storage.Put(&topospatia.Record{
			ID:        "rec-v2",
			CellID:    v2.ID,
			Dimension: 0,
			Vector:    topospatia.StalkVector{12.0, 3.0},
			Payload:   map[string]interface{}{"entity": "Node Gamma", "status": "replica"},
		})
	}

	consensus := topospatia.NewConsensusEngine(sheaf, storage)
	srv := topospatia.NewServer(sheaf, storage, consensus)

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("TopoSpatiaDB active and listening on http://%s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
