package renderer

import (
	"encoding/json"
	"fmt"
	"io"
	iofs "io/fs"
	"sort"
	"text/template"

	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
)

// ExportThreats writes a threat findings report in markdown table format.
func ExportThreats(w io.Writer, g *graph.Graph) error {
	// Collect all nodes with threats, sorted by severity
	type finding struct {
		Resource    string
		Type        string
		Code        string
		Severity    string
		Title       string
		Detail      string
		Remediation string
	}

	var findings []finding
	counts := map[string]int{"critical": 0, "high": 0, "medium": 0, "info": 0}

	for _, n := range g.Nodes {
		for _, f := range n.ThreatFindings {
			findings = append(findings, finding{
				Resource:    n.ID,
				Type:        n.Type,
				Code:        f.Code,
				Severity:    f.Severity,
				Title:       f.Title,
				Detail:      f.Detail,
				Remediation: f.Remediation,
			})
			counts[f.Severity]++
		}
	}

	// Sort by severity weight (critical first)
	sevWeight := map[string]int{"critical": 4, "high": 3, "medium": 2, "info": 1}
	sort.Slice(findings, func(i, j int) bool {
		return sevWeight[findings[i].Severity] > sevWeight[findings[j].Severity]
	})

	total := len(findings)
	sevIcon := map[string]string{"critical": "🔴", "high": "🟠", "medium": "🟡", "info": "🔵"}

	fmt.Fprintf(w, "# TF-Lens Threat Report\n\n")
	fmt.Fprintf(w, "**%d findings** across %d resources\n\n", total, len(g.Nodes))
	fmt.Fprintf(w, "| Severity | Count |\n")
	fmt.Fprintf(w, "|----------|-------|\n")
	if counts["critical"] > 0 {
		fmt.Fprintf(w, "| 🔴 Critical | %d |\n", counts["critical"])
	}
	if counts["high"] > 0 {
		fmt.Fprintf(w, "| 🟠 High | %d |\n", counts["high"])
	}
	if counts["medium"] > 0 {
		fmt.Fprintf(w, "| 🟡 Medium | %d |\n", counts["medium"])
	}
	if counts["info"] > 0 {
		fmt.Fprintf(w, "| 🔵 Info | %d |\n", counts["info"])
	}
	fmt.Fprintf(w, "\n---\n\n")

	fmt.Fprintf(w, "## Findings\n\n")
	fmt.Fprintf(w, "| Sev | Code | Resource | Title | Remediation |\n")
	fmt.Fprintf(w, "|-----|------|----------|-------|-------------|\n")
	for _, f := range findings {
		icon := sevIcon[f.Severity]
		fmt.Fprintf(w, "| %s | %s | `%s` | %s | %s |\n",
			icon, f.Code, f.Resource, f.Title, f.Remediation)
	}

	if total == 0 {
		fmt.Fprintf(w, "\n✅ No security findings detected.\n")
	}

	return nil
}

// ExportJSON writes the graph as a JSON document for programmatic consumption.
// Includes summary statistics for threats, costs, drift, and diff.
func ExportJSON(w io.Writer, g *graph.Graph) error {
	elements := graph.BuildElements(g)

	// Build summary stats
	threatCounts := map[string]int{"critical": 0, "high": 0, "medium": 0, "info": 0}
	diffCounts := map[string]int{"added": 0, "removed": 0, "updated": 0}
	var totalCost float64
	var driftCount int

	for _, n := range g.Nodes {
		if n.ThreatMaxSeverity != "" {
			threatCounts[n.ThreatMaxSeverity]++
		}
		if n.ChangeType != "" {
			diffCounts[string(n.ChangeType)]++
		}
		totalCost += n.MonthlyCost
		if n.DriftStatus != "" {
			driftCount++
		}
	}

	totalThreats := 0
	for _, v := range threatCounts {
		totalThreats += v
	}

	resp := struct {
		Elements  []graph.Element `json:"elements"`
		NodeCount int             `json:"nodeCount"`
		EdgeCount int             `json:"edgeCount"`
		Summary   *jsonSummary    `json:"summary,omitempty"`
	}{
		Elements:  elements,
		NodeCount: len(g.Nodes),
		EdgeCount: len(g.Edges),
	}

	// Only include summary if there's data
	if totalThreats > 0 || totalCost > 0 || driftCount > 0 ||
		diffCounts["added"]+diffCounts["removed"]+diffCounts["updated"] > 0 {
		resp.Summary = &jsonSummary{}
		if totalThreats > 0 {
			resp.Summary.Threats = &jsonThreatSummary{
				Total:    totalThreats,
				Critical: threatCounts["critical"],
				High:     threatCounts["high"],
				Medium:   threatCounts["medium"],
				Info:     threatCounts["info"],
			}
		}
		if totalCost > 0 {
			resp.Summary.Cost = &jsonCostSummary{
				MonthlyCost: totalCost,
			}
		}
		if driftCount > 0 {
			resp.Summary.Drift = &jsonDriftSummary{
				DriftedResources: driftCount,
			}
		}
		if diffCounts["added"]+diffCounts["removed"]+diffCounts["updated"] > 0 {
			resp.Summary.Diff = &jsonDiffSummary{
				Added:   diffCounts["added"],
				Removed: diffCounts["removed"],
				Updated: diffCounts["updated"],
			}
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

type jsonSummary struct {
	Threats *jsonThreatSummary `json:"threats,omitempty"`
	Cost    *jsonCostSummary   `json:"cost,omitempty"`
	Drift   *jsonDriftSummary  `json:"drift,omitempty"`
	Diff    *jsonDiffSummary   `json:"diff,omitempty"`
}

type jsonThreatSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Info     int `json:"info"`
}

type jsonCostSummary struct {
	MonthlyCost float64 `json:"monthlyCost"`
}

type jsonDriftSummary struct {
	DriftedResources int `json:"driftedResources"`
}

type jsonDiffSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Updated int `json:"updated"`
}

func ExportHTML(w io.Writer, g *graph.Graph, resolver *icons.Resolver) error {
	elements := graph.BuildElements(g)
	elemJSON, err := json.MarshalIndent(elements, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising elements: %w", err)
	}

	tourJSON, err := json.Marshal(g.TourSteps)
	if err != nil {
		return fmt.Errorf("serialising tour steps: %w", err)
	}
	if g.TourSteps == nil {
		tourJSON = []byte("[]")
	}

	inlineJS, offline := loadBundledJS()
	return htmlTemplate.Execute(w, templateData{
		Elements:      string(elemJSON),
		Offline:       offline,
		InlineJS:      inlineJS,
		TourStepsJSON: string(tourJSON),
	})
}

func loadBundledJS() (string, bool) {
	// Read from the go:embed FS populated by `make bundle`.
	// Files live at js/<name> inside bundleFS (defined in bundle.go).
	// If any file is missing (fresh clone, bundle not run yet),
	// we return ("", false) and the template falls back to CDN tags.
	files := []string{
		"cytoscape.min.js",
		"dagre.min.js",
		"cytoscape-dagre.min.js",
		"cytoscape-node-html-label.min.js",
		"cytoscape-expand-collapse.min.js",
	}
	var combined []byte
	for _, f := range files {
		b, err := iofs.ReadFile(bundleFS, f)
		if err != nil {
			// File not present — bundle step hasn't been run yet.
			// Caller will use CDN fallback (requires internet).
			return "", false
		}
		combined = append(combined, b...)
		combined = append(combined, '\n')
	}
	return string(combined), true
}

// ── Abbreviations ────────────────────────────────────────────────────────────

// ── Template ─────────────────────────────────────────────────────────────────

type templateData struct {
	Elements      string
	Offline       bool
	InlineJS      string
	TourStepsJSON string // JSON array of tour steps; "[]" when none
}

var htmlTemplate = template.Must(template.New("d").Parse(htmlSrc))

const htmlSrc = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>TF-Lens</title>
<style>
:root{
  --bg-body:#F0F2F5;--bg-bar:#232F3E;--bg-panel:#FFFFFF;--bg-card:#FFFFFF;
  --text-primary:#1A202C;--text-secondary:#4A5568;--text-muted:#718096;
  --border:#E2E8F0;--sb-bg:rgba(255,255,255,0.85);
}
body.dark{
  --bg-body:#0D1117;--bg-bar:#161B22;--bg-panel:#161B22;--bg-card:#1C2128;
  --text-primary:#E6EDF3;--text-secondary:#8B949E;--text-muted:#6E7681;
  --border:#30363D;--sb-bg:rgba(22,27,34,0.9);
}
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}

