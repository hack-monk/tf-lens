// Package diff compares two Graph instances and annotates nodes with
// ChangeType (added, removed, updated, unchanged).
//
// Usage:
//
//	newGraph := graph.Build(newResources)
//	baseGraph := graph.Build(baseResources)
//	diff.Annotate(newGraph, baseGraph)
//
// After Annotate, newGraph contains:
//   - Nodes present only in newGraph   → ChangeAdded
//   - Nodes present only in baseGraph  → ChangeRemoved  (appended to newGraph)
//   - Nodes present in both with same attributes → ChangeNone
//   - Nodes present in both with different attributes → ChangeUpdated
package diff

import (
	"encoding/json"

	"github.com/hack-monk/tf-lens/internal/graph"
)

// Annotate mutates newGraph by comparing it against baseGraph.
// Nodes that existed in baseGraph but not in newGraph are appended
// with ChangeType = ChangeRemoved so they appear in the diff diagram.
func Annotate(newGraph, baseGraph *graph.Graph) {
	// Index both graphs by node ID
	newIndex := indexNodes(newGraph)
	baseIndex := indexNodes(baseGraph)

	// Mark each node in the new graph
	for _, n := range newGraph.Nodes {
		if base, exists := baseIndex[n.ID]; !exists {
			n.ChangeType = graph.ChangeAdded
		} else if attrsChanged(n, base) {
			n.ChangeType = graph.ChangeUpdated
		} else {
			n.ChangeType = graph.ChangeNone
		}
	}

	// Append removed nodes (in base but not in new)
	for _, base := range baseGraph.Nodes {
		if _, exists := newIndex[base.ID]; !exists {
			removed := *base // copy
			removed.ChangeType = graph.ChangeRemoved
			newGraph.Nodes = append(newGraph.Nodes, &removed)
		}
	}
}

// Summary returns counts of each change type for logging / CI output.
func Summary(g *graph.Graph) map[graph.ChangeType]int {
	counts := make(map[graph.ChangeType]int)
	for _, n := range g.Nodes {
		counts[n.ChangeType]++
	}
	return counts
}

// ---- helpers ----------------------------------------------------------------

func indexNodes(g *graph.Graph) map[string]*graph.Node {
	idx := make(map[string]*graph.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		idx[n.ID] = n
	}
	return idx
}

// attrsChanged does a deep equality check on the Attributes maps.
// We serialise both to JSON for a simple but correct comparison.
func attrsChanged(a, b *graph.Node) bool {
	aJSON, _ := json.Marshal(a.Attributes)
	bJSON, _ := json.Marshal(b.Attributes)
	return string(aJSON) != string(bJSON)
}
