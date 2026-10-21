package topospatia

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Server exposes TopoSpatiaDB over HTTP REST & JSON.
type Server struct {
	mu           sync.RWMutex
	sheaf        *SimplicialSheaf
	storage      *StorageEngine
	consensus    *ConsensusEngine
	mux          *http.ServeMux
	authToken    string
	allowedOrigin string
}

func NewServer(sheaf *SimplicialSheaf, storage *StorageEngine, consensus *ConsensusEngine) *Server {
	authToken := os.Getenv("TOPOSPATIA_AUTH_TOKEN")
	allowedOrigin := os.Getenv("TOPOSPATIA_CORS_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}

	s := &Server{
		sheaf:         sheaf,
		storage:       storage,
		consensus:     consensus,
		mux:           http.NewServeMux(),
		authToken:     authToken,
		allowedOrigin: allowedOrigin,
	}
	s.registerRoutes()
	return s
}

// SetAuthToken updates the Bearer token dynamically.
func (s *Server) SetAuthToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authToken = token
}

// SetAllowedOrigin updates CORS policy origin.
func (s *Server) SetAllowedOrigin(origin string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedOrigin = origin
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/v1/status", s.handleStatus)
	s.mux.HandleFunc("/api/v1/simplex", s.handleSimplex)
	s.mux.HandleFunc("/api/v1/records", s.handleRecords)
	s.mux.HandleFunc("/api/v1/records/", s.handleSingleRecord)
	s.mux.HandleFunc("/api/v1/consensus/diffuse", s.handleConsensusDiffuse)
	s.mux.HandleFunc("/api/v1/topology/laplacian", s.handleTopologyLaplacian)
	s.mux.HandleFunc("/api/v1/topology/hodge-decompose", s.handleHodgeDecompose)
	s.mux.HandleFunc("/api/v1/topology/betti", s.handleBettiNumbers)
	s.mux.HandleFunc("/api/v1/rag/diffuse", s.handleRAGDiffuse)
	s.mux.HandleFunc("/api/v1/openapi.json", s.handleOpenAPISpec)
	s.mux.HandleFunc("/docs", s.handleSwaggerUI)
	s.mux.HandleFunc("/", s.handleStaticOrDashboard)
}

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(openAPISpecJSON))
}

func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(swaggerUIHTML))
}

func (s *Server) checkAuth(r *http.Request) bool {
	s.mu.RLock()
	expectedToken := s.authToken
	s.mu.RUnlock()

	if expectedToken == "" {
		// Open access if not configured
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	providedToken := strings.TrimPrefix(authHeader, "Bearer ")
	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) == 1
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	origin := s.allowedOrigin
	s.mu.RUnlock()

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Protect modifying API routes if auth token is active
	if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet {
		if !s.checkAuth(r) {
			http.Error(w, "Unauthorized: invalid or missing Bearer token", http.StatusUnauthorized)
			return
		}
	}

	// Enforce 1MB max body limit against payload-exhaustion attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.sheaf.CompileOperators()
	allRecs := s.storage.AllRecords()
	dim0 := len(s.sheaf.GetCellsByDim(0))
	dim1 := len(s.sheaf.GetCellsByDim(1))
	dim2 := len(s.sheaf.GetCellsByDim(2))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"engine":         "TopoSpatiaDB",
		"version":        "1.0.0-holographic",
		"mathematics":    "Multidimensional Sheaf Cohomology & Hodge Laplacians",
		"record_count":   len(allRecs),
		"vertices_count": dim0,
		"edges_count":    dim1,
		"faces_count":    dim2,
		"stalk_dim":      s.sheaf.StalkDim(),
		"max_dim":        s.sheaf.MaxDim(),
	})
}

