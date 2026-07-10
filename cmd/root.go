// Package cmd implements the tf-lens CLI: export, serve, version.
package cmd

import (
	"fmt"
	"os"
)

// Version is set at build time via -ldflags
var Version = "dev"

const usageText = `TF-Lens parses Terraform plan and state files and renders them
as interactive infrastructure diagrams.

Usage:
  tf-lens export [flags]   Export a self-contained HTML diagram
  tf-lens serve  [flags]   Start a local HTTP server with a live diagram
  tf-lens version          Print the tf-lens version

Run 'tf-lens <command> --help' for command flags.

Examples:
  tf-lens export --plan plan.json
  tf-lens export --plan new.json --diff old.json --out changes.html
  tf-lens serve --plan plan.json --watch
`

// Execute is the entrypoint called from main.go
func Execute() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "export":
		err = runExport(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "version":
		fmt.Printf("tf-lens %s\n", Version)
	case "help", "-h", "--help":
		fmt.Print(usageText)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usageText)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
