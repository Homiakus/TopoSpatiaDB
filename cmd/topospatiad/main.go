package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"topospatia"
)

const (
	Version   = "1.2.0"
	BannerStr = `
  _____                 ____              _   _       ____  ____  
 |_   _|___  _ __   ___/ ___| _ __   __ _| |_(_) __ _|  _ \| __ ) 
   | | / _ \| '_ \ / _ \___ \| '_ \ / _` + "`" + ` | __| |/ _` + "`" + ` | | | |  _ \ 
   | || (_) | |_) | (_) |__) | |_) | (_| | |_| | (_| | |_| | |_) |
   |_| \___/| .__/ \___/____/| .__/ \__,_|\__|_|\__,_|____/|____/ 
            |_|              |_|                                  
   Multidimensional Sheaf Holographic Database Engine
`
)

func defaultDataDir() string {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			dataDir = ".topospatia_data"
		} else {
			dataDir = filepath.Join(home, ".topospatia_data")
		}
	}
	return dataDir
}

func printUsage() {
	fmt.Printf("%s\nTopoSpatiaDB %s — High-Performance Multidimensional Sheaf Holographic DBMS\n\n", BannerStr, Version)
	fmt.Println("USAGE:")
	fmt.Println("  topospatiad [command] [flags]")
	fmt.Println("\nCOMMANDS:")
	fmt.Println("  server              Start the TopoSpatiaDB HTTP REST and Web engine (default)")
	fmt.Println("  shell               Launch interactive REPL terminal for queries & topological inspection")
	fmt.Println("  status              Query running TopoSpatiaDB instance status and metrics")
	fmt.Println("  diffuse             Trigger topological consensus diffusion across sheaves")
	fmt.Println("  betti               Compute Betti numbers (harmonic topology & cycle holes)")
	fmt.Println("  version             Display engine version and build information")
	fmt.Println("\nSERVER FLAGS:")
	fmt.Println("  -port <number>      HTTP service port (default: 8089 or $PORT)")
	fmt.Println("  -data-dir <path>    Storage path for WAL and data shards (default: ~/.topospatia_data)")
	fmt.Println("  -max-dim <int>      Maximum topological simplicial dimension (default: 2)")
	fmt.Println("  -stalk-dim <int>    Stalk vector space Hilbert dimension (default: 2)")
	fmt.Println("  -fsync              Enforce synchronous fsync for write-ahead log (default: false)")
	fmt.Println("\nSHELL FLAGS:")
	fmt.Println("  -url <address>      Target TopoSpatiaDB endpoint (default: http://localhost:8089)")
	fmt.Println("\nEXAMPLES:")
	fmt.Println("  topospatiad server -port 9000 -data-dir ./db_data")
	fmt.Println("  topospatiad shell -url http://127.0.0.1:8089")
	fmt.Println("  topospatiad status")
	fmt.Println("  topospatiad diffuse")
	fmt.Println()
}

