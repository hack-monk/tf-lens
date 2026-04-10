// Package diff compares two Graph instances and annotates nodes with
// ChangeType (added, removed, updated, unchanged).
package diff

import (
	"encoding/json"

	"github.com/hack-monk/tf-lens/internal/graph"
)

// Annotate mutates newGraph by comparing it against baseGraph.
//
// After Annotate, newGraph contains:
//   - Nodes present only in newGraph   → ChangeAdded
//   - Nodes present only in baseGraph  → ChangeRemoved  (appended)
//   - Nodes present in both, attrs changed → ChangeUpdated
//   - Nodes present in both, same attrs   → ChangeNone
func Annotate(newGraph, baseGraph *graph.Graph) {
	newIndex  := indexNodes(newGraph)
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

	// Append removed nodes (present in base, absent in new).
	// If the removed node's parent no longer exists in the new graph,
	// clear ParentID so it renders as a free-floating node rather than
	// creating an orphaned compound structure.
	for _, base := range baseGraph.Nodes {
		if _, exists := newIndex[base.ID]; !exists {
			removed := *base // copy — don't mutate baseGraph
			removed.ChangeType = graph.ChangeRemoved
			if removed.ParentID != "" {
				if _, parentExists := newIndex[removed.ParentID]; !parentExists {
					removed.ParentID = ""
				}
			}
			newGraph.Nodes = append(newGraph.Nodes, &removed)
		}
	}
}

// Summary returns counts of each ChangeType across all nodes.
// Useful for CLI output and CI assertions.
func Summary(g *graph.Graph) map[graph.ChangeType]int {
	counts := make(map[graph.ChangeType]int)
	for _, n := range g.Nodes {
		counts[n.ChangeType]++
	}
	return counts
}

// ── helpers ──────────────────────────────────────────────────────────────────

func indexNodes(g *graph.Graph) map[string]*graph.Node {
	idx := make(map[string]*graph.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		idx[n.ID] = n
	}
	return idx
}

// attrsChanged returns true when two nodes have different Attributes.
// JSON serialisation gives us deep equality with map key ordering handled.
func attrsChanged(a, b *graph.Node) bool {
	aJSON, _ := json.Marshal(a.Attributes)
	bJSON, _ := json.Marshal(b.Attributes)
	return string(aJSON) != string(bJSON)
}