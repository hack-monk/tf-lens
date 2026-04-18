// Package server implements the tf-lens serve HTTP server.
// It exposes the following endpoints:
//
//	GET /           → serves the interactive diagram HTML
//	GET /api/graph  → returns the graph as JSON (consumed by the diagram JS)
//	GET /api/events → SSE stream that pushes "reload" when the graph changes
//	GET /health     → health check
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
)

// Server holds the running state for the diagram HTTP server.
type Server struct {
	port     int
	resolver *icons.Resolver

	mu       sync.RWMutex
	g        *graph.Graph
	elements []element // pre-built, served as JSON

	// SSE subscribers — each is a channel that receives reload notifications
	subMu sync.Mutex
	subs  map[chan struct{}]struct{}
}

// New creates a Server. Call Serve() to start listening.
func New(port int, g *graph.Graph, resolver *icons.Resolver) *Server {
	s := &Server{
		port:     port,
		g:        g,
		resolver: resolver,
		subs:     make(map[chan struct{}]struct{}),
	}
	s.elements = buildElements(g)
	return s
}

// Reload swaps the graph with a new one and notifies all SSE subscribers.
func (s *Server) Reload(g *graph.Graph) {
	s.mu.Lock()
	s.g = g
	s.elements = buildElements(g)
	s.mu.Unlock()

	s.notifySubs()
}

func (s *Server) notifySubs() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // skip slow subscribers
		}
	}
}

func (s *Server) addSub(ch chan struct{}) {
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
}

func (s *Server) removeSub(ch chan struct{}) {
	s.subMu.Lock()
	delete(s.subs, ch)
	s.subMu.Unlock()
}

// Serve starts the HTTP server and blocks until it is stopped.
// It prints the listening address to stdout before blocking.
func (s *Server) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/graph", s.handleGraph)
	mux.HandleFunc("/api/events", s.handleEvents)
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
	s.mu.RLock()
	elements := s.elements
	nodeCount := len(s.g.Nodes)
	edgeCount := len(s.g.Edges)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:*")
	w.Header().Set("Cache-Control", "no-cache")

	resp := graphResponse{
		Elements:  elements,
		NodeCount: nodeCount,
		EdgeCount: edgeCount,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("error encoding graph response: %v", err)
	}
}

// handleEvents is an SSE endpoint. Clients connect and receive "reload"
// events whenever the graph is updated via Reload().
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan struct{}, 1)
	s.addSub(ch)
	defer s.removeSub(ch)

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: ok\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			fmt.Fprintf(w, "event: reload\ndata: updated\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	nodeCount := len(s.g.Nodes)
	edgeCount := len(s.g.Edges)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","nodes":%d,"edges":%d}`,
		nodeCount, edgeCount)
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
	ThreatSeverity string        `json:"threatSeverity,omitempty"`
	ThreatCodes    []string      `json:"threatCodes,omitempty"`
	ThreatFindings []findingData     `json:"threatFindings,omitempty"`
	MonthlyCost    float64           `json:"monthlyCost,omitempty"`
	DriftStatus    string            `json:"driftStatus,omitempty"`
	DriftChanges   []driftChangeData `json:"driftChanges,omitempty"`
}

type driftChangeData struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type findingData struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation"`
}

type edgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
	Flow   bool   `json:"flow,omitempty"`
	Kind   string `json:"flowKind,omitempty"`
}