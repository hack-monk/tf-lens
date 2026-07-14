# PNG/SVG Graph Export — Design

## Problem

Users can only get the graph out of tf-lens as an HTML file (export mode)
or a live page (serve mode). There's no way to drop a static image into a
doc, slide deck, or design tool. Cytoscape.js (already bundled) supports
this natively for PNG; SVG needs one small additional extension.

## Scope

Client-side only. No new Go types, no new CLI flags, no server changes.
Touches the shared `htmlTemplate` in `internal/renderer/export.go`, which
both `export` and `serve` modes render from — so this ships to both modes
in one change.

Out of scope: resolution/scale picker, filename customization, batch
export, non-image formats (PDF etc). None requested; adding them now is
speculative.

## Trigger

- Toolbar button `Export ▾` next to the existing Fit/Zoom buttons
  (`.btn` class, same visual style as `#minimap-toggle` etc).
- Click opens a 2-item menu: **PNG** / **SVG**.
- Keyboard shortcut `E` opens the same menu (same pattern as existing `M`
  → minimap toggle, keydown listener at export.go:1624).

## Export mechanics

- **PNG**: `cy.png({full: true, bg: null, scale: 2})`
  - `full: true` — fits the whole graph regardless of current pan/zoom.
  - `bg: null` — transparent background (per user decision — no theme
    bg baked in).
  - `scale: 2` — fixed retina-quality raster, no UI control for this.
- **SVG**: `cy.svg({full: true, bg: null})` via the `cytoscape-svg`
  extension (not currently bundled — see below). Same full-graph,
  transparent semantics.
- Both trigger a client-side `<a download>` blob. No server round-trip,
  no new endpoint.
- Filename: static `tf-lens-graph.png` / `tf-lens-graph.svg`.

## Bundle changes

Mirrors the existing pattern used for `cytoscape-dagre`,
`cytoscape-node-html-label`, `cytoscape-expand-collapse` exactly:

1. **Makefile** — add `CYTOSCAPE_SVG_VERSION`, download URL, and a
   `bundle-check` entry for the new file.
2. **`internal/renderer/export.go`**, `loadBundledJS()` — add
   `"cytoscape-svg.min.js"` to the required-files list (export.go:226-228).
3. **CDN fallback `<script src>` tag** — add alongside the other 4, for
   the no-bundle dev path (export.go:831-835).
4. **`bundle.go`** — no change needed. `//go:embed js` already embeds the
   whole directory; any new file dropped into `internal/renderer/js/` is
   picked up automatically.

## Testing

Extend `internal/renderer/export_test.go` with assertions that the
generated HTML contains the export button/menu markup and the
`cy.svg(`/`cy.png(` calls — same style as existing template-content
checks in that file. No new test file; this isn't a new package.

## Non-goals (explicitly deferred)

- Scale/resolution picker — fixed at 2x, revisit if users ask.
- Theme-matched background option — transparent only, per decision.
- Export of current viewport vs full graph — full graph only, per decision.