/* ── Thin custom scrollbar ── */
::-webkit-scrollbar{width:6px}
::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:#CBD5E0;border-radius:3px}
::-webkit-scrollbar-thumb:hover{background:#A0AEC0}
*{scrollbar-width:thin;scrollbar-color:#CBD5E0 transparent}

body{
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;
  background:var(--bg-body);
  display:flex;flex-direction:column;height:100vh;overflow:hidden;
  color:var(--text-primary);
  transition:background .2s,color .2s;
}

/* ── Toolbar ── */
#bar{
  height:52px;min-height:52px;
  display:flex;align-items:center;gap:8px;padding:0 16px;
  background:#232F3E;border-bottom:3px solid #FF9900;
  flex-shrink:0;z-index:100;
}
#dashboard{
  display:flex;align-items:center;gap:8px;padding:6px 16px;
  background:var(--bg-bar);border-bottom:1px solid #2D3748;
  flex-shrink:0;flex-wrap:wrap;
}
.dp{
  display:inline-flex;align-items:center;gap:5px;
  padding:3px 10px;border-radius:12px;
  background:#2D3748;color:#E2E8F0;
  font-size:11px;font-weight:600;cursor:pointer;
  border:1px solid #4A5568;
  transition:background .15s;
}
.dp:hover{background:#4A5568}
#dark-toggle{
  margin-left:auto;padding:4px 10px;
  background:transparent;border:1px solid #4A5568;
  border-radius:6px;color:#A0AEC0;cursor:pointer;font-size:12px;
}
#dark-toggle:hover{background:#2D3748;color:#E2E8F0}
#filter-chips{
  display:flex;gap:4px;flex-wrap:wrap;padding:0 16px 6px;
  background:var(--bg-bar);
}
.fchip{
  display:inline-flex;align-items:center;gap:4px;
  padding:2px 8px;border-radius:10px;
  background:#3182CE;color:#FFF;font-size:10px;font-weight:600;cursor:pointer;
}
.fchip:after{content:'×';margin-left:2px;opacity:.7}
.fchip:hover:after{opacity:1}
#minimap{
  position:absolute;bottom:44px;right:12px;
  width:180px;height:110px;
  background:var(--bg-panel);border:1px solid var(--border);
  border-radius:6px;overflow:hidden;z-index:50;
  box-shadow:0 2px 8px rgba(0,0,0,.15);
}
#minimap canvas{width:100%;height:100%}
#minimap-vp{
  position:absolute;border:2px solid #FF9900;border-radius:2px;
  pointer-events:none;
}
#minimap-toggle{
  position:absolute;bottom:44px;right:200px;
  background:var(--sb-bg);border:1px solid var(--border);
  border-radius:6px;padding:3px 7px;font-size:11px;
  cursor:pointer;z-index:51;color:var(--text-secondary);
}
#tour-overlay{
  position:fixed;inset:0;background:rgba(0,0,0,.6);
  z-index:200;display:none;align-items:flex-end;justify-content:center;
  padding-bottom:80px;
}
#tour-overlay.active{display:flex}
#tour-card{
  background:var(--bg-panel);border-radius:12px;padding:24px;
  max-width:440px;width:90%;box-shadow:0 8px 32px rgba(0,0,0,.4);
  position:relative;
}
#tour-step-num{
  font-size:10px;font-weight:700;color:var(--text-muted);
  letter-spacing:.8px;text-transform:uppercase;margin-bottom:6px;
}
#tour-title{font-size:18px;font-weight:700;color:var(--text-primary);margin-bottom:10px}
#tour-narration{font-size:13px;line-height:1.6;color:var(--text-secondary);margin-bottom:20px}
#tour-controls{display:flex;gap:8px;justify-content:flex-end}
.tour-btn{
  padding:6px 16px;border-radius:6px;border:none;cursor:pointer;
  font-size:12px;font-weight:600;
}
#tour-prev{background:var(--border);color:var(--text-primary)}
#tour-next{background:#FF9900;color:#FFF}
#tour-exit{
  position:absolute;top:12px;right:14px;
  background:none;border:none;cursor:pointer;
  font-size:18px;color:var(--text-muted);
}
#tour-start-btn{display:none}
#logo{
  display:flex;align-items:center;gap:7px;
  font-size:15px;font-weight:700;color:#F7FAFC;
  letter-spacing:-.3px;white-space:nowrap;margin-right:4px;
}
.sw{position:relative}
.sw svg{position:absolute;left:9px;top:50%;transform:translateY(-50%);pointer-events:none;color:#718096}
#q{
  width:220px;padding:6px 10px 6px 30px;
  border-radius:6px;border:1px solid #2D3748;
  background:#2D3748;color:#E2E8F0;font-size:13px;outline:none;
  transition:border-color .15s,background .15s;
}
#q:focus{border-color:#FF9900;background:#1A202C}
#q::placeholder{color:#4A5568}
#qx{
  position:absolute;right:28px;top:50%;transform:translateY(-50%);
  color:#4A5568;font-size:16px;cursor:pointer;display:none;
  width:18px;height:18px;text-align:center;line-height:18px;
  border-radius:50%;transition:color .12s,background .12s;
}
#qx:hover{color:#F7FAFC;background:rgba(255,255,255,.12)}
#qc{
  position:absolute;right:8px;top:50%;transform:translateY(-50%);
  font-size:9px;color:#718096;display:none;pointer-events:none;
  font-weight:600;
}
.bg{display:flex;gap:1px}
.btn{
  height:30px;padding:0 11px;border-radius:5px;
  border:1px solid #2D3748;background:#2D3748;color:#CBD5E0;
  font-size:12px;font-weight:600;cursor:pointer;
  display:flex;align-items:center;gap:4px;white-space:nowrap;
  transition:background .12s,color .12s,border-color .12s;
}
.btn:hover{background:#4A5568;color:#F7FAFC;border-color:#4A5568}
.btn-p:hover{background:#FF9900;border-color:#FF9900;color:#1A202C}
.vsp{width:1px;height:20px;background:#2D3748;margin:0 4px;flex-shrink:0}
#leg{display:flex;align-items:center;margin-left:auto}
.lg{display:flex;align-items:center;gap:10px;padding:0 10px}
.lg+.lg{border-left:1px solid #2D3748}
.li{display:flex;align-items:center;gap:5px;font-size:11px;color:#718096;white-space:nowrap}
.ld{width:10px;height:10px;border-radius:2px;flex-shrink:0}

/* ── Canvas ── */
#cy{
  flex:1;
  background-color:#F0F2F5;
  background-image:
    linear-gradient(rgba(160,174,192,.2) 1px,transparent 1px),
    linear-gradient(90deg,rgba(160,174,192,.2) 1px,transparent 1px);
  background-size:20px 20px;
}

/* ── Node card (HTML label) ── */
.nc{
  width:80px;height:80px;
  background:#FFF;border-radius:10px;
  border:1.5px solid #E2E8F0;
  display:flex;flex-direction:column;
  align-items:center;justify-content:center;
  gap:0;
  box-shadow:0 1px 3px rgba(0,0,0,.07),0 1px 2px rgba(0,0,0,.04);
  cursor:pointer;user-select:none;
  transition:box-shadow .15s,transform .1s;
  position:relative;overflow:hidden;
}
/* top accent stripe per category */
.nc::before{
  content:'';position:absolute;top:0;left:0;right:0;height:3px;
  border-radius:10px 10px 0 0;
}
.nc--networking{border-color:#BEE3F8}.nc--networking::before{background:#147EBA}
.nc--compute   {border-color:#FDDCB5}.nc--compute::before   {background:#ED7100}
.nc--storage   {border-color:#C6F6D5}.nc--storage::before   {background:#3F8624}
.nc--security  {border-color:#FED7D7}.nc--security::before  {background:#DD344C}
.nc--messaging {border-color:#E9D8FD}.nc--messaging::before {background:#8C4FFF}
.nc--unknown   {border-color:#E2E8F0}.nc--unknown::before   {background:#A0AEC0}

/* badge inside card */
.nc__b{
  width:48px;height:48px;border-radius:9px;
  display:grid;place-items:center;
}
.nc--networking .nc__b{background:#EBF8FF}
.nc--compute    .nc__b{background:#FFFAF0}
.nc--storage    .nc__b{background:#F0FFF4}
.nc--security   .nc__b{background:#FFF5F5}
.nc--messaging  .nc__b{background:#FAF5FF}
.nc--unknown    .nc__b{background:#F7FAFC}

/* abbreviation text */
.nc__t{
  font-family:ui-monospace,'Menlo','Monaco','Consolas',monospace;
  font-size:13px;font-weight:800;letter-spacing:-.3px;
  line-height:1;display:block;text-align:center;
  position:relative;top:-1px;
}
.nc--networking .nc__t{color:#147EBA}
.nc--compute    .nc__t{color:#C05621}
.nc--storage    .nc__t{color:#276749}
.nc--security   .nc__t{color:#C53030}
.nc--messaging  .nc__t{color:#6B46C1}
.nc--unknown    .nc__t{color:#718096}

.nc:hover{
  box-shadow:0 6px 20px rgba(0,0,0,.13),0 2px 6px rgba(0,0,0,.08);
  transform:translateY(-2px) scale(1.03);
}
.nc--added  {outline:2.5px solid #38A169;outline-offset:2px}
.nc--removed{outline:2.5px dashed #E53E3E;outline-offset:2px;opacity:.6}
.nc--updated{outline:2.5px solid #D69E2E;outline-offset:2px}
.nc--drifted{outline:2.5px solid #9F7AEA;outline-offset:2px}
.nc__drift{
  position:absolute;top:4px;right:4px;
  width:15px;height:15px;border-radius:50%;
  display:flex;align-items:center;justify-content:center;
  font-size:9px;font-weight:900;color:#FFF;
  background:#9F7AEA;
  box-shadow:0 1px 3px rgba(0,0,0,.35);line-height:1;
}
.nc--sel    {outline:3px solid #0073BB;outline-offset:2px;box-shadow:0 0 0 5px rgba(0,115,187,.13)}
.nc__threat{
  position:absolute;bottom:4px;right:4px;
  width:15px;height:15px;border-radius:50%;
  display:flex;align-items:center;justify-content:center;
  font-size:9px;font-weight:900;color:#FFF;
  box-shadow:0 1px 3px rgba(0,0,0,.35);line-height:1;
}
.nc__threat--critical{background:#C53030}
.nc__threat--high    {background:#C05621}
.nc__threat--medium  {background:#975A16}
.nc__threat--info    {background:#2B6CB0}
.nc__cost{
  position:absolute;top:4px;left:4px;
  padding:1px 5px;border-radius:8px;
  font-size:8px;font-weight:700;color:#276749;
  background:#F0FFF4;border:1px solid #9AE6B4;
  line-height:1.3;white-space:nowrap;
  box-shadow:0 1px 2px rgba(0,0,0,.12);
}

/* ── Container label (HTML overlay, sits on top border) ── */
.cl{
  /* Positioned by JS to sit exactly on top of compound node border */
  position:absolute;
  white-space:nowrap;
  background:#FFFFFF;
  border:1.5px solid currentColor;
  border-radius:4px;
  padding:2px 8px;
  font-size:10px;font-weight:700;
  letter-spacing:.6px;text-transform:uppercase;
  pointer-events:none;
  box-shadow:0 1px 3px rgba(0,0,0,.08);
}
.cl--networking{color:#147EBA}
.cl--compute   {color:#ED7100}
.cl--storage   {color:#3F8624}
.cl--security  {color:#DD344C}
.cl--messaging {color:#8C4FFF}
.cl--unknown   {color:#718096}

/* ── Statusbar ── */
#sb{
  position:absolute;bottom:14px;left:14px;
  display:flex;gap:8px;z-index:10;pointer-events:none;
}
.sp{
  background:rgba(255,255,255,.72);
  backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px);
  border:1px solid rgba(226,232,240,.6);
  border-radius:20px;padding:5px 12px;
  font-size:11px;color:#4A5568;white-space:nowrap;
  box-shadow:0 2px 8px rgba(0,0,0,.06);
  transition:background .2s,box-shadow .2s;
}
.sp b{color:#1A202C;font-weight:700}

/* ── Diff banner ── */
#diffbar{
  position:absolute;top:60px;left:50%;transform:translateX(-50%);
  display:none;align-items:center;gap:6px;
  background:rgba(255,255,255,.95);
  border:1px solid #E2E8F0;border-radius:20px;
  padding:5px 14px;font-size:11px;font-weight:600;
  box-shadow:0 2px 8px rgba(0,0,0,.1);
  z-index:50;pointer-events:none;white-space:nowrap;
}
#diffbar.show{display:flex}
.db-item{display:flex;align-items:center;gap:4px;color:#4A5568}
.db-added  {color:#276749}
.db-removed{color:#C53030}
.db-updated{color:#975A16}
.db-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}

/* ── Detail panel ── */
#panel{
  position:absolute;right:0;top:55px;bottom:0;width:320px;min-width:220px;max-width:60vw;
  background:#FFF;border-left:1px solid #E2E8F0;
  box-shadow:-4px 0 20px rgba(0,0,0,.06);
  display:flex;flex-direction:column;
  transform:translateX(100%);
  transition:transform .22s cubic-bezier(.4,0,.2,1);
  z-index:30;
}
#panel-resize{
  position:absolute;left:-4px;top:0;bottom:0;width:8px;
  cursor:col-resize;z-index:31;
}
#panel-resize:hover,#panel-resize.active{
  background:linear-gradient(90deg,transparent 2px,#FF9900 2px,#FF9900 4px,transparent 4px);
}
#panel.open{transform:translateX(0)}
#ph{
  padding:16px;background:#1A202C;color:#F7FAFC;
  flex-shrink:0;display:flex;gap:10px;align-items:flex-start;
}
#phi{
  width:40px;height:40px;border-radius:8px;
  display:grid;place-items:center;
  font-family:ui-monospace,'Menlo',monospace;
  font-size:13px;font-weight:800;color:#FFF;flex-shrink:0;
}
#phn{font-size:14px;font-weight:600;word-break:break-all;line-height:1.3;flex:1;min-width:0}
#pht{font-size:10px;color:#718096;margin-top:3px;font-family:ui-monospace,'Menlo',monospace}
#phx{
  background:none;border:none;color:#4A5568;
  cursor:pointer;font-size:22px;line-height:1;
  padding:0;flex-shrink:0;transition:color .12s;
}
#phx:hover{color:#F7FAFC}
#pb{flex:1;overflow-y:auto;padding:14px 16px;scroll-behavior:smooth}
.pa{margin-bottom:14px}
.pk{font-size:10px;color:#A0AEC0;text-transform:uppercase;letter-spacing:.8px;font-weight:700;margin-bottom:4px}
.pv{font-size:12px;color:#2D3748;line-height:1.5;word-break:break-all}
.pc{font-family:ui-monospace,'Menlo',monospace;font-size:11px;background:#EDF2F7;padding:3px 7px;border-radius:4px;color:#2B6CB0;display:inline-block}
.pd{height:1px;background:#EDF2F7;margin:14px 0}
.badge{display:inline-flex;align-items:center;gap:4px;padding:3px 9px;border-radius:20px;font-size:11px;font-weight:700}
.bn{background:#EBF8FF;color:#2B6CB0}
.bc{background:#FFFAF0;color:#C05621}
.bs{background:#F0FFF4;color:#276749}
.bse{background:#FFF5F5;color:#C53030}
.bm{background:#FAF5FF;color:#6B46C1}
.bu{background:#EDF2F7;color:#4A5568}
.cb{display:inline-flex;align-items:center;gap:5px;padding:3px 9px;border-radius:4px;font-size:11px;font-weight:700}
.ca{background:#F0FFF4;color:#276749;border:1px solid #9AE6B4}
.cr{background:#FFF5F5;color:#C53030;border:1px solid #FEB2B2}
.cu{background:#FFFFF0;color:#975A16;border:1px solid #F6E05E}
.attr-table{width:100%;border-collapse:collapse;font-size:11px;margin-top:6px}
.attr-table tr{border-bottom:1px solid #EDF2F7}
.attr-table tr:last-child{border-bottom:none}
.attr-table td{padding:4px 0;vertical-align:top}
.attr-table td:first-child{color:#718096;width:35%;padding-right:8px;word-break:break-word}

/* ── Button active state ── */
.btn:active{transform:scale(.95)}

/* ── Keyboard help overlay ── */
#khelp{
  display:none;position:fixed;inset:0;z-index:200;
  background:rgba(0,0,0,.55);backdrop-filter:blur(4px);-webkit-backdrop-filter:blur(4px);
  align-items:center;justify-content:center;
}
#khelp.show{display:flex}
#khelp-box{
  background:#FFF;border-radius:12px;padding:28px 32px;
  box-shadow:0 20px 60px rgba(0,0,0,.25);max-width:380px;width:90%;
  animation:kfadeIn .18s ease-out;
}
@keyframes kfadeIn{from{opacity:0;transform:scale(.95)}to{opacity:1;transform:scale(1)}}
#khelp-box h3{font-size:14px;color:#1A202C;margin-bottom:16px;font-weight:700}
.krow{display:flex;align-items:center;justify-content:space-between;padding:6px 0;border-bottom:1px solid #EDF2F7}
.krow:last-child{border-bottom:none}
.kkey{
  display:inline-flex;align-items:center;justify-content:center;
  min-width:28px;height:24px;padding:0 8px;
  background:#EDF2F7;border:1px solid #E2E8F0;border-radius:5px;
  font-size:11px;font-weight:700;font-family:ui-monospace,monospace;color:#2D3748;
}
.kdesc{font-size:12px;color:#4A5568}
</style>
</head>
<body>

<div id="bar">
  <div id="logo">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#FF9900" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
    TF-Lens
  </div>
  <div class="sw">
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
    <input id="q" type="text" placeholder="Search resources…" oninput="doSearch(this.value)" autocomplete="off">
    <span id="qx" onclick="clearSearch()" title="Clear">×</span>
    <span id="qc"></span>
  </div>
  <div class="bg">
    <button class="btn btn-p" onclick="fitG()" title="F">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/></svg>
      Fit
    </button>
    <button class="btn" onclick="cy.zoom(cy.zoom()*1.3)" title="+">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="11" y1="8" x2="11" y2="14"/><line x1="8" y1="11" x2="14" y2="11"/></svg>
    </button>
    <button class="btn" onclick="cy.zoom(cy.zoom()*.77)" title="-">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="8" y1="11" x2="14" y2="11"/></svg>
    </button>
  </div>
  <div class="vsp"></div>
  <div class="bg" id="view-toggle" style="display:none">
    <button class="btn" id="btn-deps" onclick="setView('deps')" style="background:#4A5568;color:#F7FAFC;border-color:#4A5568">Dependencies</button>
    <button class="btn" id="btn-flow" onclick="setView('flow')">Flow</button>
    <button class="btn" id="btn-both" onclick="setView('both')">Both</button>
  </div>
  <div class="vsp" id="view-sep" style="display:none"></div>
  <div id="leg">
    <div class="lg">
      <div class="li"><div class="ld" style="background:#147EBA"></div>Network</div>
      <div class="li"><div class="ld" style="background:#ED7100"></div>Compute</div>
      <div class="li"><div class="ld" style="background:#3F8624"></div>Storage</div>
      <div class="li"><div class="ld" style="background:#DD344C"></div>Security</div>
      <div class="li"><div class="ld" style="background:#8C4FFF"></div>Messaging</div>
    </div>
    <div class="lg">
      <div class="li"><div class="ld" style="background:#38A169"></div>Added</div>
      <div class="li"><div class="ld" style="background:#E53E3E;opacity:.7"></div>Removed</div>
      <div class="li"><div class="ld" style="background:#D69E2E"></div>Changed</div>
      <div class="li"><div class="ld" style="background:#9F7AEA"></div>Drifted</div>
    </div>
    <div class="lg" id="flow-legend" style="display:none">
      <div class="li"><div class="ld" style="background:#3182CE"></div>Ingress</div>
      <div class="li"><div class="ld" style="background:#38A169"></div>Data</div>
      <div class="li"><div class="ld" style="background:#D69E2E"></div>Event</div>
    </div>
  </div>
  <button id="tour-start-btn" class="btn" onclick="startTour()" style="display:none;background:#FF9900;color:#1A202C;font-weight:700">&#x25B6; Start Tour</button>
  <button id="dark-toggle" onclick="doToggleDark()" title="Toggle dark mode">☀</button>
</div>

<div id="dashboard"></div>
<div id="filter-chips" style="display:none"></div>

<div id="cy" style="position:relative"></div>
<button id="minimap-toggle" onclick="toggleMinimap()" title="M">⊞ Map</button>
<div id="minimap" style="display:none">
  <canvas id="minimap-canvas"></canvas>
  <div id="minimap-vp"></div>
</div>

<div id="sb">
  <div class="sp" id="sc"></div>
  <div class="sp" id="sz" style="display:none"></div>
  <div class="sp" id="sh" style="display:none">Press <b>Esc</b> to clear &nbsp;·&nbsp; <b>F</b> to fit &nbsp;·&nbsp; <b>?</b> help</div>
</div>

<div id="diffbar">
  <span style="color:#718096;font-size:10px;margin-right:4px">DIFF</span>
  <span class="db-item db-added"  id="db-a"><span class="db-dot" style="background:#38A169"></span><span id="db-ac">0</span> added</span>
  <span style="color:#CBD5E0">·</span>
  <span class="db-item db-removed" id="db-r"><span class="db-dot" style="background:#E53E3E"></span><span id="db-rc">0</span> removed</span>
  <span style="color:#CBD5E0">·</span>
  <span class="db-item db-updated" id="db-u"><span class="db-dot" style="background:#D69E2E"></span><span id="db-uc">0</span> changed</span>
</div>

<div id="panel">
  <div id="panel-resize"></div>
  <div id="ph">
    <div id="phi">?</div>
    <div style="flex:1;min-width:0">
      <div id="phn">Resource</div>
      <div id="pht"></div>
    </div>
    <button id="phx" onclick="closePanel()" title="Esc">×</button>
  </div>
  <div id="pb"></div>
</div>

<div id="tour-overlay">
  <div id="tour-card">
    <button id="tour-exit" onclick="exitTour()">&#xD7;</button>
    <div id="tour-step-num"></div>
    <div id="tour-title"></div>
    <div id="tour-narration"></div>
    <div id="tour-controls">
      <button class="tour-btn" id="tour-prev" onclick="prevTourStep()">&#x2190; Prev</button>
      <button class="tour-btn" id="tour-next" onclick="nextTourStep()">Next &#x2192;</button>
    </div>
  </div>
</div>

<div id="khelp" onclick="if(event.target===this)toggleHelp()">
  <div id="khelp-box">
    <h3>Keyboard Shortcuts</h3>
    <div class="krow"><span class="kdesc">Fit diagram to screen</span><span class="kkey">F</span></div>
    <div class="krow"><span class="kdesc">Zoom in</span><span class="kkey">+</span></div>
    <div class="krow"><span class="kdesc">Zoom out</span><span class="kkey">-</span></div>
    <div class="krow"><span class="kdesc">Clear selection & search</span><span class="kkey">Esc</span></div>
    <div class="krow"><span class="kdesc">Focus search box</span><span class="kkey">/</span></div>
    <div class="krow"><span class="kdesc">Show this help</span><span class="kkey">?</span></div>
    <div class="krow"><span class="kdesc">Toggle minimap</span><span class="kkey">M</span></div>
  </div>
</div>

{{if .Offline}}
<script>{{.InlineJS}}</script>
{{else}}
<script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape/3.28.1/cytoscape.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/dagre/0.8.5/dagre.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/cytoscape-dagre@2.5.0/cytoscape-dagre.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/cytoscape-node-html-label@1.2.1/dist/cytoscape-node-html-label.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/cytoscape-expand-collapse@4.1.0/cytoscape-expand-collapse.min.js"></script>
{{end}}

<script>
(function(){
if(typeof cytoscape==='undefined'){
  document.body.innerHTML='<div style="padding:40px;color:#C53030;font-size:14px">⚠️ Cytoscape.js failed to load. Check your internet connection.</div>';
  return;
}

function fmtCost(n){
  if(n<0.01) return '<$0.01';
  if(n<1) return '$'+n.toFixed(2);
  if(n<1000) return '$'+n.toFixed(2);
  return '$'+n.toFixed(0).replace(/\B(?=(\d{3})+(?!\d))/g,',');
}

var CAT={
  networking:{label:'Networking',color:'#147EBA',badge:'bn'},
  compute:   {label:'Compute',   color:'#ED7100',badge:'bc'},
  storage:   {label:'Storage',   color:'#3F8624',badge:'bs'},
  security:  {label:'Security',  color:'#DD344C',badge:'bse'},
  messaging: {label:'Messaging', color:'#8C4FFF',badge:'bm'},
  unknown:   {label:'Unknown',   color:'#A0AEC0',badge:'bu'},
};

var ELEMENTS = {{.Elements}};
var TOUR_STEPS = {{.TourStepsJSON}};

// ── Dark mode ────────────────────────────────────────────────────────────
(function(){
  var saved = localStorage.getItem('tflens-dark');
  if(saved === '1') document.body.classList.add('dark');
})();
window.doToggleDark = function(){
  var on = document.body.classList.toggle('dark');
  localStorage.setItem('tflens-dark', on ? '1' : '0');
  var btn = document.getElementById('dark-toggle');
  if(btn) btn.textContent = on ? '🌙' : '☀';
};
(function(){
  var btn = document.getElementById('dark-toggle');
  if(btn) btn.textContent = document.body.classList.contains('dark') ? '🌙' : '☀';
})();

// ── Cytoscape styles: layout geometry only, no visuals on leaf nodes ──────
var cy = cytoscape({
  container: document.getElementById('cy'),
  elements: ELEMENTS,
  style: [
    {
      selector: 'node',
      style: {
        'shape': 'roundrectangle',
        'width': '80px', 'height': '80px',
        'background-opacity': 0,
        'border-width': 0,
        'label': 'data(label)',
        'text-valign': 'bottom', 'text-halign': 'center',
        'text-margin-y': '10px',
        'font-size': '11.5px',
        'font-family': "-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif",
        'font-weight': '600',
        'color': '#2D3748',
        'text-wrap': 'ellipsis',
        'text-max-width': '100px',
      }
    },
    {
      // Compound containers: visible box, NO label (we render label via HTML overlay)
      selector: '$node > node',
      style: {
        'shape': 'roundrectangle',
        'padding': '48px',
        'background-image': 'none',
        'background-color': '#FAFCFF',
        'background-opacity': 0.6,
        'border-width': '1.5px',
        'border-style': 'dashed',
        'border-color': '#147EBA',
        'label': '',  // hide native label — HTML overlay handles it
      }
    },
    {
      selector: 'edge',
      style: {
        'curve-style': 'taxi',
        'taxi-direction': 'auto',
        'taxi-turn': '50px',
        'taxi-turn-min-distance': '10px',
        'width': '1.2px',
        'line-color': '#A0AEC0',
        'target-arrow-color': '#718096',
        'target-arrow-shape': 'triangle',
        'arrow-scale': 0.6,
        'source-distance-from-node': '8px',
        'target-distance-from-node': '8px',
        'opacity': 0.85,
        'label': 'data(label)',
        'font-size': '9px',
        'font-family': "-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif",
        'font-weight': '600',
        'color': '#718096',
        'text-background-color': '#F0F2F5',
        'text-background-opacity': 0.9,
        'text-background-padding': '2px',
        'text-rotation': 'autorotate',
        'text-margin-y': '-8px',
      }
    },
    {selector:'edge:selected',    style:{'line-color':'#0073BB','target-arrow-color':'#0073BB','width':'2px','opacity':'1'}},
    {selector:'edge[?flow]',      style:{
        'line-style':'dashed','line-dash-pattern':[6,3],
        'line-color':'#38A169','target-arrow-color':'#38A169',
        'width':'2px','opacity':0.9,'arrow-scale':0.8,
        'label':'data(label)','font-size':'9px','font-weight':'700',
        'color':'#276749','text-background-color':'#F0FFF4',
        'text-background-opacity':0.95,'text-background-padding':'3px',
        'text-rotation':'autorotate','text-margin-y':'-8px',
    }},
    {selector:'edge[flowKind="ingress"]', style:{'line-color':'#3182CE','target-arrow-color':'#3182CE','color':'#2B6CB0','text-background-color':'#EBF8FF'}},
    {selector:'edge[flowKind="event"]',   style:{'line-color':'#D69E2E','target-arrow-color':'#D69E2E','color':'#975A16','text-background-color':'#FFFFF0'}},
    {selector:'edge[flowKind="data"]',    style:{'line-color':'#38A169','target-arrow-color':'#38A169','color':'#276749','text-background-color':'#F0FFF4'}},
    {selector:'.faded',           style:{opacity:0.1}},
    {selector:'.edge-hl',         style:{'line-color':'#0073BB','target-arrow-color':'#0073BB',width:'2px',opacity:1}},
    {selector:'.flow-hidden',     style:{display:'none'}},
    {selector:'.dep-hidden',      style:{display:'none'}},
  ],
  layout: {
    name:'dagre', rankDir:'TB',
    nodeSep:60, rankSep:80, edgeSep:20,
    padding:60, spacingFactor:1.1, animate:false,
  },
  wheelSensitivity: 0.2,
  minZoom: 0.1, maxZoom: 4,
  boxSelectionEnabled: false, selectionType:'single',
});

window.cy = cy;

// ── HTML labels for LEAF nodes ────────────────────────────────────────────
if(typeof cy.nodeHtmlLabel === 'function'){
  cy.nodeHtmlLabel([{
    query: 'node',
    halign:'center', valign:'center',
    halignBox:'center', valignBox:'center',
    tpl: function(d){
      if(d.isParent) return '<div style="display:none"></div>';
      var cat = d.category||'unknown';
      var chg = d.changeType ? ' nc--'+d.changeType : '';
      var threatBadge = '';
      if(d.threatSeverity && d.threatSeverity !== ''){
        var ti = {critical:'!', high:'!', medium:'~', info:'i'}[d.threatSeverity] || '!';
        threatBadge = '<div class="nc__threat nc__threat--'+d.threatSeverity+'" title="'+d.threatSeverity+'">'+ti+'</div>';
      }
      var costBadge = '';
      if(d.monthlyCost && d.monthlyCost > 0){
        costBadge = '<div class="nc__cost" title="$'+d.monthlyCost.toFixed(2)+'/mo">'+fmtCost(d.monthlyCost)+'</div>';
      }
      var driftClass = d.driftStatus ? ' nc--drifted' : '';
      var driftBadge = '';
      if(d.driftStatus){
        driftBadge = '<div class="nc__drift" title="State drift detected">⚡</div>';
      }
      return '<div class="nc nc--'+cat+chg+driftClass+'" data-id="'+d.id+'">'
           + '<div class="nc__b"><span class="nc__t">'+(d.abbrev||'?')+'</span></div>'
           + (d.driftStatus ? driftBadge : threatBadge) + costBadge
           + '</div>';
    }
  }]);
}

// ── Expand-collapse extension ─────────────────────────────────────────────
var ecApi = null;
if(typeof cy.expandCollapse === 'function'){
  ecApi = cy.expandCollapse({
    layoutBy: {name:'dagre',rankDir:'TB',nodeSep:50,rankSep:80},
    fisheye: false,
    animate: true,
    animationDuration: 200,
    undoable: false,
  });
}

// Double-click on a module/parent node to collapse/expand
cy.on('dblclick', 'node', function(evt){
  var n = evt.target;
  if(!n.isParent() || !ecApi) return;
  if(n.data('collapsed')){
    ecApi.expand(n);
    n.data('collapsed', false);
  } else {
    ecApi.collapse(n);
    n.data('collapsed', true);
  }
});

// ── Container labels rendered as absolutely-positioned HTML overlays ──────
// This approach places the label ON the border line rather than inside,
// matching the AWS architecture diagram convention.
function renderContainerLabels(){
  var container = document.getElementById('cy');
  // Remove old labels
  container.querySelectorAll('.cl').forEach(function(el){ el.remove(); });

  var pan  = cy.pan();
  var zoom = cy.zoom();

  cy.nodes().filter(function(n){ return n.isParent(); }).forEach(function(n){
    var bb  = n.boundingBox();
    var cat = n.data('category') || 'networking';

    // Convert graph coords → screen coords
    var screenX = (bb.x1 + (bb.x2-bb.x1)/2) * zoom + pan.x;
    var screenY = bb.y1 * zoom + pan.y;

    var el = document.createElement('div');
    el.className = 'cl cl--'+cat;
    el.textContent = n.data('label');

    // Position: horizontally centered on the top border line,
    // vertically centered on the border (half above, half below)
    el.style.position = 'absolute';
    el.style.left     = screenX + 'px';
    el.style.top      = screenY + 'px';
    el.style.transform = 'translate(-50%, -50%)';
    el.style.zIndex    = '20';

    // Scale font and padding with zoom level
    var fz = Math.max(8, Math.min(13, 10 * zoom));
    el.style.fontSize   = fz + 'px';
    el.style.padding    = Math.max(1, 2*zoom) + 'px ' + Math.max(4, 8*zoom) + 'px';

    container.appendChild(el);
  });
}

cy.on('render', renderContainerLabels);
cy.on('pan zoom', renderContainerLabels);
renderContainerLabels();

// ── Summary dashboard ────────────────────────────────────────────────────
(function(){
  var dash = document.getElementById('dashboard');
  if(!dash) return;

  var leafCount = cy.nodes().filter(function(n){ return !n.isParent(); }).length;
  var modCount  = cy.nodes().filter(function(n){ return n.data('type')==='module'; }).length;

  function pill(icon, text, onclick){
    var d = document.createElement('div');
    d.className = 'dp';
    d.innerHTML = icon + ' ' + text;
    if(onclick) d.onclick = onclick;
    return d;
  }

  dash.appendChild(pill('📦', leafCount + ' Resources'));

  if(modCount > 0){
    dash.appendChild(pill('🧩', modCount + ' Modules'));
  }

  // Threat pill
  var tc = {critical:0, high:0, medium:0};
  cy.nodes().forEach(function(n){
    var s = n.data('threatSeverity');
    if(s && tc[s] !== undefined) tc[s]++;
  });
  var totalThreats = tc.critical + tc.high + tc.medium;
  if(totalThreats > 0){
    var parts = [];
    if(tc.critical) parts.push('🔴'+tc.critical);
    if(tc.high)     parts.push('🟠'+tc.high);
    if(tc.medium)   parts.push('🟡'+tc.medium);
    dash.appendChild(pill('⚠', parts.join(' ') + ' Threats', function(){
      cy.nodes().forEach(function(n){ if(!n.data('threatSeverity')) n.addClass('faded'); else n.removeClass('faded'); });
      cy.edges().forEach(function(e){ e.addClass('faded'); });
    }));
  }

  // Cost pill
  var totalCostDash = 0;
  cy.nodes().forEach(function(n){ totalCostDash += (n.data('monthlyCost')||0); });
  if(totalCostDash > 0){
    dash.appendChild(pill('💰', fmtCost(totalCostDash)+'/mo', function(){
      var costNodes = [];
      cy.nodes().forEach(function(n){ if(n.data('monthlyCost')>0 && !n.isParent()) costNodes.push(n); });
      costNodes.sort(function(a,b){ return b.data('monthlyCost')-a.data('monthlyCost'); });
      cy.nodes().addClass('faded');
      costNodes.forEach(function(n){ n.removeClass('faded'); });
      cy.edges().addClass('faded');
    }));
  }

  // Drift pill
  var driftCountDash = 0;
  cy.nodes().forEach(function(n){ if(n.data('driftStatus')) driftCountDash++; });
  if(driftCountDash > 0){
    dash.appendChild(pill('⚡', driftCountDash + ' Drifted', function(){
      cy.nodes().forEach(function(n){ if(!n.data('driftStatus')) n.addClass('faded'); else n.removeClass('faded'); });
      cy.edges().addClass('faded');
    }));
  }
})();

// ── Statusbar ────────────────────────────────────────────────────────────
var lc = cy.nodes().filter(function(n){ return !n.isParent(); }).length;
var ec = cy.edges().length;
document.getElementById('sc').innerHTML =
  '<b>'+lc+'</b> resources &nbsp;·&nbsp; <b>'+ec+'</b> connections';

// ── Diff banner — count change types from element data ─────────────────
var diffCounts = {added:0, removed:0, updated:0};
ELEMENTS.forEach(function(el){
  if(el.group==='nodes' && el.data.changeType && el.data.changeType!==''){
    if(diffCounts[el.data.changeType] !== undefined){
      diffCounts[el.data.changeType]++;
    }
  }
});
var hasDiff = diffCounts.added + diffCounts.removed + diffCounts.updated > 0;
if(hasDiff){
  document.getElementById('db-ac').textContent = diffCounts.added;
  document.getElementById('db-rc').textContent = diffCounts.removed;
  document.getElementById('db-uc').textContent = diffCounts.updated;
  document.getElementById('diffbar').classList.add('show');
}

// ── Threat summary statusbar pill ─────────────────────────────────────
var tc = {critical:0, high:0, medium:0};
ELEMENTS.forEach(function(el){
  if(el.group==='nodes' && el.data.threatSeverity && tc[el.data.threatSeverity]!==undefined){
    tc[el.data.threatSeverity]++;
  }
});
var totalThreats = tc.critical + tc.high + tc.medium;
if(totalThreats > 0){
  var tp = document.createElement('div');
  tp.className = 'sp';
  var parts = [];
  if(tc.critical>0) parts.push('<span style="color:#C53030;font-weight:700">🔴 '+tc.critical+'</span>');
  if(tc.high>0)     parts.push('<span style="color:#C05621;font-weight:700">🟠 '+tc.high+'</span>');
  if(tc.medium>0)   parts.push('<span style="color:#975A16;font-weight:700">🟡 '+tc.medium+'</span>');
  tp.innerHTML = '⚠&nbsp; ' + parts.join(' · ') + ' &nbsp;threats';
  document.getElementById('sb').appendChild(tp);
}

// ── Cost summary pill ────────────────────────────────────────────────────
var totalCost = 0;
ELEMENTS.forEach(function(el){
  if(el.group==='nodes' && el.data.monthlyCost) totalCost += el.data.monthlyCost;
});
if(totalCost > 0){
  var cp = document.createElement('div');
  cp.className = 'sp';
  cp.innerHTML = '💰&nbsp; <b>' + fmtCost(totalCost) + '</b>/mo';
  document.getElementById('sb').appendChild(cp);
}

// ── Drift summary pill ───────────────────────────────────────────────────
var driftCount = 0;
ELEMENTS.forEach(function(el){
  if(el.group==='nodes' && el.data.driftStatus) driftCount++;
});
if(driftCount > 0){
  var dp = document.createElement('div');
  dp.className = 'sp';
  dp.innerHTML = '🔀&nbsp; <b style="color:#9F7AEA">'+driftCount+'</b> drifted';
  document.getElementById('sb').appendChild(dp);
}

// ── Zoom indicator ──────────────────────────────────────────────────────
var szEl = document.getElementById('sz');
var zoomTimer;
function showZoom(){
  var pct = Math.round(cy.zoom()*100);
  szEl.innerHTML = '🔍&nbsp; <b>'+pct+'%</b>';
  szEl.style.display = '';
  clearTimeout(zoomTimer);
  zoomTimer = setTimeout(function(){ szEl.style.display = 'none'; }, 2000);
}
cy.on('zoom', showZoom);

// ── Fit ──────────────────────────────────────────────────────────────────
function fitG(){ cy.fit(undefined, 60); }
window.fitG = fitG;
setTimeout(fitG, 80);

// ── Selection ────────────────────────────────────────────────────────────
var selId = null;

function highlight(node){
  clearSel();
  selId = node.id();
  var el = document.querySelector('.nc[data-id="'+selId+'"]');
  if(el) el.classList.add('nc--sel');
  cy.elements().addClass('faded');
  node.closedNeighborhood().removeClass('faded');
  node.connectedEdges().addClass('edge-hl').removeClass('faded');
}

function clearSel(){
  cy.elements().removeClass('faded edge-hl');
  document.querySelectorAll('.nc--sel').forEach(function(el){ el.classList.remove('nc--sel'); });
  selId = null;
}

// ── Events ───────────────────────────────────────────────────────────────
cy.on('tap','node',function(evt){
  var n = evt.target;
  if(n.isParent()) return;
  highlight(n);
  openPanel(n.data());
  document.getElementById('sh').style.display = '';
});

cy.on('tap',function(evt){
  if(evt.target===cy){ clearSel(); closePanel(); document.getElementById('sh').style.display='none'; }
});

// ── Panel ────────────────────────────────────────────────────────────────
window.openPanel = function(d){
  var cat = d.category||'unknown';
  var c   = CAT[cat]||CAT.unknown;
  var phi = document.getElementById('phi');
  phi.textContent    = d.abbrev||'?';
  phi.style.background = c.color;
  document.getElementById('phn').textContent = d.label;
  document.getElementById('pht').textContent = d.type;

  var h = '';
  h += '<div class="pa"><div class="pk">Category</div>'
     + '<div class="pv"><span class="badge '+c.badge+'">'+c.label+'</span></div></div>';
  if(d.changeType && d.changeType!==''){
    var ci={added:'✚',removed:'✕',updated:'↻'};
    var cc={added:'ca',removed:'cr',updated:'cu'};
    h += '<div class="pa"><div class="pk">Change</div>'
       + '<div class="pv"><span class="cb '+(cc[d.changeType]||'')+'">'+
         (ci[d.changeType]||'')+' '+d.changeType.toUpperCase()+'</span></div></div>';
  }
  // ── Human context section ─────────────────────────────────────────────
  if(d.humanLabel || d.description || d.owner || d.docsURL){
    h += '<div class="pd"></div>';
    if(d.humanLabel){
      h += '<div class="pa"><div class="pk">Name</div>'
         + '<div class="pv" style="font-weight:700;font-size:14px;color:var(--text-primary)">'+d.humanLabel+'</div></div>';
    }
    if(d.description){
      h += '<div class="pa" style="align-items:flex-start"><div class="pk">What it does</div>'
         + '<div class="pv" style="line-height:1.5;color:var(--text-secondary)">'+d.description+'</div></div>';
    }
    if(d.owner){
      h += '<div class="pa"><div class="pk">Owner</div>'
         + '<div class="pv"><span style="background:#2D3748;color:#E2E8F0;padding:1px 8px;border-radius:10px;font-size:11px">'+d.owner+'</span></div></div>';
    }
    if(d.environment){
      h += '<div class="pa"><div class="pk">Environment</div>'
         + '<div class="pv"><span style="background:#276749;color:#C6F6D5;padding:1px 8px;border-radius:10px;font-size:11px">'+d.environment+'</span></div></div>';
    }
    if(d.docsURL && /^https?:\/\//.test(d.docsURL)){
      h += '<div class="pa"><div class="pk">Docs</div>'
         + '<div class="pv"><a href="'+d.docsURL+'" target="_blank" rel="noopener" style="color:#3182CE;font-size:12px;word-break:break-all">'+d.docsURL+'</a></div></div>';
    }
  }

  // ── AWS Service glossary ──────────────────────────────────────────────
  if(d.glossaryName || d.glossaryOneLiner){
    h += '<div class="pd"></div>';
    h += '<div class="pa" style="align-items:flex-start"><div class="pk" style="color:var(--text-muted)">About this service</div>';
    h += '<div style="margin-top:4px">';
    if(d.glossaryName){
      h += '<div style="font-size:12px;font-weight:700;color:var(--text-primary);margin-bottom:4px">'+d.glossaryName+'</div>';
    }
    if(d.glossaryOneLiner){
      h += '<div style="font-size:11px;color:var(--text-secondary);line-height:1.5">'+d.glossaryOneLiner+'</div>';
    }
    h += '</div></div>';
  }

  h += '<div class="pd"></div>';
  h += '<div class="pa"><div class="pk">Address</div><div class="pv"><span class="pc">'+d.id+'</span></div></div>';
  h += '<div class="pa"><div class="pk">Type</div><div class="pv"><span class="pc">'+d.type+'</span></div></div>';

  // ── Drift ─────────────────────────────────────────────────────────────
  if(d.driftStatus && d.driftChanges && d.driftChanges.length > 0){
    h += '<div class="pd"></div>';
    h += '<div class="pa"><div class="pk" style="color:#9F7AEA">🔀 State drift ('+d.driftChanges.length+' attributes changed)</div>';
    h += '<div style="margin-top:6px">';
    h += '<table class="attr-table" style="font-size:10px">';
    h += '<tr style="border-bottom:2px solid #E9D8FD"><td style="color:#9F7AEA;font-weight:700">Attribute</td><td style="color:#9F7AEA;font-weight:700">Expected</td><td style="color:#9F7AEA;font-weight:700">Actual</td></tr>';
    d.driftChanges.forEach(function(c){
      h += '<tr>';
      h += '<td style="color:#4A5568;font-weight:600">'+c.path+'</td>';
      h += '<td style="color:#276749;background:#F0FFF4;padding:2px 4px;border-radius:3px;font-family:ui-monospace,monospace;font-size:9px">'+c.expected+'</td>';
      h += '<td style="color:#C53030;background:#FFF5F5;padding:2px 4px;border-radius:3px;font-family:ui-monospace,monospace;font-size:9px">'+c.actual+'</td>';
      h += '</tr>';
    });
    h += '</table></div></div>';
  }

  // ── Cost ──────────────────────────────────────────────────────────────
  if(d.monthlyCost && d.monthlyCost > 0){
    h += '<div class="pd"></div>';
    h += '<div class="pa"><div class="pk" style="color:#276749">💰 Cost estimate</div>';
    h += '<div class="pv" style="font-size:16px;font-weight:700;color:#276749;margin-top:4px">'
       + fmtCost(d.monthlyCost) + '<span style="font-size:11px;font-weight:400;color:#718096">/mo</span></div></div>';
  }

  // ── Threat findings ──────────────────────────────────────────────────
  if(d.threatFindings && d.threatFindings.length > 0){
    var sevColors = {critical:'#C53030',high:'#C05621',medium:'#975A16',info:'#2B6CB0'};
    var sevBgs    = {critical:'#FFF5F5',high:'#FFFAF0',medium:'#FFFFF0',info:'#EBF8FF'};
    var sevLabels = {critical:'CRITICAL',high:'HIGH',medium:'MEDIUM',info:'INFO'};
    h += '<div class="pd"></div>';
    h += '<div class="pa"><div class="pk" style="color:'+(sevColors[d.threatSeverity]||'#718096')+'">⚠ Security findings ('+d.threatFindings.length+')</div>';
    h += '<div style="display:flex;flex-direction:column;gap:8px;margin-top:6px">';
    d.threatFindings.forEach(function(f){
      var fc = sevColors[f.severity]||'#718096';
      var fb = sevBgs[f.severity]||'#F7FAFC';
      h += '<div style="background:'+fb+';border:1px solid '+fc+';border-radius:6px;padding:10px;font-size:11px">';
      h += '<div style="display:flex;align-items:center;gap:6px;margin-bottom:6px">';
      h += '<span style="background:'+fc+';color:#FFF;padding:1px 6px;border-radius:3px;font-size:9px;font-weight:800;letter-spacing:.5px">'+(sevLabels[f.severity]||'')+'</span>';
      h += '<span style="color:'+fc+';font-weight:700;font-family:ui-monospace,monospace;font-size:10px">'+f.code+'</span>';
      h += '</div>';
      h += '<div style="color:#1A202C;font-weight:600;margin-bottom:4px">'+f.title+'</div>';
      h += '<div style="color:#4A5568;line-height:1.5;margin-bottom:6px">'+f.detail+'</div>';
      h += '<div style="color:#276749;background:#F0FFF4;border:1px solid #C6F6D5;border-radius:4px;padding:6px 8px;line-height:1.5">';
      h += '<span style="font-weight:700">Fix: </span>'+f.remediation+'</div>';
      h += '</div>';
    });
    h += '</div></div>';
  }

  document.getElementById('pb').innerHTML = h;
  document.getElementById('panel').classList.add('open');
};

window.closePanel = function(){
  document.getElementById('panel').classList.remove('open');
};

// ── Search with filter syntax ─────────────────────────────────────────────
// Supported: type: module: threat: owner:
function parseFilters(raw){
  var filters = [];
  var rest = raw;
  var re = /(type|module|threat|owner):([^\s]+)/g;
  var m;
  while((m = re.exec(raw)) !== null){
    filters.push({key: m[1], val: m[2].toLowerCase()});
    rest = rest.replace(m[0], '').trim();
  }
  return {text: rest.trim().toLowerCase(), filters: filters};
}

function matchesFilters(n, text, filters){
  // Text match
  if(text && !(
    (n.data('label')||'').toLowerCase().includes(text) ||
    (n.data('type') ||'').toLowerCase().includes(text) ||
    (n.data('abbrev')||'').toLowerCase().includes(text) ||
    n.id().toLowerCase().includes(text) ||
    (n.data('humanLabel')||'').toLowerCase().includes(text) ||
    (n.data('description')||'').toLowerCase().includes(text)
  )) return false;
  // Structured filters
  for(var i=0;i<filters.length;i++){
    var f = filters[i];
    if(f.key==='type'    && !(n.data('type')||'').toLowerCase().includes(f.val)) return false;
    if(f.key==='module'  && !(n.id()||'').toLowerCase().includes('module.'+f.val)) return false;
    if(f.key==='threat'  && (n.data('threatSeverity')||'').toLowerCase() !== f.val) return false;
    if(f.key==='owner'   && (n.data('owner')||'').toLowerCase() !== f.val) return false;
  }
  return true;
}

function renderFilterChips(filters){
  var bar = document.getElementById('filter-chips');
  if(!bar) return;
  bar.innerHTML = '';
  if(!filters.length){ bar.style.display='none'; return; }
  bar.style.display='flex';
  filters.forEach(function(f, idx){
    var chip = document.createElement('span');
    chip.className = 'fchip';
    chip.textContent = f.key+':'+f.val;
    chip.onclick = function(){
      var q = document.getElementById('q');
      var val = q.value.replace(f.key+':'+f.val, '').trim();
      q.value = val;
      doSearch(val);
    };
    bar.appendChild(chip);
  });
}

window.doSearch = function(q){
  clearSel();
  var raw = (q||'').trim();
  var qx  = document.getElementById('qx');
  var qc  = document.getElementById('qc');
  qx.style.display = raw ? '' : 'none';

  var parsed = parseFilters(raw);
  renderFilterChips(parsed.filters);

  if(!raw){ cy.elements().removeClass('faded'); qc.style.display='none'; return; }

  var matched = 0;
  var leafCount = 0;
  cy.nodes().forEach(function(n){
    if(n.isParent()) return;
    leafCount++;
    var m = matchesFilters(n, parsed.text, parsed.filters);
    if(m){ n.removeClass('faded'); matched++; } else n.addClass('faded');
  });
  cy.edges().forEach(function(e){
    var ok = !e.source().hasClass('faded') && !e.target().hasClass('faded');
    if(ok) e.removeClass('faded'); else e.addClass('faded');
  });
  qc.textContent = matched+'/'+leafCount;
  qc.style.display = '';
};

window.clearSearch = function(){
  document.getElementById('q').value = '';
  document.getElementById('qx').style.display = 'none';
  document.getElementById('qc').style.display = 'none';
  document.getElementById('filter-chips').style.display = 'none';
  cy.elements().removeClass('faded');
};

// ── Help overlay ─────────────────────────────────────────────────────────
window.toggleHelp = function(){
  document.getElementById('khelp').classList.toggle('show');
};

// ── Keyboard ─────────────────────────────────────────────────────────────
document.addEventListener('keydown',function(e){
  // Close help overlay on Escape
  var helpOpen = document.getElementById('khelp').classList.contains('show');
  if(helpOpen && (e.key==='Escape'||e.key==='Esc')){ toggleHelp(); return; }

  if(e.target.matches('input')){
    if(e.key==='Escape'||e.key==='Esc'){ e.target.blur(); clearSearch(); clearSel(); closePanel(); document.getElementById('sh').style.display='none'; }
    return;
  }
  if(e.key==='Escape'||e.key==='Esc'){
    clearSel(); closePanel(); clearSearch();
    document.getElementById('sh').style.display='none';
  }
  if(e.key==='f'||e.key==='F') fitG();
  if(e.key==='+'||e.key==='=') cy.zoom(cy.zoom()*1.3);
  if(e.key==='-') cy.zoom(cy.zoom()*.77);
  if(e.key==='/'){e.preventDefault(); document.getElementById('q').focus();}
  if(e.key==='?') toggleHelp();
});

// ── Flow view toggle ─────────────────────────────────────────────────────
var hasFlow = ELEMENTS.some(function(el){ return el.group==='edges' && el.data.flow; });
if(hasFlow){
  document.getElementById('view-toggle').style.display='';
  document.getElementById('view-sep').style.display='';
  document.getElementById('flow-legend').style.display='';
}

var currentView = 'deps';
window.setView = function(view){
  currentView = view;
  var btnDeps = document.getElementById('btn-deps');
  var btnFlow = document.getElementById('btn-flow');
  var btnBoth = document.getElementById('btn-both');
  var active = 'background:#4A5568;color:#F7FAFC;border-color:#4A5568';
  var inactive = '';
  btnDeps.style.cssText = view==='deps' ? active : inactive;
  btnFlow.style.cssText = view==='flow' ? active : inactive;
  btnBoth.style.cssText = view==='both' ? active : inactive;

  cy.edges().forEach(function(e){
    var isFlow = e.data('flow');
    e.removeClass('flow-hidden dep-hidden');
    if(view==='deps' && isFlow) e.addClass('flow-hidden');
    if(view==='flow' && !isFlow) e.addClass('dep-hidden');
  });
};

// ── Panel resize ─────────────────────────────────────────────────────────
(function(){
  var handle = document.getElementById('panel-resize');
  var panel  = document.getElementById('panel');
  var dragging = false;
  handle.addEventListener('mousedown', function(e){
    e.preventDefault();
    dragging = true;
    handle.classList.add('active');
    panel.style.transition = 'none';
  });
  document.addEventListener('mousemove', function(e){
    if(!dragging) return;
    var w = window.innerWidth - e.clientX;
    if(w < 220) w = 220;
    if(w > window.innerWidth * 0.6) w = window.innerWidth * 0.6;
    panel.style.width = w + 'px';
  });
  document.addEventListener('mouseup', function(){
    if(!dragging) return;
    dragging = false;
    handle.classList.remove('active');
    panel.style.transition = '';
  });
})();

// ── Minimap ──────────────────────────────────────────────────────────────
var minimapVisible = false;
window.toggleMinimap = function(){
  minimapVisible = !minimapVisible;
  document.getElementById('minimap').style.display = minimapVisible ? '' : 'none';
  if(minimapVisible) initMinimap();
};

function initMinimap(){ drawMinimap(); }
function drawMinimap(){
  var canvas = document.getElementById('minimap-canvas');
  if(!canvas) return;
  var mm = document.getElementById('minimap');
  canvas.width  = mm.offsetWidth;
  canvas.height = mm.offsetHeight;
  var ctx = canvas.getContext('2d');
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  var bb = cy.elements().boundingBox();
  if(!bb || bb.w === 0 || bb.h === 0) return;

  var scaleX = canvas.width  / bb.w;
  var scaleY = canvas.height / bb.h;
  var scale  = Math.min(scaleX, scaleY) * 0.9;
  var offX   = (canvas.width  - bb.w * scale) / 2;
  var offY   = (canvas.height - bb.h * scale) / 2;

  cy.nodes().forEach(function(n){
    if(n.isParent()) return;
    var nbb = n.boundingBox();
    var x = (nbb.x1 - bb.x1) * scale + offX;
    var y = (nbb.y1 - bb.y1) * scale + offY;
    var w = nbb.w * scale;
    var h = nbb.h * scale;
    var cat = n.data('category') || 'unknown';
    var colors = {networking:'#147EBA',compute:'#ED7100',storage:'#3F8624',security:'#DD344C',messaging:'#8C4FFF',unknown:'#A0AEC0'};
    ctx.fillStyle = colors[cat] || '#A0AEC0';
    ctx.fillRect(x, y, Math.max(w, 3), Math.max(h, 3));
  });

  var ext = cy.extent();
  var vpx = (ext.x1 - bb.x1) * scale + offX;
  var vpy = (ext.y1 - bb.y1) * scale + offY;
  var vpw = ext.w * scale;
  var vph = ext.h * scale;
  var vp = document.getElementById('minimap-vp');
  vp.style.left   = Math.max(0, vpx) + 'px';
  vp.style.top    = Math.max(0, vpy) + 'px';
  vp.style.width  = Math.min(vpw, canvas.width)  + 'px';
  vp.style.height = Math.min(vph, canvas.height) + 'px';
}

cy.on('pan zoom', function(){ if(minimapVisible) initMinimap(); });

document.addEventListener('keydown', function(e){
  if(e.target.matches('input')) return;
  if(e.key === 'm' || e.key === 'M') toggleMinimap();
});

// ── Guided Tour ──────────────────────────────────────────────────────────
(function(){
  var steps = TOUR_STEPS || [];
  var cur = 0;

  if(steps.length > 0){
    var btn = document.getElementById('tour-start-btn');
    if(btn) btn.style.display = '';
  }

  function updateHash(){
    if(cur >= 0 && cur < steps.length){
      history.replaceState(null,'','#tour='+(cur+1));
    }
  }

  function showStep(idx){
    if(idx < 0 || idx >= steps.length) return;
    cur = idx;
    var s = steps[idx];
    document.getElementById('tour-step-num').textContent = 'Step '+(idx+1)+' of '+steps.length;
    document.getElementById('tour-title').textContent = s.Title || s.title || '';
    document.getElementById('tour-narration').textContent = s.Narration || s.narration || '';
    document.getElementById('tour-prev').style.visibility = idx === 0 ? 'hidden' : '';
    document.getElementById('tour-next').textContent = idx === steps.length-1 ? 'Finish' : 'Next \u2192';

    var resource = s.Resource || s.resource;
    cy.elements().removeClass('faded tour-spotlight');
    var target = cy.getElementById(resource);
    if(target && target.length){
      cy.nodes().forEach(function(n){ if(n.id()!==resource) n.addClass('faded'); });
      cy.edges().addClass('faded');
      cy.animate({fit:{eles:target.closedNeighborhood(), padding:80}}, {duration:400});
    }
    updateHash();
  }

  window.startTour = function(){
    if(!steps.length) return;
    document.getElementById('tour-overlay').classList.add('active');
    var m = location.hash.match(/#tour=(\d+)/);
    var startIdx = m ? Math.max(0, Math.min(parseInt(m[1],10)-1, steps.length-1)) : 0;
    showStep(startIdx);
  };

  window.exitTour = function(){
    document.getElementById('tour-overlay').classList.remove('active');
    cy.elements().removeClass('faded tour-spotlight');
    history.replaceState(null,'','#');
  };

  window.nextTourStep = function(){
    if(cur >= steps.length-1){ exitTour(); return; }
    showStep(cur+1);
  };

  window.prevTourStep = function(){
    if(cur > 0) showStep(cur-1);
  };

  var initM = location.hash.match(/#tour=(\d+)/);
  if(initM && steps.length > 0){
    setTimeout(function(){ window.startTour(); }, 600);
  }
})();

})();
</script>
</body>
</html>
`
