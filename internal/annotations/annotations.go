// Package graph builds the node/edge model from parsed Terraform resources.
// It applies AWS-aware grouping logic (VPC → Subnet → AZ → Instance)
// and assigns categories for colour coding.
package annotations

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/hack-monk/tf-lens/internal/graph"
)

// Annotation enriches a single resource node with human-authored context.
type Annotation struct {
	Resource    string `yaml:"resource"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	Docs        string `yaml:"docs"`
	Owner       string `yaml:"owner"`
}

// TourStep defines one step in a guided tour of the infrastructure.
type TourStep struct {
	Step      int    `yaml:"step"`
	Resource  string `yaml:"resource"`
	Title     string `yaml:"title"`
	Narration string `yaml:"narration"`
}

// File is the parsed tf-lens.yaml content.
type File struct {
	Annotations []Annotation `yaml:"annotations"`
	Tour        []TourStep   `yaml:"tour"`
}

// Parse reads and validates a tf-lens.yaml annotation file.
func Parse(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading annotations file: %w", err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing annotations YAML: %w", err)
	}
	return &f, nil
}

// Apply merges annotation data into graph nodes (per-field override, not all-or-nothing)
// and attaches tour steps to the graph.
// Resources in the annotation file that don't exist in the graph are silently ignored.
func Apply(g *graph.Graph, f *File) {
	// Build node index for O(1) lookup
	idx := make(map[string]*graph.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		idx[n.ID] = n
	}

	for _, a := range f.Annotations {
		n, ok := idx[a.Resource]
		if !ok {
			continue // resource not in graph — silently skip
		}
		if a.Label != "" {
			n.HumanLabel = a.Label
		}
		if a.Description != "" {
			n.Description = a.Description
		}
		if a.Docs != "" {
			n.DocsURL = a.Docs
		}
		if a.Owner != "" {
			n.Owner = a.Owner
		}
	}

	// Attach tour steps to graph
	for _, ts := range f.Tour {
		g.TourSteps = append(g.TourSteps, graph.TourStep{
			Step:      ts.Step,
			Resource:  ts.Resource,
			Title:     ts.Title,
			Narration: ts.Narration,
		})
	}
}
