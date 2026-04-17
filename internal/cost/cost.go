// Package cost parses Infracost JSON output and maps per-resource
// monthly costs to Terraform resource addresses.
//
// Usage:
//
//	infracost breakdown --path . --format json > cost.json
//	tf-lens export --plan plan.json --cost cost.json
package cost

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/hack-monk/tf-lens/internal/graph"
)

// ResourceCost holds the parsed cost data for a single Terraform resource.
type ResourceCost struct {
	Address     string  // Terraform address, e.g. "aws_instance.web"
	MonthlyCost float64 // Estimated monthly cost in USD
	HourlyCost  float64 // Estimated hourly cost in USD
}

// ParseFile reads an Infracost JSON file and returns per-resource costs.
func ParseFile(path string) ([]ResourceCost, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cost file: %w", err)
	}
	return Parse(data)
}

// Parse decodes Infracost JSON bytes and extracts per-resource costs.
func Parse(data []byte) ([]ResourceCost, error) {
	var root infracostRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing infracost JSON: %w", err)
	}

	var costs []ResourceCost
	for _, proj := range root.Projects {
		for _, r := range proj.Breakdown.Resources {
			mc := parseFloat(r.MonthlyCost)
			hc := parseFloat(r.HourlyCost)
			if mc == 0 && hc == 0 {
				continue
			}
			costs = append(costs, ResourceCost{
				Address:     r.Name,
				MonthlyCost: mc,
				HourlyCost:  hc,
			})
		}
	}
	return costs, nil
}

// TotalMonthlyCost sums all resource monthly costs.
func TotalMonthlyCost(costs []ResourceCost) float64 {
	var total float64
	for _, c := range costs {
		total += c.MonthlyCost
	}
	return total
}

// AnnotateGraph attaches cost data to the corresponding graph nodes.
// Call after graph.Build().
func AnnotateGraph(g *graph.Graph, costs []ResourceCost) {
	idx := make(map[string]ResourceCost, len(costs))
	for _, c := range costs {
		idx[c.Address] = c
	}

	for _, node := range g.Nodes {
		if c, ok := idx[node.ID]; ok {
			node.MonthlyCost = c.MonthlyCost
			node.HourlyCost = c.HourlyCost
		}
	}
}

// FormatCost returns a human-readable cost string like "$12.34" or "$1,234.56".
func FormatCost(amount float64) string {
	if amount == 0 {
		return "$0"
	}
	if amount < 0.01 {
		return "<$0.01"
	}
	// Round to 2 decimal places
	rounded := math.Round(amount*100) / 100

	whole := int(rounded)
	frac := int(math.Round((rounded - float64(whole)) * 100))

	// Add thousand separators
	s := formatWithCommas(whole)
	if frac > 0 {
		return fmt.Sprintf("$%s.%02d", s, frac)
	}
	return "$" + s
}

func formatWithCommas(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// ── Infracost JSON schema (subset) ──────────────────────────────────────────

type infracostRoot struct {
	Projects []infracostProject `json:"projects"`
}

type infracostProject struct {
	Breakdown infracostBreakdown `json:"breakdown"`
}

type infracostBreakdown struct {
	Resources []infracostResource `json:"resources"`
}

type infracostResource struct {
	Name        string `json:"name"`
	MonthlyCost string `json:"monthlyCost"`
	HourlyCost  string `json:"hourlyCost"`
}
