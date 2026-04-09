package icons

import (
	"embed"
	"io/fs"
)

//go:embed svg/*.svg
var embeddedFS embed.FS

func init() {
	embeddedIcons = make(map[string][]byte)
	entries, err := fs.ReadDir(embeddedFS, "svg")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := embeddedFS.ReadFile("svg/" + e.Name())
		if err == nil {
			embeddedIcons[e.Name()] = data
		}
	}
}
