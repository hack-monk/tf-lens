package renderer

import (
	"embed"
	"io/fs"
)

// bundleFS holds the minified JS libraries downloaded by `make bundle`.
//
// The js/ directory is committed with a .gitkeep placeholder — it is
// intentionally empty in source control. The actual .js files are
// downloaded by running `make bundle` before building:
//
//	make bundle   # downloads cytoscape, dagre, cytoscape-dagre, cytoscape-node-html-label
//	make build    # compiles binary with JS baked in via go:embed
//
// When the .js files are absent (fresh clone, bundle not yet run),
// loadBundledJS() returns ("", false) and the HTML template falls back
// to CDN <script src> tags. The diagram works online but not offline.
//
// Run `make all` to do both steps in one command.

//go:embed js
var jsDir embed.FS

// bundleFS exposes the js/ subdirectory as the root of the FS,
// so callers use ReadFile("cytoscape.min.js") not ReadFile("js/cytoscape.min.js").
var bundleFS, _ = fs.Sub(jsDir, "js")