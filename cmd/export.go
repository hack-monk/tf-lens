package cmd

import (
	"fmt"
	"os"

	"github.com/hack-monk/tf-lens/internal/diff"
	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
	"github.com/hack-monk/tf-lens/internal/parser"
	"github.com/hack-monk/tf-lens/internal/renderer"
	"github.com/spf13/cobra"
)

var (
	exportPlan    string
	exportState   string
	exportOut     string
	exportIconDir string
	exportDiff    string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export an infrastructure diagram as a self-contained HTML file",
	Long: `Parses a Terraform plan or state file and writes a standalone HTML diagram.
The output file is fully self-contained and works offline with no dependencies.

Examples:
  # Basic export from a plan file
  tf-lens export --plan plan.json

  # Export from a state file with custom output path
  tf-lens export --state terraform.tfstate --out infra.html

  # Diff mode: compare new plan against old plan (shows added/removed/changed)
  tf-lens export --plan new.json --diff old.json --out changes.html

  # Diff mode: compare current plan against saved state
  tf-lens export --plan plan.json --diff terraform.tfstate --out changes.html`,

	RunE: func(cmd *cobra.Command, args []string) error {
		// ── 1. Parse primary input ────────────────────────────────────────────
		resources, err := parseInput(exportPlan, exportState)
		if err != nil {
			return fmt.Errorf("parsing input: %w", err)
		}
		fmt.Printf("📋  Parsed %d resources\n", len(resources))

		// ── 2. Build graph ────────────────────────────────────────────────────
		g := graph.Build(resources)

		// ── 3. Diff mode (optional) ───────────────────────────────────────────
		isDiff := exportDiff != ""
		if isDiff {
			baseResources, err := parseInputAuto(exportDiff)
			if err != nil {
				return fmt.Errorf("parsing diff input: %w", err)
			}
			baseGraph := graph.Build(baseResources)
			diff.Annotate(g, baseGraph)

			// Print diff summary to stdout
			summary := diff.Summary(g)
			fmt.Println()
			fmt.Println("📊  Diff summary:")
			fmt.Printf("    ✅  Added:     %d\n", summary[graph.ChangeAdded])
			fmt.Printf("    ❌  Removed:   %d\n", summary[graph.ChangeRemoved])
			fmt.Printf("    🔄  Updated:   %d\n", summary[graph.ChangeUpdated])
			fmt.Printf("    ─   Unchanged: %d\n", summary[graph.ChangeNone])
			fmt.Println()
		}

		// ── 4. Resolve icons ──────────────────────────────────────────────────
		resolver := icons.NewResolver(exportIconDir)

		// ── 5. Write HTML ─────────────────────────────────────────────────────
		outPath := exportOut
		if outPath == "" {
			outPath = "diagram.html"
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating output file %q: %w", outPath, err)
		}
		defer f.Close()

		if err := renderer.ExportHTML(f, g, resolver); err != nil {
			return fmt.Errorf("rendering diagram: %w", err)
		}

		fmt.Printf("✅  Diagram written to %s\n", outPath)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportPlan, "plan", "", "Path to terraform show -json plan output")
	exportCmd.Flags().StringVar(&exportState, "state", "", "Path to terraform.tfstate file")
	exportCmd.Flags().StringVar(&exportOut, "out", "diagram.html", "Output HTML file path")
	exportCmd.Flags().StringVar(&exportIconDir, "icon-dir", "", "Directory with custom SVG icons (optional)")
	exportCmd.Flags().StringVar(&exportDiff, "diff", "",
		"Base plan/state to diff against — auto-detects format (enables diff mode)")
}

// parseInput selects plan vs state based on which flag was provided.
func parseInput(planPath, statePath string) ([]parser.Resource, error) {
	switch {
	case planPath != "":
		return parser.ParsePlanFile(planPath)
	case statePath != "":
		return parser.ParseStateFile(statePath)
	default:
		return nil, fmt.Errorf("provide either --plan or --state")
	}
}

// parseInputAuto auto-detects whether a file is a plan or state file by
// inspecting its JSON structure, so --diff works with both formats.
func parseInputAuto(path string) ([]parser.Resource, error) {
	// Try plan first (has "planned_values" key), fall back to state
	if resources, err := parser.ParsePlanFile(path); err == nil {
		return resources, nil
	}
	return parser.ParseStateFile(path)
}