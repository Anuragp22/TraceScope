package output

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"io"
	"strings"

	"github.com/anurag/tracescope/internal/analyzer"
	"github.com/anurag/tracescope/internal/graph"
)

//go:embed d3.v7.min.js
var d3Script string

//go:embed report_template.html
var reportTemplateHTML string

// ReportData holds the data injected into the HTML template.
type ReportData struct {
	D3Script         template.JS
	GraphDataJSON    template.JS
	AnalysisDataJSON template.JS
}

// GenerateReport writes a self-contained HTML report to w.
// If analysis is nil, the report shows only the dependency graph.
func GenerateReport(w io.Writer, graphData *graph.GraphData, analysis *analyzer.AnalysisResult) error {
	// Serialize graph data (only nodes and edges, skip metadata)
	type slimGraph struct {
		Nodes []graph.Node `json:"nodes"`
		Edges []graph.Edge `json:"edges"`
	}
	graphJSON, err := json.Marshal(slimGraph{
		Nodes: graphData.Nodes,
		Edges: graphData.Edges,
	})
	if err != nil {
		return err
	}

	// Serialize analysis data (may be nil)
	var analysisJSON []byte
	if analysis != nil {
		analysisJSON, err = json.Marshal(analysis)
		if err != nil {
			return err
		}
	} else {
		analysisJSON = []byte("null")
	}

	// Sanitize JSON to prevent </script> injection
	graphStr := strings.ReplaceAll(string(graphJSON), "</", "<\\/")
	analysisStr := strings.ReplaceAll(string(analysisJSON), "</", "<\\/")

	data := ReportData{
		D3Script:         template.JS(d3Script),
		GraphDataJSON:    template.JS(graphStr),
		AnalysisDataJSON: template.JS(analysisStr),
	}

	tmpl, err := template.New("report").Parse(reportTemplateHTML)
	if err != nil {
		return err
	}

	return tmpl.Execute(w, data)
}
