package threat

import "github.com/hack-monk/tf-lens/internal/graph"

// AnnotateGraph attaches threat findings to the corresponding graph nodes.
// It mutates each node in g by setting ThreatCodes and ThreatMaxSeverity.
// Call after graph.Build() and diff.Annotate().
func AnnotateGraph(g *graph.Graph, findings []Finding) {
	idx := IndexByAddress(findings)

	for _, node := range g.Nodes {
		nodeFindings, ok := idx[node.ID]
		if !ok || len(nodeFindings) == 0 {
			continue
		}

		var codes []string
		var detailed []graph.NodeFinding
		maxSev := SeverityInfo

		for _, f := range nodeFindings {
			codes = append(codes, f.Code)
			if f.Severity.Weight() > maxSev.Weight() {
				maxSev = f.Severity
			}
			detailed = append(detailed, graph.NodeFinding{
				Code:        f.Code,
				Severity:    string(f.Severity),
				Title:       f.Title,
				Detail:      f.Detail,
				Remediation: f.Remediation,
			})
		}

		node.ThreatCodes = codes
		node.ThreatMaxSeverity = string(maxSev)
		node.ThreatFindings = detailed
	}
}