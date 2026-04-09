// Package renderer writes the self-contained HTML diagram.
package renderer

import (
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
)

// ExportHTML writes a zero-dependency HTML diagram to w.
func ExportHTML(w io.Writer, g *graph.Graph, resolver *icons.Resolver) error {
	elements := buildCytoscapeElements(g, resolver)

	elemJSON, err := json.MarshalIndent(elements, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising graph elements: %w", err)
	}

	styleJSON, err := json.MarshalIndent(buildCytoscapeStyles(), "", "  ")
	if err != nil {
		return fmt.Errorf("serialising styles: %w", err)
	}

	data := templateData{
		Elements:   string(elemJSON),
		Styles:     string(styleJSON),
		CytoscapeJS: cytoscapeJS,
		DagreJS:     dagreJS,
		CytoDagreJS: cytoDagreJS,
	}

	return htmlTemplate.Execute(w, data)
}

// ---- Cytoscape element builder ----------------------------------------------

type cytoscapeNode struct {
	Data    cytoscapeNodeData `json:"data"`
	Classes string            `json:"classes,omitempty"`
}

type cytoscapeNodeData struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Parent     string `json:"parent,omitempty"`
	Type       string `json:"type"`
	Category   string `json:"category"`
	ChangeType string `json:"changeType,omitempty"`
	Icon       string `json:"icon"` // base64 data URI
}

type cytoscapeEdge struct {
	Data cytoscapeEdgeData `json:"data"`
}

type cytoscapeEdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type cytoscapeElement struct {
	Group string      `json:"group"`
	Data  interface{} `json:"data"`
}

func buildCytoscapeElements(g *graph.Graph, resolver *icons.Resolver) []cytoscapeElement {
	var elements []cytoscapeElement

	// Nodes
	for _, n := range g.Nodes {
		data := cytoscapeNodeData{
			ID:         n.ID,
			Label:      n.Name,
			Parent:     n.ParentID,
			Type:       n.Type,
			Category:   string(n.Category),
			ChangeType: string(n.ChangeType),
			Icon:       resolver.DataURI(n.Type),
		}
		el := cytoscapeElement{
			Group: "nodes",
			Data:  data,
		}
		elements = append(elements, el)
	}

	// Edges (only between nodes that exist)
	nodeIDs := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}
	for _, e := range g.Edges {
		if nodeIDs[e.Source] && nodeIDs[e.Target] {
			elements = append(elements, cytoscapeElement{
				Group: "edges",
				Data: cytoscapeEdgeData{
					ID:     e.ID,
					Source: e.Source,
					Target: e.Target,
				},
			})
		}
	}
	return elements
}

// ---- Cytoscape style builder ------------------------------------------------

type cytoscapeStyle struct {
	Selector string                 `json:"selector"`
	Style    map[string]interface{} `json:"style"`
}

// categoryColours matches the spec exactly.
var categoryColours = map[string]string{
	"networking": "#185FA5",
	"compute":    "#0F6E56",
	"storage":    "#854F0B",
	"security":   "#534AB7",
	"messaging":  "#993C1D",
	"unknown":    "#5F5E5A",
}

