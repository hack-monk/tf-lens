package diff_test

import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/diff"
	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/parser"
)

func makeGraph(resources []parser.Resource) *graph.Graph {
	return graph.Build(resources)
}

var base = []parser.Resource{
	{Address: "aws_vpc.main", Type: "aws_vpc", Name: "main", Attributes: map[string]any{"id": "vpc-1"}},
	{Address: "aws_instance.web", Type: "aws_instance", Name: "web", Attributes: map[string]any{"instance_type": "t3.micro"}},
	{Address: "aws_s3_bucket.logs", Type: "aws_s3_bucket", Name: "logs", Attributes: map[string]any{"bucket": "logs"}},
}

var updated = []parser.Resource{
	// unchanged
	{Address: "aws_vpc.main", Type: "aws_vpc", Name: "main", Attributes: map[string]any{"id": "vpc-1"}},
	// changed: instance_type upgraded
	{Address: "aws_instance.web", Type: "aws_instance", Name: "web", Attributes: map[string]any{"instance_type": "t3.small"}},
	// added: new EKS cluster
	{Address: "aws_eks_cluster.main", Type: "aws_eks_cluster", Name: "main", Attributes: map[string]any{"name": "main"}},
	// aws_s3_bucket.logs is removed (absent from updated)
}

func TestAnnotate_AddedNode(t *testing.T) {
	newG := makeGraph(updated)
	baseG := makeGraph(base)
	diff.Annotate(newG, baseG)

	byID := indexGraph(newG)
	eks := byID["aws_eks_cluster.main"]
	if eks == nil {
		t.Fatal("aws_eks_cluster.main not found after annotate")
	}
	if eks.ChangeType != graph.ChangeAdded {
		t.Errorf("EKS ChangeType = %q, want %q", eks.ChangeType, graph.ChangeAdded)
	}
}

func TestAnnotate_RemovedNode(t *testing.T) {
	newG := makeGraph(updated)
	baseG := makeGraph(base)
	diff.Annotate(newG, baseG)

	byID := indexGraph(newG)
	s3 := byID["aws_s3_bucket.logs"]
	if s3 == nil {
		t.Fatal("aws_s3_bucket.logs should be appended as removed, but not found")
	}
	if s3.ChangeType != graph.ChangeRemoved {
		t.Errorf("S3 ChangeType = %q, want %q", s3.ChangeType, graph.ChangeRemoved)
	}
}

func TestAnnotate_UpdatedNode(t *testing.T) {
	newG := makeGraph(updated)
	baseG := makeGraph(base)
	diff.Annotate(newG, baseG)

	byID := indexGraph(newG)
	web := byID["aws_instance.web"]
	if web == nil {
		t.Fatal("aws_instance.web not found")
	}
	if web.ChangeType != graph.ChangeUpdated {
		t.Errorf("web ChangeType = %q, want %q", web.ChangeType, graph.ChangeUpdated)
	}
}

func TestAnnotate_UnchangedNode(t *testing.T) {
	newG := makeGraph(updated)
	baseG := makeGraph(base)
	diff.Annotate(newG, baseG)

	byID := indexGraph(newG)
	vpc := byID["aws_vpc.main"]
	if vpc == nil {
		t.Fatal("aws_vpc.main not found")
	}
	if vpc.ChangeType != graph.ChangeNone {
		t.Errorf("vpc ChangeType = %q, want %q (unchanged)", vpc.ChangeType, graph.ChangeNone)
	}
}

func TestAnnotate_Summary(t *testing.T) {
	newG := makeGraph(updated)
	baseG := makeGraph(base)
	diff.Annotate(newG, baseG)

	summary := diff.Summary(newG)
	if summary[graph.ChangeAdded] != 1 {
		t.Errorf("added = %d, want 1", summary[graph.ChangeAdded])
	}
	if summary[graph.ChangeRemoved] != 1 {
		t.Errorf("removed = %d, want 1", summary[graph.ChangeRemoved])
	}
	if summary[graph.ChangeUpdated] != 1 {
		t.Errorf("updated = %d, want 1", summary[graph.ChangeUpdated])
	}
	if summary[graph.ChangeNone] != 1 {
		t.Errorf("unchanged = %d, want 1", summary[graph.ChangeNone])
	}
}

func indexGraph(g *graph.Graph) map[string]*graph.Node {
	m := make(map[string]*graph.Node)
	for _, n := range g.Nodes {
		m[n.ID] = n
	}
	return m
}
