package renderer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
	"github.com/hack-monk/tf-lens/internal/renderer"
)

func TestExportHTML_TourStepsEmbedded(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking},
		},
		TourSteps: []graph.TourStep{
			{Step: 1, Resource: "aws_alb.main", Title: "Entry Point", Narration: "Traffic enters here."},
		},
	}
	var buf bytes.Buffer
	resolver := icons.NewResolver("")
	if err := renderer.ExportHTML(&buf, g, resolver); err != nil {
		t.Fatalf("ExportHTML error: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `"Entry Point"`) {
		t.Error("HTML does not contain tour step title")
	}
	if !strings.Contains(html, `"aws_alb.main"`) {
		t.Error("HTML does not contain tour step resource")
	}
}

func TestExportHTML_DarkModeElements(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"dark-toggle\"", "--bg-body", "id=\"dashboard\"", "doToggleDark"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing: %s", want)
		}
	}
}

func TestExportHTML_MinimapElements(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"minimap\"", "id=\"minimap-vp\"", "initMinimap", "M"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing minimap element: %s", want)
		}
	}
}

func TestExportHTML_SearchFilters(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"filter-chips\"", "parseFilters", "type:", "owner:"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing search filter element: %s", want)
		}
	}
}

func TestExportHTML_CollapsibleModules(t *testing.T) {
	g := &graph.Graph{Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}}}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"expandCollapse", "cytoscape-expand-collapse", "dblclick"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing collapsible modules element: %s", want)
		}
	}
}

func TestExportHTML_GuidedTour(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking}},
		TourSteps: []graph.TourStep{
			{Step: 1, Resource: "aws_alb.main", Title: "Entry Point", Narration: "Traffic enters here."},
		},
	}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"id=\"tour-overlay\"", "startTour", "nextTourStep", "Start Tour"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing tour element: %s", want)
		}
	}
}

func TestExportHTML_ContextPanel(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{
				ID: "aws_sqs_queue.orders", Type: "aws_sqs_queue", Name: "orders",
				Category:         graph.CategoryMessaging,
				HumanLabel:       "Order Processing Queue",
				Description:      "Handles order events.",
				DocsURL:          "https://wiki.example.com",
				Owner:            "payments-team",
				GlossaryName:     "Amazon SQS",
				GlossaryOneLiner: "Fully managed message queue.",
			},
		},
	}
	var buf bytes.Buffer
	renderer.ExportHTML(&buf, g, icons.NewResolver(""))
	html := buf.String()
	for _, want := range []string{"humanLabel", "glossaryName", "glossaryOneLiner", "docsURL", "owner"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing panel field: %s", want)
		}
	}
}

func TestExportHTML_EmptyTourSteps(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_alb.main", Type: "aws_alb", Name: "main", Category: graph.CategoryNetworking},
		},
	}
	var buf bytes.Buffer
	resolver := icons.NewResolver("")
	if err := renderer.ExportHTML(&buf, g, resolver); err != nil {
		t.Fatalf("ExportHTML error: %v", err)
	}
	html := buf.String()
	// Should contain empty tour steps JSON
	if !strings.Contains(html, `var TOUR_STEPS = []`) {
		t.Error("HTML should contain empty TOUR_STEPS array")
	}
}
