package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/anurag/tracescope/internal/analyzer"
	diffpkg "github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/ownership"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
)

// Server holds the state for the TraceScope API.
type Server struct {
	graphData *graph.GraphData
	graphFile string
	repoRoot  string
	store     *graph.Store
	scorer    *analyzer.RiskScorer
}

// Config holds server configuration.
type Config struct {
	Port      int
	RepoRoot  string
	GraphFile string
	Scorer    *analyzer.RiskScorer
}

// New creates a new server instance.
func New(cfg Config) (*Server, error) {
	store := graph.NewStore()
	gd, err := store.Load(cfg.GraphFile)
	if err != nil {
		return nil, fmt.Errorf("loading graph: %w", err)
	}

	scorer := cfg.Scorer
	if scorer == nil {
		scorer = &analyzer.RiskScorer{}
	}

	return &Server{
		graphData: gd,
		graphFile: cfg.GraphFile,
		repoRoot:  cfg.RepoRoot,
		store:     store,
		scorer:    scorer,
	}, nil
}

// Handler returns the HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	r := mux.NewRouter()

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/graph", s.handleGraph).Methods("GET")
	api.HandleFunc("/hotspots", s.handleHotspots).Methods("GET")
	api.HandleFunc("/analyze", s.handleAnalyze).Methods("POST")
	api.HandleFunc("/why", s.handleWhy).Methods("GET")
	api.HandleFunc("/stats", s.handleStats).Methods("GET")
	api.HandleFunc("/reload", s.handleReload).Methods("POST")
	api.HandleFunc("/ws", s.handleWebSocket)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	})

	return c.Handler(r)
}

// GET /api/graph — returns full graph (nodes + edges)
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Nodes []graph.Node `json:"nodes"`
		Edges []graph.Edge `json:"edges"`
	}
	writeJSON(w, response{Nodes: s.graphData.Nodes, Edges: s.graphData.Edges})
}

// GET /api/stats — returns graph statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	nodeTypes := map[string]int{}
	edgeTypes := map[string]int{}
	languages := map[string]int{}

	for _, n := range s.graphData.Nodes {
		nodeTypes[string(n.Type)]++
		if n.Language != "" {
			languages[n.Language]++
		}
	}
	for _, e := range s.graphData.Edges {
		edgeTypes[string(e.Type)]++
	}

	writeJSON(w, map[string]interface{}{
		"total_nodes": len(s.graphData.Nodes),
		"total_edges": len(s.graphData.Edges),
		"node_types":  nodeTypes,
		"edge_types":  edgeTypes,
		"languages":   languages,
	})
}

// GET /api/hotspots?top=20&lang=go
func (s *Server) handleHotspots(w http.ResponseWriter, r *http.Request) {
	topN := 20
	if t := r.URL.Query().Get("top"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 {
			topN = n
		}
	}

	var langFilter map[string]bool
	if lang := r.URL.Query().Get("lang"); lang != "" {
		langFilter = map[string]bool{lang: true}
	}

	result := analyzer.ComputeHotspots(s.graphData, analyzer.HotspotsOptions{
		TopN:      topN,
		Languages: langFilter,
	})

	writeJSON(w, result)
}

// POST /api/analyze — accepts diff body, returns blast radius
func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		httpError(w, "reading body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		httpError(w, "empty diff body", http.StatusBadRequest)
		return
	}

	changedFiles, err := diffpkg.ParseUnifiedDiff(body)
	if err != nil {
		httpError(w, "parsing diff: "+err.Error(), http.StatusBadRequest)
		return
	}

	depthStr := r.URL.Query().Get("depth")
	depth := 5
	if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
		depth = d
	}

	ba := analyzer.NewBlastRadiusAnalyzer(s.graphData, depth, s.scorer)
	result := ba.Analyze(changedFiles)

	// Enrich with ownership if repo root is available
	if s.repoRoot != "" {
		ownerInfo, err := ownership.ResolveOwnership(s.repoRoot, result.AffectedFunctions, result.ChangedFiles)
		if err == nil {
			result.Ownership = ownerInfo
			for i := range result.AffectedFunctions {
				af := &result.AffectedFunctions[i]
				if af.Node != nil {
					if info, ok := ownerInfo.FileAuthors[af.Node.FilePath]; ok {
						af.LastAuthor = info.LastAuthor
						af.LastEmail = info.LastEmail
						af.LastModified = info.LastModified.Format("2006-01-02T15:04:05Z07:00")
					}
				}
			}
		}
	}

	writeJSON(w, result)
}

