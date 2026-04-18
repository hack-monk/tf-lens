package flow_test

import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/flow"
	"github.com/hack-monk/tf-lens/internal/graph"
)

func TestInfer_ALBToInstance(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_alb.web", Type: "aws_alb", Attributes: map[string]any{"subnets": []any{"subnet-001"}}},
			{ID: "aws_instance.app", Type: "aws_instance", Attributes: map[string]any{"subnet_id": "subnet-001"}},
		},
	}

	edges := flow.Infer(g)
	if len(edges) == 0 {
		t.Fatal("expected flow edge from ALB to instance")
	}
	found := false
	for _, e := range edges {
		if e.Source == "aws_alb.web" && e.Target == "aws_instance.app" {
			found = true
			if e.Kind != "ingress" {
				t.Errorf("expected kind=ingress, got %q", e.Kind)
			}
		}
	}
	if !found {
		t.Error("ALB→instance flow edge not found")
	}
}

func TestInfer_APIGWToLambda(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_api_gateway_rest_api.api", Type: "aws_api_gateway_rest_api", Attributes: map[string]any{}},
			{ID: "aws_lambda_function.handler", Type: "aws_lambda_function", Attributes: map[string]any{}},
		},
	}

	edges := flow.Infer(g)
	found := false
	for _, e := range edges {
		if e.Source == "aws_api_gateway_rest_api.api" && e.Target == "aws_lambda_function.handler" {
			found = true
			if e.Label != "invoke" {
				t.Errorf("expected label=invoke, got %q", e.Label)
			}
		}
	}
	if !found {
		t.Error("APIGW→Lambda flow edge not found")
	}
}

func TestInfer_SQSToLambda(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_sqs_queue.events", Type: "aws_sqs_queue", Attributes: map[string]any{}},
			{ID: "aws_lambda_function.processor", Type: "aws_lambda_function", Attributes: map[string]any{}},
		},
		Edges: []*graph.Edge{
			{ID: "dep", Source: "aws_lambda_function.processor", Target: "aws_sqs_queue.events"},
		},
	}

	edges := flow.Infer(g)
	found := false
	for _, e := range edges {
		if e.Source == "aws_sqs_queue.events" && e.Target == "aws_lambda_function.processor" {
			found = true
			if e.Label != "triggers" {
				t.Errorf("expected label=triggers, got %q", e.Label)
			}
		}
	}
	if !found {
		t.Error("SQS→Lambda flow edge not found")
	}
}

func TestInfer_InstanceToRDS(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_instance.web", Type: "aws_instance", Attributes: map[string]any{"vpc_id": "vpc-001"}},
			{ID: "aws_db_instance.db", Type: "aws_db_instance", Attributes: map[string]any{"vpc_id": "vpc-001"}},
		},
	}

	edges := flow.Infer(g)
	found := false
	for _, e := range edges {
		if e.Source == "aws_instance.web" && e.Target == "aws_db_instance.db" {
			found = true
			if e.Kind != "data" {
				t.Errorf("expected kind=data, got %q", e.Kind)
			}
		}
	}
	if !found {
		t.Error("Instance→RDS flow edge not found")
	}
}

func TestInfer_NoFlowEdges_EmptyGraph(t *testing.T) {
	g := &graph.Graph{}
	edges := flow.Infer(g)
	if len(edges) != 0 {
		t.Errorf("expected 0 flow edges for empty graph, got %d", len(edges))
	}
}

func TestAnnotateGraph(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_alb.web", Type: "aws_alb", Attributes: map[string]any{}},
			{ID: "aws_instance.app", Type: "aws_instance", Attributes: map[string]any{}},
		},
	}

	flows := []flow.Edge{{
		ID: "flow:test", Source: "aws_alb.web", Target: "aws_instance.app",
		Label: "HTTP", Kind: "ingress",
	}}

	flow.AnnotateGraph(g, flows)

	if len(g.FlowEdges) != 1 {
		t.Fatalf("expected 1 flow edge on graph, got %d", len(g.FlowEdges))
	}
	if g.FlowEdges[0].Kind != "ingress" {
		t.Errorf("flow edge kind = %q, want ingress", g.FlowEdges[0].Kind)
	}
}
