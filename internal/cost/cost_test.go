package cost

import (
	"math"
	"testing"

	"github.com/hack-monk/tf-lens/internal/graph"
)

func TestParseFile(t *testing.T) {
	costs, err := ParseFile("../../testdata/infracost.json")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// S3 bucket has $0 cost — should be excluded
	if len(costs) != 3 {
		t.Fatalf("expected 3 resources with cost, got %d", len(costs))
	}

	byAddr := map[string]ResourceCost{}
	for _, c := range costs {
		byAddr[c.Address] = c
	}

	ec2 := byAddr["aws_instance.web"]
	if ec2.Address == "" {
		t.Fatal("missing aws_instance.web")
	}
	if math.Abs(ec2.MonthlyCost-73.584) > 0.001 {
		t.Errorf("ec2 monthly cost = %f, want 73.584", ec2.MonthlyCost)
	}
	if math.Abs(ec2.HourlyCost-0.1008) > 0.0001 {
		t.Errorf("ec2 hourly cost = %f, want 0.1008", ec2.HourlyCost)
	}

	rds := byAddr["aws_db_instance.postgres"]
	if math.Abs(rds.MonthlyCost-156.816) > 0.001 {
		t.Errorf("rds monthly cost = %f, want 156.816", rds.MonthlyCost)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTotalMonthlyCost(t *testing.T) {
	costs := []ResourceCost{
		{Address: "a", MonthlyCost: 100.50},
		{Address: "b", MonthlyCost: 200.25},
	}
	total := TotalMonthlyCost(costs)
	if math.Abs(total-300.75) > 0.001 {
		t.Errorf("total = %f, want 300.75", total)
	}
}

func TestAnnotateGraph(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_instance.web"},
			{ID: "aws_s3_bucket.logs"},
		},
	}
	costs := []ResourceCost{
		{Address: "aws_instance.web", MonthlyCost: 73.58, HourlyCost: 0.10},
	}

	AnnotateGraph(g, costs)

	if g.Nodes[0].MonthlyCost != 73.58 {
		t.Errorf("node[0] monthly cost = %f, want 73.58", g.Nodes[0].MonthlyCost)
	}
	if g.Nodes[0].HourlyCost != 0.10 {
		t.Errorf("node[0] hourly cost = %f, want 0.10", g.Nodes[0].HourlyCost)
	}
	if g.Nodes[1].MonthlyCost != 0 {
		t.Errorf("node[1] monthly cost = %f, want 0", g.Nodes[1].MonthlyCost)
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "$0"},
		{0.005, "<$0.01"},
		{1.50, "$1.50"},
		{73.584, "$73.58"},
		{100, "$100"},
		{1234.56, "$1,234.56"},
		{10000, "$10,000"},
		{1234567.89, "$1,234,567.89"},
	}
	for _, tt := range tests {
		got := FormatCost(tt.in)
		if got != tt.want {
			t.Errorf("FormatCost(%f) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