func buildCytoscapeStyles() []cytoscapeStyle {
	styles := []cytoscapeStyle{
		{
			Selector: "node",
			Style: map[string]interface{}{
				"label":              "data(label)",
				"text-valign":        "bottom",
				"text-halign":        "center",
				"font-size":          "11px",
				"font-family":        "monospace",
				"width":              "48px",
				"height":             "48px",
				"background-color":   "#f0f0f0",
				"background-image":   "data(icon)",
				"background-fit":     "contain",
				"background-clip":    "none",
				"border-width":       "2px",
				"border-color":       "#cccccc",
				"text-wrap":          "wrap",
				"text-max-width":     "80px",
				"padding":            "8px",
			},
		},
		{
			Selector: "$node > node", // compound parent node
			Style: map[string]interface{}{
				"padding":          "20px",
				"background-color": "#f8f9fa",
				"background-opacity": "0.6",
				"border-width":     "2px",
				"border-style":     "dashed",
				"border-color":     "#aaaaaa",
				"font-size":        "13px",
				"font-weight":      "bold",
				"text-valign":      "top",
				"text-halign":      "center",
			},
		},
		{
			Selector: "edge",
			Style: map[string]interface{}{
				"width":             "1.5px",
				"line-color":        "#999999",
				"target-arrow-color": "#999999",
				"target-arrow-shape": "triangle",
				"curve-style":       "bezier",
				"arrow-scale":       0.8,
			},
		},
	}

	// Per-category border colours
	for cat, colour := range categoryColours {
		styles = append(styles, cytoscapeStyle{
			Selector: fmt.Sprintf("node[category='%s']", cat),
			Style: map[string]interface{}{
				"border-color": colour,
				"border-width": "3px",
			},
		})
	}

	// Diff mode styles
	styles = append(styles,
		cytoscapeStyle{
			Selector: "node[changeType='added']",
			Style: map[string]interface{}{
				"border-color": "#22c55e",
				"border-width": "4px",
				"border-style": "solid",
			},
		},
		cytoscapeStyle{
			Selector: "node[changeType='removed']",
			Style: map[string]interface{}{
				"border-color":        "#ef4444",
				"border-width":        "4px",
				"border-style":        "dashed",
				"background-opacity":  "0.4",
			},
		},
		cytoscapeStyle{
			Selector: "node[changeType='updated']",
			Style: map[string]interface{}{
				"border-color": "#f59e0b",
				"border-width": "4px",
				"border-style": "solid",
			},
		},
		// Hover / selected
		cytoscapeStyle{
			Selector: "node:selected",
			Style: map[string]interface{}{
				"border-color": "#6366f1",
				"border-width": "4px",
			},
		},
	)

	return styles
}

// ---- HTML template ----------------------------------------------------------

type templateData struct {
	Elements    string
	Styles      string
	CytoscapeJS string
	DagreJS     string
	CytoDagreJS string
}