// GET /api/why?from=Build&to=Score&reverse=false
func (s *Server) handleWhy(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httpError(w, "missing 'from' and 'to' query parameters", http.StatusBadRequest)
		return
	}

	reverse := r.URL.Query().Get("reverse") == "true"

	// Resolve function names
	fromMatches := graph.FindNodesByName(s.graphData, from)
	if len(fromMatches) == 0 {
		httpError(w, fmt.Sprintf("no function matching %q", from), http.StatusNotFound)
		return
	}

	toMatches := graph.FindNodesByName(s.graphData, to)
	if len(toMatches) == 0 {
		httpError(w, fmt.Sprintf("no function matching %q", to), http.StatusNotFound)
		return
	}

	result := graph.FindShortestPath(s.graphData, fromMatches[0].Node.ID, toMatches[0].Node.ID, reverse)
	writeJSON(w, result)
}

// POST /api/reload — reload graph from disk
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	gd, err := s.store.Load(s.graphFile)
	if err != nil {
		httpError(w, "reloading graph: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.graphData = gd
	log.Info().Int("nodes", len(gd.Nodes)).Int("edges", len(gd.Edges)).Msg("graph reloaded")
	writeJSON(w, map[string]interface{}{
		"status": "reloaded",
		"nodes":  len(gd.Nodes),
		"edges":  len(gd.Edges),
	})
}

// WS /api/ws — WebSocket for streaming updates
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}
	defer conn.Close()

	// Send initial stats
	msg := map[string]interface{}{
		"type":  "connected",
		"nodes": len(s.graphData.Nodes),
		"edges": len(s.graphData.Edges),
	}
	conn.WriteJSON(msg)

	// Keep connection alive, read messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("encoding JSON response")
	}
}

func httpError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ListenAndServe starts the HTTP server.
func ListenAndServe(cfg Config) error {
	graphFile := cfg.GraphFile
	if graphFile == "" {
		cwd, _ := os.Getwd()
		graphFile = filepath.Join(cwd, ".tracescope", "graph.json")
		cfg.GraphFile = graphFile
	}

	if cfg.RepoRoot == "" {
		cwd, _ := os.Getwd()
		cfg.RepoRoot = cwd
	}

	srv, err := New(cfg)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Info().Str("addr", addr).Int("nodes", len(srv.graphData.Nodes)).Int("edges", len(srv.graphData.Edges)).Msg("starting TraceScope API server")

	fmt.Fprintf(os.Stderr, "\n  TraceScope API server running at http://localhost%s\n", addr)
	fmt.Fprintf(os.Stderr, "  Graph: %d nodes, %d edges\n\n", len(srv.graphData.Nodes), len(srv.graphData.Edges))
	fmt.Fprintf(os.Stderr, "  Endpoints:\n")
	fmt.Fprintf(os.Stderr, "    GET  /api/graph      Full dependency graph\n")
	fmt.Fprintf(os.Stderr, "    GET  /api/stats      Graph statistics\n")
	fmt.Fprintf(os.Stderr, "    GET  /api/hotspots   Top coupled functions\n")
	fmt.Fprintf(os.Stderr, "    POST /api/analyze    Blast radius from diff\n")
	fmt.Fprintf(os.Stderr, "    GET  /api/why        Call path between functions\n")
	fmt.Fprintf(os.Stderr, "    POST /api/reload     Reload graph from disk\n")
	fmt.Fprintf(os.Stderr, "    WS   /api/ws         WebSocket connection\n\n")

	return http.ListenAndServe(addr, srv.Handler())
}
