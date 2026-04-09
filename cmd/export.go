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
	exportDiff    string // second plan/state for diff mode
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export an infrastructure diagram as a self-contained HTML file",
	Long: `Parses a Terraform plan or state file and writes a standalone HTML diagram.
The output file embeds Cytoscape.js and works fully offline — no internet, no dependencies.

Examples:
  tf-lens export --plan plan.json
  tf-lens export --state terraform.tfstate --out infra.html
  tf-lens export --plan new.json --diff old.json --out changes.html`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- 1. Parse primary input -----------------------------------------
		resources, err := parseInput(exportPlan, exportState)
		if err != nil {
			return fmt.Errorf("parsing input: %w", err)
		}

		// --- 2. Parse diff input (optional) ---------------------------------
		var diffResources []parser.Resource
		if exportDiff != "" {
			diffResources, err = parseInput(exportDiff, "")
			if err != nil {
				return fmt.Errorf("parsing diff input: %w", err)
			}
		}

		// --- 3. Build graph -------------------------------------------------
		g := graph.Build(resources)

		// --- 4. Apply diff annotations if requested -------------------------
		if len(diffResources) > 0 {
			baseGraph := graph.Build(diffResources)
			diff.Annotate(g, baseGraph)
		}

		// --- 5. Resolve icons -----------------------------------------------
		resolver := icons.NewResolver(exportIconDir)

		// --- 6. Render HTML -------------------------------------------------
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
	exportCmd.Flags().StringVar(&exportPlan, "plan", "", "Path to terraform show -json output (plan JSON)")
	exportCmd.Flags().StringVar(&exportState, "state", "", "Path to terraform.tfstate file")
	exportCmd.Flags().StringVar(&exportOut, "out", "diagram.html", "Output HTML file path")
	exportCmd.Flags().StringVar(&exportIconDir, "icon-dir", "", "Directory with custom SVG icons (optional)")
	exportCmd.Flags().StringVar(&exportDiff, "diff", "", "Second plan/state to diff against (enables diff mode)")
}

// parseInput picks plan vs state based on which flag was supplied.
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
