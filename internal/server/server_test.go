package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/hack-monk/tf-lens/internal/graph"
)

// TestHandleGraph_IncludesTourSteps guards against the guided tour going dark
// in serve mode: /api/graph must carry TourSteps so the client can populate
// TOUR_STEPS after the async fetch (the page ships with none baked in).
func TestHandleGraph_IncludesTourSteps(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking},
		},
		TourSteps: []graph.TourStep{
			{Step: 1, Resource: "aws_alb.main", Title: "Entry Point", Narration: "Traffic enters here."},
		},
	}
	srv := New(0, g)

	req := httptest.NewRequest("GET", "/api/graph", nil)
	rec := httptest.NewRecorder()
	srv.handleGraph(rec, req)

	var resp struct {
		TourSteps []graph.TourStep `json:"tourSteps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding /api/graph response: %v", err)
	}
	if len(resp.TourSteps) != 1 {
		t.Fatalf("expected 1 tour step in /api/graph response, got %d", len(resp.TourSteps))
	}
	if resp.TourSteps[0].Title != "Entry Point" {
		t.Errorf("tour step title = %q, want %q", resp.TourSteps[0].Title, "Entry Point")
	}
}
