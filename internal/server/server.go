// Package server implements the tf-lens serve HTTP server.
// It exposes two endpoints:
//
//	GET /         → serves the interactive diagram HTML
//	GET /api/graph → returns the graph as JSON (consumed by the diagram JS)
//
// The graph is built once at startup. A future --watch flag can trigger
// rebuilds on file changes without restarting the server.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
)

// Server holds the running state for the diagram HTTP server.
type Server struct {
	port     int
	g        *graph.Graph
	resolver *icons.Resolver
	elements []element // pre-built, served as JSON
}

// New creates a Server. Call Serve() to start listening.
func New(port int, g *graph.Graph, resolver *icons.Resolver) *Server {
	s := &Server{
		port:     port,
		g:        g,
		resolver: resolver,
	}
	s.elements = buildElements(g)
	return s
}

// Serve starts the HTTP server and blocks until it is stopped.
// It prints the listening address to stdout before blocking.
func (s *Server) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/graph", s.handleGraph)
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("\n🔭  TF-Lens is running\n")
	fmt.Printf("    Local:   http://localhost:%d\n", s.port)
	fmt.Printf("    Press Ctrl+C to stop\n\n")

	return srv.ListenAndServe()
}

// handleIndex serves the full interactive diagram HTML page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, serveHTML)
}

// handleGraph returns the graph elements as JSON.
// The diagram JS fetches this on load and on every refresh.
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:*")
	w.Header().Set("Cache-Control", "no-cache")

	resp := graphResponse{
		Elements:  s.elements,
		NodeCount: len(s.g.Nodes),
		EdgeCount: len(s.g.Edges),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("error encoding graph response: %v", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","nodes":%d,"edges":%d}`,
		len(s.g.Nodes), len(s.g.Edges))
}

// ── JSON response types ──────────────────────────────────────────────────────

type graphResponse struct {
	Elements  []element `json:"elements"`
	NodeCount int       `json:"nodeCount"`
	EdgeCount int       `json:"edgeCount"`
}

type element struct {
	Group string      `json:"group"`
	Data  interface{} `json:"data"`
}

type nodeData struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Parent         string   `json:"parent,omitempty"`
	Type           string   `json:"type"`
	Category       string   `json:"category"`
	ChangeType     string   `json:"changeType,omitempty"`
	Abbrev         string   `json:"abbrev"`
	IsParent       bool     `json:"isParent"`
	ThreatSeverity string   `json:"threatSeverity,omitempty"`
	ThreatCodes    []string `json:"threatCodes,omitempty"`
}

type edgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}