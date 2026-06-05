package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/anurag/tracescope/internal/graph"
)

func TestHandleReloadUpdatesGraph(t *testing.T) {
	graphFile := filepath.Join(t.TempDir(), "graph.json")
	store := graph.NewStore()

	if err := store.Save(&graph.GraphData{
		Nodes: []graph.Node{{ID: "file:1", Type: graph.NodeFile, Name: "old.go", FilePath: "old.go", Language: "go"}},
	}, graphFile); err != nil {
		t.Fatalf("save initial graph: %v", err)
	}

	srv, err := New(Config{GraphFile: graphFile, RepoRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	if err := store.Save(&graph.GraphData{
		Nodes: []graph.Node{
			{ID: "file:1", Type: graph.NodeFile, Name: "new.go", FilePath: "new.go", Language: "go"},
			{ID: "func:1", Type: graph.NodeFunction, Name: "Run", FilePath: "new.go", Language: "go"},
		},
	}, graphFile); err != nil {
		t.Fatalf("save replacement graph: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/reload", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("reload status = %d, body=%s", rr.Code, rr.Body.String())
	}

	reloaded := srv.graphSnapshot()
	if len(reloaded.Nodes) != 2 {
		t.Fatalf("expected 2 nodes after reload, got %d", len(reloaded.Nodes))
	}
}

func TestConcurrentGraphReadAndReload(t *testing.T) {
	graphFile := filepath.Join(t.TempDir(), "graph.json")
	store := graph.NewStore()
	if err := store.Save(&graph.GraphData{
		Nodes: []graph.Node{{ID: "file:1", Type: graph.NodeFile, Name: "main.go", FilePath: "main.go", Language: "go"}},
	}, graphFile); err != nil {
		t.Fatalf("save graph: %v", err)
	}

	srv, err := New(Config{GraphFile: graphFile, RepoRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	handler := srv.Handler()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("graph status = %d", rr.Code)
			}
		}()
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/reload", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("reload status = %d", rr.Code)
			}
		}()
	}

	wg.Wait()
}
