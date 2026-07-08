# TF-Lens — Onboarding Context Layer & UI Enhancements

**Date:** 2026-07-08  
**Status:** Approved  
**Scope:** Context annotation system, guided tour mode, UI improvements for team onboarding

---

## Problem

TF-Lens already renders accurate AWS architecture diagrams. The gap for team onboarding is understanding:

- **What** a resource is (new joiner doesn't know SQS from MSK)
- **How** resources connect (flow and dependency paths are not self-evident)
- **Why** a resource exists (business context is scattered across Confluence, Notion, and `.tf` comments)

Primary consumers: teams sharing exported HTML with new joiners, or new joiners running `tf-lens serve` locally.

---

## Goals

1. Auto-infer human-readable context from what's already in Terraform (zero-config)
2. Allow teams to override and enrich with a committed `tf-lens.yaml` annotation file
3. Provide an interactive guided tour mode for structured onboarding
4. Improve diagram readability for large infrastructure: minimap, collapsible modules, better search, summary dashboard, dark mode

---

## Out of Scope

- Multi-cloud (Azure/GCP) — separate future spec
- PR comment bot / SARIF export — phase 2
- Compliance framework mapping (CIS/SOC2/PCI) — phase 2
- External wiki integration (Confluence/Notion API pull) — phase 2

---

## Feature Design

### 1. Auto-Inference (zero-config context)

Extend the parser to extract readable context from the existing plan JSON:

| Source | Extracted field |
|---|---|
| `tags.Name` | Node display label |
| `tags.Description` | Short description |
| `tags.Team` / `tags.Owner` | Owner |
| `tags.Environment` | Environment badge |
| Resource `description` field (IAM, SG) | Description |
| Local resource name (`aws_sqs_queue.order_processor`) | Readable label fallback ("order processor") |
| Module path | Breadcrumb ("module.payments.aws_rds_cluster.main") |

No new CLI flags. Context shows wherever it exists.

---

### 2. `tf-lens.yaml` Annotation File

Teams commit a YAML file alongside Terraform. Loaded via `--annotations tf-lens.yaml`.

**Schema:**

```yaml
annotations:
  - resource: <terraform_resource_address>   # e.g. aws_sqs_queue.order_processor
    label: "Human-readable name"
    description: "What it does and why it exists."
    docs: "https://link-to-wiki-or-runbook"
    owner: "team-name"

tour:
  - step: 1
    resource: <terraform_resource_address>
    title: "Step title"
    narration: "What the new joiner should understand at this step."
```

**Merge rules:**
- Auto-inferred values are used as defaults
- `tf-lens.yaml` values override on a per-field basis (not all-or-nothing)
- Missing resources in the annotation file are silently ignored (partial annotation is valid)

**New flag:**
```
--annotations   Path to tf-lens.yaml annotation file (optional)
```

---

### 3. AWS Service Glossary

Built-in static map (`resource_type → {name, one_liner, docs_url}`) covering all 30 resource types TF-Lens already handles. Embedded in the binary — no network call, works fully offline.

Example entry:
```
aws_sqs_queue → {
  name: "Amazon SQS",
  one_liner: "Fully managed message queue. Decouples producers from consumers so neither blocks the other.",
  docs_url: "https://docs.aws.amazon.com/sqs/"
}
```

Displayed in the detail panel below human annotations. New joiners who don't know a resource type get a one-liner without leaving the diagram.

---

### 4. Guided Tour Mode

Defined by the `tour:` block in `tf-lens.yaml`. Steps are ordered and each points to a resource address.

**UI behaviour:**
- Toolbar gets a **"Start Tour"** button (only rendered when tour steps exist)
- On start: dims the full diagram, spotlights the target resource (zoom-to-fit + highlight ring), shows an overlay card with step number, title, and narration
- **Prev / Next / Exit** controls in the overlay card
- Tour state is URL-hash driven: `#tour=3` — shareable link to any step
- Works fully offline in exported HTML

**Serve mode:** tour steps reloaded on each request (respects `--watch`).

---

### 5. Summary Dashboard Panel

Persistent top strip above the canvas. Always visible; collapses to icon row on small viewports.

```
[ 42 Resources ]  [ $234/mo ]  [ 🔴2 🟠3 🟡4 Threats ]  [ ⚡2 Drifted ]  [ 3 Modules ]
```

- Pills only appear when the corresponding data was loaded (no `--cost` → no cost pill)
- Each pill is clickable: activates the relevant overlay and filters to that category
- Clicking cost pill activates cost heatmap; clicking threat pill activates threat overlay filtered to that severity

---

### 6. Minimap

Fixed 180×120px panel, bottom-right corner (above statusbar).

- Full graph rendered at reduced scale
- Viewport indicator rectangle — drag to pan main canvas
- Toggle with `M` key; collapses to a small icon when hidden
- Implemented via Cytoscape viewport API — no new dependency

---

### 7. Better Search

Extend existing search (`/` to focus, result count, clear button) with filter syntax:

| Filter | Example |
|---|---|
| Resource type | `/type:rds` |
| Tag value | `/tag:env=prod` |
| Module | `/module:payments` |
| Threat severity | `/threat:critical` |
| Owner (from annotations) | `/owner:payments-team` |

- Filters are additive
- Active filters shown as removable chips below the search bar
- Non-matching nodes dimmed (not hidden) — spatial context preserved
- Keyboard shortcut `/` still focuses search

---

### 8. Collapsible Module Groups

Terraform modules already render as compound container nodes.

- **Double-click module container** → collapses children into a summary node: "payments [12 resources]"
- **Double-click again** → expands
- Collapse state synced to URL hash: `#collapsed=module.payments,module.auth`
- Exported HTML can be shared with specific modules pre-collapsed
- Implemented via `cytoscape-expand-collapse` extension (~15KB — only new JS dependency)

---

### 9. Dark Mode

- Toggle button in toolbar (☀/🌙)
- State persists in `localStorage`
- Implemented via CSS custom properties — single variable swap, no duplicate style sheets
- All overlays (threat badges, drift outline, cost badge, tour spotlight) readable in both modes
- Exported HTML includes both themes; toggle works offline

---

## Architecture

### New Go packages

```
internal/
├── annotations/     # Parse tf-lens.yaml → AnnotationMap
│                    # Merge into graph.Node after graph build
├── glossary/        # Static map[string]ServiceInfo, go:embed, zero I/O
```

### Changes to existing packages

| Package | Change |
|---|---|
| `internal/graph` | Add fields to `Node`: `Label`, `Description`, `DocsURL`, `Owner`, `Environment`, `ServiceGlossary` |
| `internal/parser` | Extract tags, description, readable name from plan JSON |
| `internal/renderer` | Embed tour JSON, glossary, annotation data in HTML template; add all new UI components |
| `cmd/export.go` | Add `--annotations` flag |
| `cmd/serve.go` | Add `--annotations` flag; reload annotations on watch |

### Data flow

```
tf-lens export --plan plan.json --annotations tf-lens.yaml

  parser      → parse plan JSON → graph.Nodes (tags + descriptions auto-extracted)
  annotations → merge tf-lens.yaml → override label/description/docs/owner per node
               → attach tour steps to graph.Graph
  glossary    → attach ServiceInfo to each node by resource type
  renderer    → embed graph JSON + tour steps + glossary into HTML template

tf-lens serve --plan plan.json --annotations tf-lens.yaml --watch
  → same pipeline; annotations + graph reloaded on file change via SSE
```

### Frontend changes (HTML template in `internal/renderer/`)

| Feature | DOM / JS change |
|---|---|
| Summary dashboard | `<div id="dashboard">` above canvas; populated from graph JSON on load |
| Minimap | `<div id="minimap">` + Cytoscape viewport sync event handlers |
| Better search | Extend existing search handler; add filter chip UI below search bar |
| Collapsible modules | Double-click handler on compound nodes; URL hash sync |
| Dark mode | CSS custom properties + `localStorage` toggle handler |
| Guided tour | Tour JSON in `<script>`; tour overlay component; URL hash router |
| Context detail panel | Extend existing panel: label, description, docs link, owner, glossary one-liner |

### New JS dependency

`cytoscape-expand-collapse` (~15KB minified) — required for collapsible module groups. Add to `Makefile` bundle step alongside existing Cytoscape plugins.

---

## Success Criteria

- New joiner opens exported HTML and can answer "what does this queue do?" without leaving the diagram
- Teams can author a `tf-lens.yaml` in under 10 minutes and cover their critical resources
- Guided tour walks a new joiner through infra entry-to-data-layer in < 5 minutes
- Minimap makes large diagrams (50+ nodes) navigable without losing spatial context
- All features work fully offline in exported HTML
- No regression to existing overlays (threat, cost, drift, flow, diff)

---

## Phasing

### Phase 1 (this spec)
- Auto-inference from tags/descriptions
- `tf-lens.yaml` annotation file + `--annotations` flag
- AWS service glossary
- Guided tour mode
- Summary dashboard
- Minimap
- Better search with filter syntax
- Collapsible module groups
- Dark mode

### Phase 2 (future specs)
- PR comment bot (post diff diagram + threat summary as GitHub PR comment)
- SARIF export for GitHub Advanced Security
- Compliance framework mapping (CIS/SOC2/PCI-DSS)
- External wiki integration (Confluence/Notion API)
- Azure / GCP support
