package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/anurag/tracescope/internal/analyzer"
	diffpkg "github.com/anurag/tracescope/internal/diff"
	"github.com/anurag/tracescope/internal/graph"
	"github.com/anurag/tracescope/internal/ownership"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
)

// Server holds the state for the TraceScope API.
type Server struct {
	mu        sync.RWMutex
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

	r.HandleFunc("/api/graph", s.handleGraph).Methods("GET")
	r.HandleFunc("/api/hotspots", s.handleHotspots).Methods("GET")
	r.HandleFunc("/api/analyze/branches", s.handleBranches).Methods("GET")
	r.HandleFunc("/api/analyze/diff", s.handleGitDiff).Methods("GET")
	r.HandleFunc("/api/analyze", s.handleAnalyze).Methods("POST")
	r.HandleFunc("/api/why", s.handleWhy).Methods("GET")
	r.HandleFunc("/api/stats", s.handleStats).Methods("GET")
	r.HandleFunc("/api/reload", s.handleReload).Methods("POST")

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
	graphData := s.graphSnapshot()
	type response struct {
		Nodes []graph.Node `json:"nodes"`
		Edges []graph.Edge `json:"edges"`
	}
	writeJSON(w, response{Nodes: graphData.Nodes, Edges: graphData.Edges})
}

// GET /api/stats — returns graph statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	graphData := s.graphSnapshot()
	nodeTypes := map[string]int{}
	edgeTypes := map[string]int{}
	languages := map[string]int{}

	for _, n := range graphData.Nodes {
		nodeTypes[string(n.Type)]++
		if n.Language != "" {
			languages[n.Language]++
		}
	}
	for _, e := range graphData.Edges {
		edgeTypes[string(e.Type)]++
	}

	writeJSON(w, map[string]interface{}{
		"total_nodes": len(graphData.Nodes),
		"total_edges": len(graphData.Edges),
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

	result := analyzer.ComputeHotspots(s.graphSnapshot(), analyzer.HotspotsOptions{
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

	ba := analyzer.NewBlastRadiusAnalyzer(s.graphSnapshot(), depth, s.scorer)
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
	graphData := s.graphSnapshot()
	fromMatches := graph.FindNodesByName(graphData, from)
	if len(fromMatches) == 0 {
		httpError(w, fmt.Sprintf("no function matching %q", from), http.StatusNotFound)
		return
	}

	toMatches := graph.FindNodesByName(graphData, to)
	if len(toMatches) == 0 {
		httpError(w, fmt.Sprintf("no function matching %q", to), http.StatusNotFound)
		return
	}

	result := graph.FindShortestPath(graphData, fromMatches[0].Node.ID, toMatches[0].Node.ID, reverse)
	writeJSON(w, result)
}

// POST /api/reload — reload graph from disk
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	gd, err := s.store.Load(s.graphFile)
	if err != nil {
		httpError(w, "reloading graph: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.replaceGraph(gd)
	log.Info().Int("nodes", len(gd.Nodes)).Int("edges", len(gd.Edges)).Msg("graph reloaded")
	writeJSON(w, map[string]interface{}{
		"status": "reloaded",
		"nodes":  len(gd.Nodes),
		"edges":  len(gd.Edges),
	})
}

// GET /api/analyze/branches — list git branches
func (s *Server) handleBranches(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("git", "-C", s.repoRoot, "branch", "--format=%(refname:short)").Output()
	if err != nil {
		httpError(w, "listing branches: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}

	// Also get current branch
	currentOut, _ := exec.Command("git", "-C", s.repoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	current := strings.TrimSpace(string(currentOut))

	writeJSON(w, map[string]interface{}{
		"branches": branches,
		"current":  current,
	})
}

// GET /api/analyze/diff?base=main&head=HEAD&uncommitted=true
func (s *Server) handleGitDiff(w http.ResponseWriter, r *http.Request) {
	base := r.URL.Query().Get("base")
	head := r.URL.Query().Get("head")
	uncommitted := r.URL.Query().Get("uncommitted") == "true"

	var args []string
	if uncommitted {
		// Staged + unstaged changes
		args = []string{"-C", s.repoRoot, "diff", "HEAD"}
	} else if base != "" && head != "" {
		args = []string{"-C", s.repoRoot, "diff", base + "..." + head}
	} else if base != "" {
		args = []string{"-C", s.repoRoot, "diff", base + "...HEAD"}
	} else {
		// Default: uncommitted changes
		args = []string{"-C", s.repoRoot, "diff", "HEAD"}
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		// git diff returns exit code 1 if there are differences — check if output exists
		if len(out) == 0 {
			httpError(w, "git diff failed: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if len(out) == 0 {
		httpError(w, "no changes found", http.StatusNotFound)
		return
	}

	// Parse and analyze the diff
	changedFiles, err := diffpkg.ParseUnifiedDiff(out)
	if err != nil {
		httpError(w, "parsing diff: "+err.Error(), http.StatusInternalServerError)
		return
	}

	depthStr := r.URL.Query().Get("depth")
	depth := 5
	if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
		depth = d
	}

	ba := analyzer.NewBlastRadiusAnalyzer(s.graphSnapshot(), depth, s.scorer)
	result := ba.Analyze(changedFiles)

	// Enrich with ownership
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

func (s *Server) graphSnapshot() *graph.GraphData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graphData
}

func (s *Server) replaceGraph(graphData *graph.GraphData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.graphData = graphData
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
	fmt.Fprintf(os.Stderr, "    POST /api/reload     Reload graph from disk\n\n")

	return http.ListenAndServe(addr, srv.Handler())
}
