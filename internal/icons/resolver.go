// Package icons implements the three-step icon resolution waterfall:
//  1. User-supplied --icon-dir directory
//  2. Embedded default SVGs (go:embed)
//  3. Prefix fallback (aws_db_cluster → aws_db.svg)
//  4. Generic dashed-box SVG (never blank, never an error)
package icons

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

// Resolver resolves SVG icons for Terraform resource types.
type Resolver struct {
	userDir string
}

// NewResolver creates a Resolver. Pass an empty string if the user did not
// supply --icon-dir.
func NewResolver(userDir string) *Resolver {
	return &Resolver{userDir: userDir}
}

// Resolve returns the SVG bytes for the given Terraform resource type.
// It never returns nil — the generic fallback is always available.
func (r *Resolver) Resolve(resourceType string) []byte {
	// Step 1 — user-supplied directory
	if r.userDir != "" {
		if b := readFile(filepath.Join(r.userDir, resourceType+".svg")); b != nil {
			return b
		}
	}

	// Step 2 — embedded defaults (exact match)
	if b := readEmbedded(resourceType + ".svg"); b != nil {
		return b
	}

	// Step 3 — prefix fallback: aws_db_instance_automated_backups → aws_db_instance → aws_db
	parts := strings.Split(resourceType, "_")
	for i := len(parts) - 1; i > 1; i-- {
		prefix := strings.Join(parts[:i], "_")

		// User dir prefix fallback
		if r.userDir != "" {
			if b := readFile(filepath.Join(r.userDir, prefix+".svg")); b != nil {
				return b
			}
		}
		// Embedded prefix fallback
		if b := readEmbedded(prefix + ".svg"); b != nil {
			return b
		}
	}

	// Step 4 — generic fallback (dashed box with label)
	return genericSVG(resourceType)
}

// DataURI returns the icon as a base64-encoded data URI suitable for
// embedding directly in HTML <img src="..."> or Cytoscape node style.
func (r *Resolver) DataURI(resourceType string) string {
	b := r.Resolve(resourceType)
	return "data:image/svg+xml;base64," + encodeBase64(b)
}

// ---- embedded defaults -------------------------------------------------------

// embeddedIcons is populated by go:generate / go:embed in icons_embed.go.
// We keep the embed directive in a separate file so this file stays testable
// without the full icons/ directory present.
var embeddedIcons map[string][]byte

func readEmbedded(name string) []byte {
	if embeddedIcons == nil {
		return nil
	}
	return embeddedIcons[name]
}

// ---- helpers ----------------------------------------------------------------

func readFile(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return b
}

// genericSVG returns a dashed rectangle with the resource type as a label.
// This is the final fallback — it is always valid SVG.
func genericSVG(resourceType string) []byte {
	// Truncate very long resource type names for readability
	label := resourceType
	if len(label) > 24 {
		label = label[:22] + "…"
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
  <rect x="2" y="2" width="60" height="60" rx="6"
    fill="none" stroke="#5F5E5A" stroke-width="2" stroke-dasharray="6,3"/>
  <text x="32" y="36" font-size="7" font-family="monospace"
    fill="#5F5E5A" text-anchor="middle">` + label + `</text>
</svg>`
	return []byte(svg)
}

func encodeBase64(b []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte
	for i := 0; i < len(b); i += 3 {
		var buf [3]byte
		n := copy(buf[:], b[i:])
		result = append(result,
			chars[buf[0]>>2],
			chars[(buf[0]&0x3)<<4|buf[1]>>4],
		)
		if n > 1 {
			result = append(result, chars[(buf[1]&0xf)<<2|buf[2]>>6])
		} else {
			result = append(result, '=')
		}
		if n > 2 {
			result = append(result, chars[buf[2]&0x3f])
		} else {
			result = append(result, '=')
		}
	}
	return string(result)
}