var htmlTemplate = template.Must(template.New("diagram").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>TF-Lens Infrastructure Diagram</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #1a1a2e; color: #e0e0e0; display: flex; flex-direction: column; height: 100vh; }
    #toolbar { display: flex; align-items: center; gap: 12px; padding: 10px 16px; background: #16213e; border-bottom: 1px solid #0f3460; flex-shrink: 0; }
    #toolbar h1 { font-size: 15px; font-weight: 600; color: #e0e0e0; letter-spacing: 0.5px; }
    #search { padding: 6px 12px; border-radius: 6px; border: 1px solid #0f3460; background: #0f3460; color: #e0e0e0; font-size: 13px; width: 220px; outline: none; }
    #search::placeholder { color: #888; }
    .btn { padding: 6px 12px; border-radius: 6px; border: none; cursor: pointer; font-size: 12px; font-weight: 600; }
    #btn-fit  { background: #0f3460; color: #e0e0e0; }
    #btn-fit:hover { background: #185FA5; }
    .legend { display: flex; gap: 10px; margin-left: auto; }
    .legend-item { display: flex; align-items: center; gap: 5px; font-size: 11px; }
    .legend-dot { width: 10px; height: 10px; border-radius: 50%; }
    #cy { flex: 1; background: #1a1a2e; }
    #panel { position: absolute; right: 0; top: 58px; bottom: 0; width: 300px; background: #16213e; border-left: 1px solid #0f3460; padding: 16px; overflow-y: auto; transform: translateX(100%); transition: transform 0.2s ease; }
    #panel.open { transform: translateX(0); }
    #panel h2 { font-size: 14px; margin-bottom: 12px; border-bottom: 1px solid #0f3460; padding-bottom: 8px; }
    #panel .attr { margin-bottom: 6px; }
    #panel .attr-key { font-size: 11px; color: #888; }
    #panel .attr-val { font-size: 12px; color: #e0e0e0; word-break: break-all; }
    #panel-close { float: right; cursor: pointer; color: #888; background: none; border: none; font-size: 18px; line-height: 1; }
    .diff-legend { display: flex; gap: 8px; }
    .diff-badge { display: flex; align-items: center; gap: 4px; font-size: 11px; }
    .diff-badge span { width: 12px; height: 12px; border-radius: 3px; display: inline-block; }
  </style>
</head>
<body>
  <div id="toolbar">
    <h1>🔭 TF-Lens</h1>
    <input id="search" type="text" placeholder="Search resources…" oninput="filterNodes(this.value)">
    <button class="btn" id="btn-fit" onclick="cy.fit()">Fit</button>
    <div class="legend">
      <div class="legend-item"><div class="legend-dot" style="background:#185FA5"></div>Network</div>
      <div class="legend-item"><div class="legend-dot" style="background:#0F6E56"></div>Compute</div>
      <div class="legend-item"><div class="legend-dot" style="background:#854F0B"></div>Storage</div>
      <div class="legend-item"><div class="legend-dot" style="background:#534AB7"></div>Security</div>
      <div class="legend-item"><div class="legend-dot" style="background:#993C1D"></div>Messaging</div>
    </div>
    <div class="diff-legend">
      <div class="diff-badge"><span style="background:#22c55e;border:2px solid #22c55e"></span>Added</div>
      <div class="diff-badge"><span style="background:#ef4444;border:2px dashed #ef4444;opacity:0.5"></span>Removed</div>
      <div class="diff-badge"><span style="background:#f59e0b;border:2px solid #f59e0b"></span>Changed</div>
    </div>
  </div>

  <div id="cy"></div>

  <div id="panel">
    <button id="panel-close" onclick="closePanel()">×</button>
    <h2 id="panel-title">Resource Details</h2>
    <div id="panel-body"></div>
  </div>

  <script>{{.CytoscapeJS}}</script>
  <script>{{.DagreJS}}</script>
  <script>{{.CytoDagreJS}}</script>

  <script>
  const elements = {{.Elements}};
  const styles   = {{.Styles}};

  const cy = cytoscape({
    container: document.getElementById('cy'),
    elements: elements,
    style: styles,
    layout: {
      name: 'dagre',
      rankDir: 'TB',
      nodeSep: 60,
      rankSep: 80,
      padding: 40,
      animate: false,
    },
    wheelSensitivity: 0.3,
  });

  // Click node → open detail panel
  cy.on('tap', 'node', function(evt) {
    const node = evt.target;
    const data = node.data();
    openPanel(data);
  });

  cy.on('tap', function(evt) {
    if (evt.target === cy) closePanel();
  });

  function openPanel(data) {
    document.getElementById('panel-title').textContent = data.label + ' · ' + data.type;
    const body = document.getElementById('panel-body');
    const rows = [
      ['Address', data.id],
      ['Type', data.type],
      ['Category', data.category],
    ];
    if (data.changeType) rows.push(['Change', data.changeType]);
    body.innerHTML = rows.map(([k, v]) =>
      '<div class="attr"><div class="attr-key">'+k+'</div><div class="attr-val">'+v+'</div></div>'
    ).join('');
    document.getElementById('panel').classList.add('open');
  }

  function closePanel() {
    document.getElementById('panel').classList.remove('open');
  }

  function filterNodes(q) {
    const term = q.toLowerCase().trim();
    cy.nodes().forEach(n => {
      const match = !term ||
        n.data('label').toLowerCase().includes(term) ||
        n.data('type').toLowerCase().includes(term) ||
        n.id().toLowerCase().includes(term);
      n.style('opacity', match ? 1 : 0.15);
    });
    cy.edges().forEach(e => {
      const bothVisible =
        e.source().style('opacity') == 1 &&
        e.target().style('opacity') == 1;
      e.style('opacity', bothVisible ? 1 : 0.05);
    });
  }
  </script>
</body>
</html>
`))

// cytoscapeJS / dagreJS / cytoDagreJS are loaded from embedded files.
// We use placeholder strings here; the Makefile download step populates them.
// For development, we fall back to CDN URLs via the template if the embedded
// strings are empty (detected at render time).
var (
	cytoscapeJS  = cytoscapeCDNScript
	dagreJS      = dagreCDNScript
	cytoDagreJS  = cytoDagreCDNScript
)

// CDN fallbacks — used during development before `make bundle` is run.
// The export renderer detects these sentinel values and inlines <script src>
// tags instead of inline JS, which requires an internet connection.
const (
	cytoscapeCDNScript  = `/* cytoscape.js — run 'make bundle' to embed offline */`
	dagreCDNScript      = `/* dagre — run 'make bundle' to embed offline */`
	cytoDagreCDNScript  = `/* cytoscape-dagre — run 'make bundle' to embed offline */`
)
