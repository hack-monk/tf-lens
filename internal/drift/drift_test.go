package drift

import (
	"testing"

	"github.com/hack-monk/tf-lens/internal/graph"
)

func TestParseFile(t *testing.T) {
	drifted, err := ParseFile("../../testdata/drift.json")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// S3 bucket has no-op action — should be excluded
	if len(drifted) != 2 {
		t.Fatalf("expected 2 drifted resources, got %d", len(drifted))
	}

	byAddr := map[string]DriftedResource{}
	for _, d := range drifted {
		byAddr[d.Address] = d
	}

	ec2 := byAddr["aws_instance.web"]
	if ec2.Address == "" {
		t.Fatal("missing aws_instance.web")
	}
	if ec2.Action != "update" {
		t.Errorf("ec2 action = %q, want 'update'", ec2.Action)
	}
	if len(ec2.Changes) == 0 {
		t.Fatal("expected attribute changes for aws_instance.web")
	}

	// Check specific drift: instance_type changed from t3.medium to t3.large
	found := false
	for _, c := range ec2.Changes {
		if c.Path == "instance_type" {
			found = true
			if c.Expected != "t3.medium" {
				t.Errorf("instance_type expected = %q, want 't3.medium'", c.Expected)
			}
			if c.Actual != "t3.large" {
				t.Errorf("instance_type actual = %q, want 't3.large'", c.Actual)
			}
		}
	}
	if !found {
		t.Error("instance_type change not detected")
	}

	sg := byAddr["aws_security_group.web_sg"]
	if sg.Action != "update" {
		t.Errorf("sg action = %q, want 'update'", sg.Action)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseNoDrift(t *testing.T) {
	data := []byte(`{"format_version":"1.2","resource_drift":[],"resource_changes":[],"planned_values":{"root_module":{"resources":[]}}}`)
	drifted, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(drifted) != 0 {
		t.Errorf("expected 0 drifted, got %d", len(drifted))
	}
}

func TestAnnotateGraph(t *testing.T) {
	g := &graph.Graph{
		Nodes: []*graph.Node{
			{ID: "aws_instance.web"},
			{ID: "aws_s3_bucket.assets"},
		},
	}
	drifted := []DriftedResource{
		{
			Address: "aws_instance.web",
			Type:    "aws_instance",
			Action:  "update",
			Changes: []AttributeChange{
				{Path: "instance_type", Expected: "t3.medium", Actual: "t3.large"},
			},
		},
	}

	AnnotateGraph(g, drifted)

	if g.Nodes[0].DriftStatus != "update" {
		t.Errorf("node[0] drift status = %q, want 'update'", g.Nodes[0].DriftStatus)
	}
	if len(g.Nodes[0].DriftChanges) != 1 {
		t.Fatalf("node[0] drift changes = %d, want 1", len(g.Nodes[0].DriftChanges))
	}
	if g.Nodes[0].DriftChanges[0].Path != "instance_type" {
		t.Errorf("change path = %q, want 'instance_type'", g.Nodes[0].DriftChanges[0].Path)
	}
	if g.Nodes[1].DriftStatus != "" {
		t.Errorf("node[1] should have no drift, got %q", g.Nodes[1].DriftStatus)
	}
}

func TestSummary(t *testing.T) {
	drifted := []DriftedResource{
		{Address: "a", Action: "update"},
		{Address: "b", Action: "update"},
		{Address: "c", Action: "delete"},
	}
	s := Summary(drifted)
	if s["update"] != 2 {
		t.Errorf("update count = %d, want 2", s["update"])
	}
	if s["delete"] != 1 {
		t.Errorf("delete count = %d, want 1", s["delete"])
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		in   interface{}
		want string
	}{
		{nil, "null"},
		{"hello", "hello"},
		{true, "true"},
		{false, "false"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
	}
	for _, tt := range tests {
		got := formatValue(tt.in)
		if got != tt.want {
			t.Errorf("formatValue(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