func (s *Server) handleSimplex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Vertices []int `json:"vertices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Guard against Combinatorial Blowup DoS: limit max simplex dimension
	if len(req.Vertices) > 4 {
		http.Error(w, "Simplex vertex count exceeds maximum dimension (limit: 4 vertices for 3-simplex)", http.StatusBadRequest)
		return
	}

	cell, err := s.sheaf.AddCell(req.Vertices)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.sheaf.CompileOperators()

	writeJSON(w, http.StatusCreated, cell)
}

func (s *Server) handleRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cellID := r.URL.Query().Get("cell_id")
		if cellID != "" {
			recs, err := s.storage.QueryByCell(cellID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, recs)
			return
		}
		writeJSON(w, http.StatusOK, s.storage.AllRecords())

	case http.MethodPost:
		var rec Record
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		if err := s.storage.Put(&rec); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, rec)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSingleRecord(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/records/")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rec, err := s.storage.Get(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, rec)
	case http.MethodDelete:
		if err := s.storage.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "id": id})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleConsensusDiffuse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	report, err := s.consensus.DiffuseGlobal0CoChain()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleTopologyLaplacian(w http.ResponseWriter, r *http.Request) {
	dimStr := r.URL.Query().Get("dim")
	dim, err := strconv.Atoi(dimStr)
	if err != nil {
		dim = 0
	}
	s.sheaf.CompileOperators()

	s.sheaf.mu.RLock()
	lap, ok := s.sheaf.laplacians[dim]
	s.sheaf.mu.RUnlock()

	if !ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{"dimension": dim, "matrix": [][]float64{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"dimension": dim,
		"matrix":    lap,
	})
}

func (s *Server) handleHodgeDecompose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Flow     []float64 `json:"flow"`
		MaxSteps int       `json:"max_steps"`
		Dt       float64   `json:"dt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.MaxSteps <= 0 {
		req.MaxSteps = 100
	}
	if req.Dt <= 0 {
		req.Dt = 0.05
	}

	s.sheaf.CompileOperators()
	decomp := s.sheaf.DecomposeFlow1(req.Flow, req.MaxSteps, req.Dt)
	writeJSON(w, http.StatusOK, decomp)
}

func (s *Server) handleBettiNumbers(w http.ResponseWriter, r *http.Request) {
	s.sheaf.CompileOperators()
	b0, b1 := s.sheaf.ComputeBettiNumbers(1e-6)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"betti_0":     b0,
		"betti_1":     b1,
		"description": "betti_0 = connected components, betti_1 = non-bounding 1D cycles/holes",
	})
}

