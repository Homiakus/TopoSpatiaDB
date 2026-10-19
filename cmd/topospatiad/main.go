package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

	opts := topospatia.DefaultOptions(dataDir)
	db, err := topospatia.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open TopoSpatiaDB: %v", err)
	}
	defer db.Close()

	// Seed an initial topological simplicial network: triangle 0-1-2
	v0, _ := db.AddCell([]int{0})
	v1, _ := db.AddCell([]int{1})
	v2, _ := db.AddCell([]int{2})
	db.AddCell([]int{0, 1})
	db.AddCell([]int{1, 2})
	db.AddCell([]int{0, 2})
	db.AddCell([]int{0, 1, 2})
	db.CompileOperators()

	// Seed initial records if empty
	if len(db.AllRecords()) == 0 {
		_ = db.Put(&topospatia.Record{
			ID:        "rec-v0",
			CellID:    v0.ID,
			Dimension: 0,
			Vector:    topospatia.StalkVector{10.0, 5.0},
			Payload:   map[string]interface{}{"entity": "Node Alpha", "status": "primary"},
		})
		_ = db.Put(&topospatia.Record{
			ID:        "rec-v1",
			CellID:    v1.ID,
			Dimension: 0,
			Vector:    topospatia.StalkVector{8.0, 7.0},
			Payload:   map[string]interface{}{"entity": "Node Beta", "status": "replica"},
		})
		_ = db.Put(&topospatia.Record{
			ID:        "rec-v2",
			CellID:    v2.ID,
			Dimension: 0,
			Vector:    topospatia.StalkVector{12.0, 3.0},
			Payload:   map[string]interface{}{"entity": "Node Gamma", "status": "replica"},
		})
	}

	srv := topospatia.NewServer(db.Sheaf(), db.Storage(), db.Consensus())

	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("TopoSpatiaDB active and listening on http://%s", addr)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
