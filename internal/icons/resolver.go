// Package icons implements the three-step icon resolution waterfall.
package icons

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
)

// Resolver resolves SVG icons for Terraform resource types.
type Resolver struct {
	userDir string
}

// NewResolver creates a Resolver. Pass empty string if --icon-dir not set.
func NewResolver(userDir string) *Resolver {
	return &Resolver{userDir: userDir}
}

// Resolve returns the SVG bytes for the given resource type.
func (r *Resolver) Resolve(resourceType string) []byte {
	// 1. User-supplied directory
	if r.userDir != "" {
		if b := readFile(filepath.Join(r.userDir, resourceType+".svg")); b != nil {
			return b
		}
	}
	// 2. Embedded defaults (exact match)
	if b := readEmbedded(resourceType + ".svg"); b != nil {
		return b
	}
	// 3. Prefix fallback: aws_db_instance → aws_db → aws
	parts := strings.Split(resourceType, "_")
	for i := len(parts) - 1; i > 1; i-- {
		prefix := strings.Join(parts[:i], "_")
		if r.userDir != "" {
			if b := readFile(filepath.Join(r.userDir, prefix+".svg")); b != nil {
				return b
			}
		}
		if b := readEmbedded(prefix + ".svg"); b != nil {
			return b
		}
	}
	// 4. Generic fallback
	return genericSVG(resourceType)
}

// DataURI returns a proper base64 data URI using Go's standard library encoder.
// This is the only encoding format Cytoscape.js reliably accepts for SVG
// background-image on nodes.
func (r *Resolver) DataURI(resourceType string) string {
	b := r.Resolve(resourceType)
	encoded := base64.StdEncoding.EncodeToString(b)
	return "data:image/svg+xml;base64," + encoded
}

// ---- embedded icon map (populated by embed.go) ------------------------------

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

func genericSVG(resourceType string) []byte {
	label := resourceType
	if len(label) > 16 {
		label = label[:14] + "..."
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
		`<rect x="20" y="20" width="60" height="60" rx="12" fill="none" ` +
		`stroke="#8C8C8C" stroke-width="3" stroke-dasharray="8,4"/>` +
		`<text x="50" y="50" text-anchor="middle" dominant-baseline="central" ` +
		`font-family="Arial,sans-serif" font-size="12" font-weight="bold" fill="#8C8C8C">` +
		label + `</text></svg>`
	return []byte(svg)
}