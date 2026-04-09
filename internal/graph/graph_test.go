package graph_test

import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/parser"
)

func sampleResources() []parser.Resource {
	return []parser.Resource{
		{
			Address:  "aws_vpc.main",
			Type:     "aws_vpc",
			Name:     "main",
			Provider: "aws",
			Attributes: map[string]any{
				"id":         "vpc-001",
				"cidr_block": "10.0.0.0/16",
			},
		},
		{
			Address:  "aws_subnet.public",
			Type:     "aws_subnet",
			Name:     "public",
			Provider: "aws",
			Attributes: map[string]any{
				"id":     "subnet-001",
				"vpc_id": "vpc-001",
			},
			Dependencies: []string{"aws_vpc.main"},
		},
		{
			Address:  "aws_instance.web",
			Type:     "aws_instance",
			Name:     "web",
			Provider: "aws",
			Attributes: map[string]any{
				"id":        "i-001",
				"subnet_id": "subnet-001",
			},
			Dependencies: []string{"aws_subnet.public"},
		},
	}
}

func TestBuild_NodeCount(t *testing.T) {
	g := graph.Build(sampleResources())
	if len(g.Nodes) != 3 {
		t.Errorf("node count = %d, want 3", len(g.Nodes))
	}
}

func TestBuild_EdgeCount(t *testing.T) {
	g := graph.Build(sampleResources())
	if len(g.Edges) != 2 {
		t.Errorf("edge count = %d, want 2", len(g.Edges))
	}
}

func TestBuild_CategoryAssignment(t *testing.T) {
	g := graph.Build(sampleResources())
	byID := make(map[string]*graph.Node)
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}

	cases := []struct {
		id   string
		want graph.Category
	}{
		{"aws_vpc.main", graph.CategoryNetworking},
		{"aws_subnet.public", graph.CategoryNetworking},
		{"aws_instance.web", graph.CategoryCompute},
	}
	for _, tc := range cases {
		n, ok := byID[tc.id]
		if !ok {
			t.Errorf("node %q not found", tc.id)
			continue
		}
		if n.Category != tc.want {
			t.Errorf("node %q category = %q, want %q", tc.id, n.Category, tc.want)
		}
	}
}

func TestBuild_VPCNesting(t *testing.T) {
	g := graph.Build(sampleResources())
	byID := make(map[string]*graph.Node)
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	subnet := byID["aws_subnet.public"]
	if subnet == nil {
		t.Fatal("subnet node not found")
	}
	if subnet.ParentID != "aws_vpc.main" {
		t.Errorf("subnet.ParentID = %q, want aws_vpc.main", subnet.ParentID)
	}
}

func TestBuild_SubnetNesting(t *testing.T) {
	g := graph.Build(sampleResources())
	byID := make(map[string]*graph.Node)
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	instance := byID["aws_instance.web"]
	if instance == nil {
		t.Fatal("instance node not found")
	}
	if instance.ParentID != "aws_subnet.public" {
		t.Errorf("instance.ParentID = %q, want aws_subnet.public", instance.ParentID)
	}
}

func TestBuild_EmptyInput(t *testing.T) {
	g := graph.Build(nil)
	if g == nil {
		t.Fatal("Build(nil) returned nil")
	}
	if len(g.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(g.Nodes))
	}
}
