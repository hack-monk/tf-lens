package server

import "github.com/hack-monk/tf-lens/internal/graph"

func buildElements(g *graph.Graph) []graph.Element {
	return graph.BuildElements(g)
}
