// Package drift detects state drift from a Terraform refresh-only plan.
//
// Drift occurs when someone modifies AWS resources manually (via console,
// CLI, or another tool) and the actual cloud state diverges from the
// Terraform state file.
//
// Usage:
//
//	terraform plan -refresh-only -out=drift.bin
//	terraform show -json drift.bin > drift.json
//	tf-lens export --plan plan.json --drift drift.json
package drift

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hack-monk/tf-lens/internal/graph"

	tfjson "github.com/hashicorp/terraform-json"
)

// DriftedResource represents a single resource whose cloud state differs
// from the Terraform state.
type DriftedResource struct {
	Address string            // Terraform address, e.g. "aws_instance.web"
	Type    string            // Resource type, e.g. "aws_instance"
	Action  string            // "update", "delete", "create" (what Terraform would do to fix)
	Changes []AttributeChange // Per-attribute diffs
}

// AttributeChange describes a single attribute that drifted.
type AttributeChange struct {
	Path     string // Attribute path, e.g. "tags.Name"
	Expected string // Value in Terraform state
	Actual   string // Value in cloud (after drift)
}

// ParseFile reads a refresh-only plan JSON file and returns drifted resources.
func ParseFile(path string) ([]DriftedResource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading drift file: %w", err)
	}
	return Parse(data)
}

// Parse decodes a Terraform plan JSON and extracts resource drift entries.
func Parse(data []byte) ([]DriftedResource, error) {
	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parsing plan JSON: %w", err)
	}

	var drifted []DriftedResource

	for _, rc := range plan.ResourceDrift {
		if rc.Change == nil {
			continue
		}

		action := summariseActions(rc.Change.Actions)
		if action == "no-op" {
			continue
		}

		changes := diffAttributes(rc.Change.Before, rc.Change.After)

		drifted = append(drifted, DriftedResource{
			Address: rc.Address,
			Type:    rc.Type,
			Action:  action,
			Changes: changes,
		})
	}

	return drifted, nil
}

// RunRefreshOnly runs `terraform plan -refresh-only` against a directory
// and returns detected drift.
func RunRefreshOnly(tfDir string) ([]DriftedResource, error) {
	// Check terraform is installed
	if _, err := exec.LookPath("terraform"); err != nil {
		return nil, fmt.Errorf("terraform CLI not found on PATH")
	}

	// Create temp file for plan output
	tmpFile, err := os.CreateTemp("", "tf-lens-drift-*.bin")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Run terraform plan -refresh-only
	planCmd := exec.Command("terraform", "plan", "-refresh-only",
		"-out="+tmpPath, "-input=false", "-no-color")
	planCmd.Dir = tfDir
	planCmd.Stderr = os.Stderr
	if err := planCmd.Run(); err != nil {
		return nil, fmt.Errorf("terraform plan -refresh-only failed: %w", err)
	}

	// Convert to JSON
	showCmd := exec.Command("terraform", "show", "-json", tmpPath)
	showCmd.Dir = tfDir
	out, err := showCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform show -json failed: %w", err)
	}

	return Parse(out)
}

// AnnotateGraph attaches drift data to the corresponding graph nodes.
func AnnotateGraph(g *graph.Graph, drifted []DriftedResource) {
	idx := make(map[string]DriftedResource, len(drifted))
	for _, d := range drifted {
		idx[d.Address] = d
	}

	for _, node := range g.Nodes {
		if d, ok := idx[node.ID]; ok {
			node.DriftStatus = d.Action
			for _, c := range d.Changes {
				node.DriftChanges = append(node.DriftChanges, graph.NodeDriftChange{
					Path:     c.Path,
					Expected: c.Expected,
					Actual:   c.Actual,
				})
			}
		}
	}
}

// Summary returns drift counts by action type.
func Summary(drifted []DriftedResource) map[string]int {
	counts := map[string]int{}
	for _, d := range drifted {
		counts[d.Action]++
	}
	return counts
}

// ── helpers ─────────────────────────────────────────────────────────────────

func summariseActions(actions tfjson.Actions) string {
	for _, a := range actions {
		switch a {
		case tfjson.ActionUpdate:
			return "update"
		case tfjson.ActionDelete:
			return "delete"
		case tfjson.ActionCreate:
			return "create"
		}
	}
	return "no-op"
}

// diffAttributes compares before/after values and returns changed attributes.
func diffAttributes(before, after interface{}) []AttributeChange {
	beforeMap, ok1 := toStringMap(before)
	afterMap, ok2 := toStringMap(after)
	if !ok1 || !ok2 {
		return nil
	}

	var changes []AttributeChange

	// Check all keys in before
	for key, bVal := range beforeMap {
		aVal, exists := afterMap[key]
		if !exists {
			changes = append(changes, AttributeChange{
				Path:     key,
				Expected: formatValue(bVal),
				Actual:   "(removed)",
			})
			continue
		}
		bStr := formatValue(bVal)
		aStr := formatValue(aVal)
		if bStr != aStr {
			changes = append(changes, AttributeChange{
				Path:     key,
				Expected: bStr,
				Actual:   aStr,
			})
		}
	}

	// Check for new keys in after
	for key, aVal := range afterMap {
		if _, exists := beforeMap[key]; !exists {
			changes = append(changes, AttributeChange{
				Path:     key,
				Expected: "(not set)",
				Actual:   formatValue(aVal),
			})
		}
	}

	return changes
}

func toStringMap(v interface{}) (map[string]interface{}, bool) {
	if v == nil {
		return map[string]interface{}{}, true
	}
	m, ok := v.(map[string]interface{})
	return m, ok
}

func formatValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	case map[string]interface{}, []interface{}:
		b, _ := json.Marshal(val)
		s := string(b)
		if len(s) > 80 {
			return s[:77] + "..."
		}
		return s
	default:
		return fmt.Sprintf("%v", val)
	}
}

// escapeHTML escapes strings for safe embedding in HTML.
func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
