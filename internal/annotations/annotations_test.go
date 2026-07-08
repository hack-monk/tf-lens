package annotations_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hack-monk/tf-lens/internal/annotations"
	"github.com/hack-monk/tf-lens/internal/graph"
)

const sampleYAML = `
annotations:
  - resource: aws_sqs_queue.orders
    label: "Order Processing Queue"
    description: "Decouples checkout from fulfillment."
    docs: "https://wiki.example.com/order-queue"
    owner: "payments-team"

tour:
  - step: 1
    resource: aws_alb.main
    title: "Entry Point"
    narration: "All traffic enters here."
  - step: 2
    resource: aws_sqs_queue.orders
    title: "Order Queue"
    narration: "Async handoff to fulfillment."
`

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "tf-lens-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestParse_ValidYAML(t *testing.T) {
	path := writeTempYAML(t, sampleYAML)
	f, err := annotations.Parse(path)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(f.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(f.Annotations))
	}
	a := f.Annotations[0]
	if a.Resource != "aws_sqs_queue.orders" {
		t.Errorf("Resource = %q, want %q", a.Resource, "aws_sqs_queue.orders")
	}
	if a.Label != "Order Processing Queue" {
		t.Errorf("Label = %q", a.Label)
	}
	if a.Owner != "payments-team" {
		t.Errorf("Owner = %q", a.Owner)
	}
	if len(f.Tour) != 2 {
		t.Fatalf("expected 2 tour steps, got %d", len(f.Tour))
	}
	if f.Tour[0].Step != 1 || f.Tour[0].Resource != "aws_alb.main" {
		t.Errorf("tour step 1 mismatch: %+v", f.Tour[0])
	}
}

func TestParse_MissingFile(t *testing.T) {
	_, err := annotations.Parse(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestApply_MergesIntoNodes(t *testing.T) {
	path := writeTempYAML(t, sampleYAML)
	f, _ := annotations.Parse(path)

	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_sqs_queue.orders", Type: "aws_sqs_queue", Name: "orders"},
			{ID: "aws_alb.main", Type: "aws_alb", Name: "main"},
		},
	}
	annotations.Apply(g, f)

	// Check annotation merged
	var queueNode *graph.Node
	for _, n := range g.Nodes {
		if n.ID == "aws_sqs_queue.orders" {
			queueNode = n
		}
	}
	if queueNode == nil {
		t.Fatal("queue node not found")
	}
	if queueNode.HumanLabel != "Order Processing Queue" {
		t.Errorf("HumanLabel = %q", queueNode.HumanLabel)
	}
	if queueNode.Description != "Decouples checkout from fulfillment." {
		t.Errorf("Description = %q", queueNode.Description)
	}
	if queueNode.Owner != "payments-team" {
		t.Errorf("Owner = %q", queueNode.Owner)
	}

	// Check tour steps attached to graph
	if len(g.TourSteps) != 2 {
		t.Fatalf("expected 2 tour steps on graph, got %d", len(g.TourSteps))
	}
}
