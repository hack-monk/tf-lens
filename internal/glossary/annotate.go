package glossary

import "github.com/hack-monk/tf-lens/internal/graph"

// AnnotateGraph sets GlossaryName and GlossaryOneLiner on every node
// whose resource type has an entry in the catalog.
func AnnotateGraph(g *graph.Graph) {
	for _, n := range g.Nodes {
		if info, ok := Lookup(n.Type); ok {
			n.GlossaryName = info.Name
			n.GlossaryOneLiner = info.OneLiner
		}
	}
}
