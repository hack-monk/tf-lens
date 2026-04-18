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

// ── Module grouping tests ───────────────────────────────────────────────────

func TestBuild_ModuleGrouping(t *testing.T) {
	resources := []parser.Resource{
		{
			Address:    "module.network.aws_vpc.main",
			Type:       "aws_vpc",
			Name:       "main",
			Provider:   "aws",
			Module:     "module.network",
			Attributes: map[string]any{"id": "vpc-001"},
		},
		{
			Address:    "module.network.aws_subnet.public",
			Type:       "aws_subnet",
			Name:       "public",
			Provider:   "aws",
			Module:     "module.network",
			Attributes: map[string]any{"id": "subnet-001", "vpc_id": "vpc-001"},
		},
		{
			Address:    "aws_instance.standalone",
			Type:       "aws_instance",
			Name:       "standalone",
			Provider:   "aws",
			Attributes: map[string]any{"id": "i-001"},
		},
	}

	g := graph.Build(resources)
	byID := make(map[string]*graph.Node)
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}

	// Synthetic module node should exist
	modNode, ok := byID["module.network"]
	if !ok {
		t.Fatal("expected synthetic node for module.network")
	}
	if modNode.Type != "module" {
		t.Errorf("module node type = %q, want module", modNode.Type)
	}

	// VPC should be nested under module (infrastructure nesting may override for subnet)
	vpc := byID["module.network.aws_vpc.main"]
	if vpc == nil {
		t.Fatal("vpc node not found")
	}
	if vpc.ParentID != "module.network" {
		t.Errorf("vpc.ParentID = %q, want module.network", vpc.ParentID)
	}

	// Subnet should be nested under VPC (infra nesting overrides module grouping)
	subnet := byID["module.network.aws_subnet.public"]
	if subnet == nil {
		t.Fatal("subnet node not found")
	}
	if subnet.ParentID != "module.network.aws_vpc.main" {
		t.Errorf("subnet.ParentID = %q, want module.network.aws_vpc.main", subnet.ParentID)
	}

	// Standalone resource should have no parent
	standalone := byID["aws_instance.standalone"]
	if standalone == nil {
		t.Fatal("standalone node not found")
	}
	if standalone.ParentID != "" {
		t.Errorf("standalone.ParentID = %q, want empty", standalone.ParentID)
	}
}

func TestBuild_NestedModules(t *testing.T) {
	resources := []parser.Resource{
		{
			Address:    "module.infra.module.vpc.aws_vpc.main",
			Type:       "aws_vpc",
			Name:       "main",
			Provider:   "aws",
			Module:     "module.infra.module.vpc",
			Attributes: map[string]any{"id": "vpc-001"},
		},
	}

	g := graph.Build(resources)
	byID := make(map[string]*graph.Node)
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}

	// Both module levels should exist
	if _, ok := byID["module.infra"]; !ok {
		t.Error("expected synthetic node for module.infra")
	}
	if _, ok := byID["module.infra.module.vpc"]; !ok {
		t.Error("expected synthetic node for module.infra.module.vpc")
	}

	// Inner module should be nested under outer module
	inner := byID["module.infra.module.vpc"]
	if inner.ParentID != "module.infra" {
		t.Errorf("inner module ParentID = %q, want module.infra", inner.ParentID)
	}

	// VPC should be nested under inner module
	vpc := byID["module.infra.module.vpc.aws_vpc.main"]
	if vpc.ParentID != "module.infra.module.vpc" {
		t.Errorf("vpc.ParentID = %q, want module.infra.module.vpc", vpc.ParentID)
	}
}

func TestBuild_NoModules_NoSyntheticNodes(t *testing.T) {
	g := graph.Build(sampleResources())
	for _, n := range g.Nodes {
		if n.Type == "module" {
			t.Errorf("unexpected synthetic module node: %s", n.ID)
		}
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