func (s *Server) handleRAGDiffuse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Seeds []int   `json:"seeds"`
		Steps int     `json:"steps"`
		Alpha float64 `json:"alpha"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Steps <= 0 {
		req.Steps = 3
	}
	if req.Alpha <= 0 {
		req.Alpha = 0.1
	}

	s.sheaf.CompileOperators()
	rankings := s.sheaf.SimplicialRAGDiffusion(req.Seeds, req.Steps, req.Alpha)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"seeds":    req.Seeds,
		"steps":    req.Steps,
		"rankings": rankings,
	})
}

//go:embed web/*
var embeddedWebFS embed.FS

func (s *Server) handleStaticOrDashboard(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Performance Caching Headers
	if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
	} else if strings.HasSuffix(path, ".html") || path == "index.html" {
		w.Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
	}

	// First try serving from local disk ./web folder for live hot-reloading development
	diskPath := filepath.Join("web", filepath.Clean("/"+path))
	if info, err := os.Stat(diskPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, diskPath)
		return
	}

	// Second try serving from embedded filesystem
	subFS, err := fs.Sub(embeddedWebFS, "web")
	if err == nil {
		if f, err := subFS.Open(path); err == nil {
			_ = f.Close()
			http.FileServer(http.FS(subFS)).ServeHTTP(w, r)
			return
		}
	}

	// Fallback to minimal dashboard HTML if files not found
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, DashboardHTML)
		return
	}

	http.NotFound(w, r)
}

const DashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>TopoSpatiaDB - Multidimensional Sheaf Holographic DBMS</title>
  <style>
    :root {
      --bg: #0d1117;
      --card: #161b22;
      --border: #30363d;
      --accent: #58a6ff;
      --text: #c9d1d9;
      --success: #238636;
      --gold: #d29922;
    }
    body {
      margin: 0;
      padding: 24px;
      background: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    }
    .header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      border-bottom: 1px solid var(--border);
      padding-bottom: 16px;
    }
    h1 {
      margin: 0;
      font-size: 24px;
      color: #58a6ff;
      display: flex;
      gap: 10px;
      align-items: center;
    }
    .badge {
      background: #238636;
      color: white;
      padding: 3px 8px;
      border-radius: 12px;
      font-size: 12px;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
      gap: 16px;
      margin-top: 20px;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 16px;
    }
    .card h2 {
      margin-top: 0;
      font-size: 16px;
      color: var(--accent);
      border-bottom: 1px solid var(--border);
      padding-bottom: 8px;
    }
    pre {
      background: #090d13;
      padding: 12px;
      border-radius: 6px;
      overflow-x: auto;
      font-size: 13px;
    }
    button {
      background: var(--accent);
      color: #0d1117;
      border: none;
      padding: 8px 16px;
      border-radius: 6px;
      cursor: pointer;
      font-weight: bold;
    }
    button:hover {
      opacity: 0.9;
    }
  </style>
</head>
<body>
  <div class="header">
    <h1>TopoSpatiaDB <span class="badge">Holographic Consensus</span></h1>
    <div>
      <a href="/docs" target="_blank" style="color:var(--accent);text-decoration:none;margin-right:12px;">Swagger Docs</a>
      <a href="/api/v1/status" target="_blank" style="color:var(--accent);text-decoration:none;">API Status</a>
    </div>
  </div>

  <div class="grid">
    <div class="card">
      <h2>Database Metrics</h2>
      <pre id="metrics">Loading...</pre>
    </div>

    <div class="card">
      <h2>Hodge Diffusion Consensus</h2>
      <p style="font-size:13px;color:#8b949e;">Projects replica conflicts onto harmonic subspace ker(Delta_0)</p>
      <button onclick="triggerDiffusion()">Run Topological Consensus</button>
      <pre id="diffusion-result">Ready</pre>
    </div>

    <div class="card">
      <h2>Hodge-Helmholtz Decomposition</h2>
      <p style="font-size:13px;color:#8b949e;">Decomposes simplicial edge flow: Gradient + Solenoidal + Harmonic</p>
      <button onclick="triggerHodgeDecompose()">Decompose Flow</button>
      <button onclick="queryBetti()" style="background:#8957e5;color:white;margin-left:8px;">Betti Numbers</button>
      <pre id="hodge-result">Ready</pre>
    </div>

    <div class="card">
      <h2>Simplicial RAG Heat Diffusion</h2>
      <p style="font-size:13px;color:#8b949e;">Multi-hop topological relevance propagation</p>
      <button onclick="triggerRAG()">Propagate Knowledge</button>
      <div id="rag-result" style="margin-top:10px;">Ready</div>
    </div>
  </div>

  <script>
    async function refreshMetrics() {
      try {
        const res = await fetch('/api/v1/status');
        const data = await res.json();
        document.getElementById('metrics').innerText = JSON.stringify(data, null, 2);
      } catch(e) {
        document.getElementById('metrics').innerText = 'Error loading status';
      }
    }

    async function triggerDiffusion() {
      const resEl = document.getElementById('diffusion-result');
      resEl.innerText = 'Diffusing heat kernel...';
      try {
        const res = await fetch('/api/v1/consensus/diffuse', { method: 'POST' });
        const data = await res.json();
        resEl.innerHTML = '<span style="color:#7ee787">Converged in ' + data.iterations + ' iterations. Initial Dirichlet: ' + data.initial_energy.toFixed(6) + ' -> Final: ' + data.final_energy.toFixed(6) + '</span>';
        refreshMetrics();
      } catch(e) {
        resEl.innerText = 'Diffusion error: ' + e;
      }
    }

    async function triggerHodgeDecompose() {
      const el = document.getElementById('hodge-result');
      el.innerText = 'Calculating Hodge-Helmholtz components...';
      try {
        const res = await fetch('/api/v1/topology/hodge-decompose', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ flow: [1.0, -0.5, 0.8, -0.2, 0.4, 0.1], max_steps: 50, dt: 0.05 })
        });
        const data = await res.json();
        el.innerHTML = '<div><strong>Total Energy:</strong> ' + (data.total_energy||0).toFixed(4) + '</div>' +
          '<div><strong>Cycle Energy:</strong> ' + (data.cycle_energy||0).toFixed(4) + '</div>' +
          '<div><strong>Harmonic:</strong> ' + JSON.stringify(data.harmonic) + '</div>';
      } catch(e) {
        el.innerText = 'Error: ' + e;
      }
    }

    async function queryBetti() {
      const el = document.getElementById('hodge-result');
      try {
        const res = await fetch('/api/v1/topology/betti');
        const data = await res.json();
        el.innerHTML = '<div style="color:#58a6ff;"><strong>Betti_0 (Components):</strong> ' + data.betti_0 + ' | <strong>Betti_1 (Cycles/Holes):</strong> ' + data.betti_1 + '</div>';
      } catch(e) {
        el.innerText = 'Error: ' + e;
      }
    }

    async function triggerRAG() {
      const el = document.getElementById('rag-result');
      el.innerText = 'Spreading simplicial wave...';
      try {
        const res = await fetch('/api/v1/rag/diffuse', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ seeds: [0], steps: 3, alpha: 0.1 })
        });
        const data = await res.json();
        el.innerHTML = '<pre>' + JSON.stringify(data.rankings, null, 2) + '</pre>';
      } catch(e) {
        el.innerText = 'Error: ' + e;
      }
    }

    refreshMetrics();
  </script>
</body>
</html>`

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>TopoSpatiaDB OpenAPI Documentation</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background: #0d1117; }
    .swagger-ui .topbar { display: none; }
    .swagger-ui { filter: invert(88%) hue-rotate(180deg); }
    .swagger-ui .highlight-code { filter: invert(100%) hue-rotate(180deg); }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      SwaggerUIBundle({
        url: '/api/v1/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`

const openAPISpecJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "TopoSpatiaDB API",
    "description": "High-Performance Multidimensional Sheaf Holographic Database REST API. Implements Hodge Laplacian consensus diffusion, Simplicial RAG heat waves, and simplicial complex management.",
    "version": "1.2.0"
  },
  "servers": [
    {
      "url": "/",
      "description": "Current TopoSpatiaDB instance"
    }
  ],
  "paths": {
    "/api/v1/status": {
      "get": {
        "summary": "Retrieve database status and topological dimension counts",
        "responses": {
          "200": {
            "description": "Engine health and simplex statistics"
          }
        }
      }
    },
    "/api/v1/simplex": {
      "get": {
        "summary": "List all simplices across all dimensions",
        "responses": {
          "200": { "description": "Array of Simplex cells" }
        }
      },
      "post": {
        "summary": "Add a new simplicial cell (e.g., triangle or edge)",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "vertices": {
                    "type": "array",
                    "items": { "type": "integer" },
                    "example": [0, 1, 2]
                  }
                },
                "required": ["vertices"]
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Simplicial cell added and compiled" }
        }
      }
    },
    "/api/v1/records": {
      "get": {
        "summary": "List stored documents or filter by cell_id",
        "parameters": [
          {
            "name": "cell_id",
            "in": "query",
            "schema": { "type": "string" },
            "description": "Filter records by simplicial cell ID"
          }
        ],
        "responses": {
          "200": { "description": "Array of records" }
        }
      },
      "post": {
        "summary": "Store a document attached to a simplicial cell stalk",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "id": { "type": "string", "example": "rec-alpha" },
                  "cell_id": { "type": "string", "example": "0" },
                  "dimension": { "type": "integer", "example": 0 },
                  "vector": {
                    "type": "array",
                    "items": { "type": "number" },
                    "example": [10.0, 5.0]
                  },
                  "payload": {
                    "type": "object",
                    "example": { "entity": "Alpha Node", "status": "active" }
                  }
                },
                "required": ["id", "cell_id"]
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Record stored" }
        }
      }
    },
    "/api/v1/records/{id}": {
      "get": {
        "summary": "Get document by ID",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": { "description": "Document details" },
          "404": { "description": "Record not found" }
        }
      },
      "delete": {
        "summary": "Delete document by ID",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": { "description": "Record deleted" }
        }
      }
    },
    "/api/v1/consensus/diffuse": {
      "post": {
        "summary": "Execute discrete Hodge Laplacian heat diffusion consensus",
        "description": "Projects conflict gradients into the harmonic subspace ker(Delta_0), driving Dirichlet energy to zero.",
        "responses": {
          "200": { "description": "Diffusion convergence report" }
        }
      }
    },
    "/api/v1/topology/laplacian": {
      "get": {
        "summary": "Retrieve dense k-Laplacian matrix",
        "parameters": [
          {
            "name": "dim",
            "in": "query",
            "schema": { "type": "integer", "default": 0 },
            "description": "Simplicial dimension k (0=vertex Laplacian, 1=Hodge 1-Laplacian)"
          }
        ],
        "responses": {
          "200": { "description": "Dense Laplacian matrix" }
        }
      }
    },
    "/api/v1/topology/hodge-decompose": {
      "post": {
        "summary": "Compute 3-way Hodge-Helmholtz decomposition of edge flow",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "flow": {
                    "type": "array",
                    "items": { "type": "number" }
                  },
                  "max_steps": { "type": "integer", "default": 50 },
                  "dt": { "type": "number", "default": 0.05 }
                },
                "required": ["flow"]
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Gradient, solenoidal (curl), and harmonic components" }
        }
      }
    },
    "/api/v1/topology/betti": {
      "get": {
        "summary": "Compute Betti numbers (connected components, 1D/2D holes)",
        "responses": {
          "200": { "description": "Topological Betti numbers" }
        }
      }
    },
    "/api/v1/rag/diffuse": {
      "post": {
        "summary": "Simplicial RAG topological relevance propagation",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "seeds": {
                    "type": "array",
                    "items": { "type": "integer" },
                    "example": [0]
                  },
                  "steps": { "type": "integer", "default": 3 },
                  "alpha": { "type": "number", "default": 0.1 }
                },
                "required": ["seeds"]
              }
            }
          }
        },
        "responses": {
          "200": { "description": "Ranked cell relevance scores" }
        }
      }
    }
  }
}`