func main() {
	if len(os.Args) < 2 {
		runServerCmd(os.Args[1:])
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "help", "-help", "--help", "-h":
		printUsage()
		return
	case "version", "-version", "--version", "-v":
		fmt.Printf("TopoSpatiaDB version %s (Go 1.26 sheaf-consensus-core)\n", Version)
		return
	case "server":
		runServerCmd(os.Args[2:])
	case "shell", "repl":
		runShellCmd(os.Args[2:])
	case "status":
		runStatusCmd(os.Args[2:])
	case "diffuse":
		runDiffuseCmd(os.Args[2:])
	case "betti":
		runBettiCmd(os.Args[2:])
	default:
		if strings.HasPrefix(cmd, "-") {
			runServerCmd(os.Args[1:])
		} else {
			fmt.Printf("Error: unknown command %q\n\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}
}

func runServerCmd(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	fs.Usage = printUsage

	defaultPort := os.Getenv("PORT")
	if defaultPort == "" {
		defaultPort = "8089"
	}

	port := fs.String("port", defaultPort, "HTTP service port")
	dataDir := fs.String("data-dir", defaultDataDir(), "Data storage directory")
	maxDim := fs.Int("max-dim", 2, "Maximum simplicial dimension")
	stalkDim := fs.Int("stalk-dim", 2, "Stalk vector Hilbert dimension")
	fsync := fs.Bool("fsync", false, "Enable synchronous WAL flush")

	_ = fs.Parse(args)

	fmt.Printf("%s\nStarting TopoSpatiaDB Server v%s...\n", BannerStr, Version)
	log.Printf("Storage directory: %s", *dataDir)
	log.Printf("Simplicial parameters: max_dim=%d, stalk_dim=%d, fsync=%v", *maxDim, *stalkDim, *fsync)

	opts := topospatia.Options{
		DataDir:  *dataDir,
		MaxDim:   *maxDim,
		StalkDim: *stalkDim,
		Fsync:    *fsync,
	}

	db, err := topospatia.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open TopoSpatiaDB: %v", err)
	}
	defer db.Close()

	// Seed default simplicial complex if empty
	if len(db.Sheaf().GetCellsByDim(0)) == 0 {
		v0, _ := db.AddCell([]int{0})
		v1, _ := db.AddCell([]int{1})
		v2, _ := db.AddCell([]int{2})
		db.AddCell([]int{0, 1})
		db.AddCell([]int{1, 2})
		db.AddCell([]int{0, 2})
		db.AddCell([]int{0, 1, 2})
		db.CompileOperators()

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
	}

	srv := topospatia.NewServer(db.Sheaf(), db.Storage(), db.Consensus())
	addr := fmt.Sprintf("0.0.0.0:%s", *port)
	log.Printf("TopoSpatiaDB active and listening on http://%s", addr)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func runStatusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	targetURL := fs.String("url", "http://localhost:8089", "Server URL")
	_ = fs.Parse(args)

	resp, err := http.Get(strings.TrimRight(*targetURL, "/") + "/api/v1/status")
	if err != nil {
		fmt.Printf("Error: could not connect to TopoSpatiaDB at %s: %v\n", *targetURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	_ = json.Indent(&pretty, body, "", "  ")
	fmt.Println(pretty.String())
}

func runDiffuseCmd(args []string) {
	fs := flag.NewFlagSet("diffuse", flag.ExitOnError)
	targetURL := fs.String("url", "http://localhost:8089", "Server URL")
	_ = fs.Parse(args)

	resp, err := http.Post(strings.TrimRight(*targetURL, "/")+"/api/v1/consensus/diffuse", "application/json", nil)
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	_ = json.Indent(&pretty, body, "", "  ")
	fmt.Printf("Topological Consensus Result:\n%s\n", pretty.String())
}

func runBettiCmd(args []string) {
	fs := flag.NewFlagSet("betti", flag.ExitOnError)
	targetURL := fs.String("url", "http://localhost:8089", "Server URL")
	_ = fs.Parse(args)

	resp, err := http.Get(strings.TrimRight(*targetURL, "/") + "/api/v1/topology/betti")
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	_ = json.Indent(&pretty, body, "", "  ")
	fmt.Printf("Betti Numbers (Topological Invariants):\n%s\n", pretty.String())
}

func runShellCmd(args []string) {
	fs := flag.NewFlagSet("shell", flag.ExitOnError)
	targetURL := fs.String("url", "http://localhost:8089", "Server URL")
	_ = fs.Parse(args)

	baseURL := strings.TrimRight(*targetURL, "/")

	fmt.Printf("%s\nTopoSpatia Interactive Shell (v%s)\nConnected to %s\n", BannerStr, Version, baseURL)
	fmt.Println("Type 'help' for available REPL commands, 'exit' or Ctrl+D to quit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("topospatia> ")
		if !scanner.Scan() {
			fmt.Println("\nBye!")
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "exit", "quit":
			fmt.Println("Bye!")
			return

		case "help":
			fmt.Println("\nInteractive Commands:")
			fmt.Println("  status                         Show server health, total records, consensus metrics")
			fmt.Println("  betti                          Inspect topological invariants and cycle holes")
			fmt.Println("  diffuse                        Execute Hodge heat diffusion consensus")
			fmt.Println("  records                        List stored documents")
			fmt.Println("  get <id>                       Retrieve a document by ID")
			fmt.Println("  cell <cell_id>                 List documents attached to a topological cell")
			fmt.Println("  add-cell <v0> [v1] [v2]...     Add a simplex cell by vertex indices (e.g. add-cell 0 1)")
			fmt.Println("  rag <seed_vertex> [steps]      Execute Simplicial RAG Heat Diffusion")
			fmt.Println("  clear                          Clear terminal screen")
			fmt.Println("  exit                           Exit shell")
			fmt.Println()

		case "clear":
			fmt.Print("\033[H\033[2J")

		case "status":
			reqGet(baseURL + "/api/v1/status")

		case "betti":
			reqGet(baseURL + "/api/v1/topology/betti")

		case "diffuse":
			reqPost(baseURL+"/api/v1/consensus/diffuse", nil)

		case "records":
			reqGet(baseURL + "/api/v1/records")

		case "get":
			if len(parts) < 2 {
				fmt.Println("Usage: get <record_id>")
				continue
			}
			reqGet(baseURL + "/api/v1/records/" + parts[1])

		case "cell":
			if len(parts) < 2 {
				fmt.Println("Usage: cell <cell_id>")
				continue
			}
			reqGet(baseURL + "/api/v1/records?cell_id=" + parts[1])

		case "add-cell":
			if len(parts) < 2 {
				fmt.Println("Usage: add-cell <v0> [v1] [v2]...")
				continue
			}
			var verts []int
			for _, p := range parts[1:] {
				if v, err := strconv.Atoi(p); err == nil {
					verts = append(verts, v)
				}
			}
			payload, _ := json.Marshal(map[string]interface{}{"vertices": verts})
			reqPost(baseURL+"/api/v1/simplex", bytes.NewReader(payload))

		case "rag":
			if len(parts) < 2 {
				fmt.Println("Usage: rag <seed_vertex> [steps=3] [alpha=0.1]")
				continue
			}
			seed, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("Invalid seed vertex:", parts[1])
				continue
			}
			steps := 3
			if len(parts) >= 3 {
				if s, err := strconv.Atoi(parts[2]); err == nil {
					steps = s
				}
			}
			alpha := 0.1
			if len(parts) >= 4 {
				if a, err := strconv.ParseFloat(parts[3], 64); err == nil {
					alpha = a
				}
			}
			payload, _ := json.Marshal(map[string]interface{}{
				"seeds": []int{seed},
				"steps": steps,
				"alpha": alpha,
			})
			reqPost(baseURL+"/api/v1/rag/diffuse", bytes.NewReader(payload))

		default:
			fmt.Printf("Unknown command: %s. Type 'help' for command list.\n", cmd)
		}
	}
}

func reqGet(url string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Request error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(body))
	}
}

func reqPost(url string, body io.Reader) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", body)
	if err != nil {
		fmt.Printf("Request error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, respBody, "", "  "); err == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(respBody))
	}
}
