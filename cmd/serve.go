package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/hack-monk/tf-lens/internal/cost"
	"github.com/hack-monk/tf-lens/internal/diff"
	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
	"github.com/hack-monk/tf-lens/internal/server"
	"github.com/hack-monk/tf-lens/internal/threat"
	"github.com/spf13/cobra"
)

var (
	servePort  int
	servePlan  string
	serveState string
	serveDiff  string
	serveNoOpen  bool
	serveThreat  bool
	serveCost    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a local HTTP server with an interactive diagram",
	Long: `Starts a local HTTP server and opens the diagram in your default browser.
The diagram is loaded dynamically from the server — click Refresh to reload
after changing your Terraform files.

Examples:
  tf-lens serve --plan plan.json
  tf-lens serve --state terraform.tfstate --port 8080
  tf-lens serve --plan new.json --diff old.json
  tf-lens serve --plan plan.json --no-open`,

	RunE: func(cmd *cobra.Command, args []string) error {
		// ── 1. Parse input ────────────────────────────────────────────────────
		resources, err := parseInput(servePlan, serveState)
		if err != nil {
			return fmt.Errorf("parsing input: %w", err)
		}
		fmt.Printf("📋  Parsed %d resources\n", len(resources))

		// ── 2. Build graph ────────────────────────────────────────────────────
		g := graph.Build(resources)

		// ── 3. Diff mode (optional) ───────────────────────────────────────────
		if serveDiff != "" {
			baseResources, err := parseInputAuto(serveDiff)
			if err != nil {
				return fmt.Errorf("parsing diff input: %w", err)
			}
			baseGraph := graph.Build(baseResources)
			diff.Annotate(g, baseGraph)

			summary := diff.Summary(g)
			fmt.Printf("📊  Diff: %d added, %d removed, %d changed\n",
				summary[graph.ChangeAdded],
				summary[graph.ChangeRemoved],
				summary[graph.ChangeUpdated],
			)
		}

		// ── 4. Threat modelling (optional) ──────────────────────────────────
		if serveThreat {
			findings := threat.Analyse(resources)
			threat.AnnotateGraph(g, findings)
			counts := map[string]int{}
			for _, f := range findings { counts[string(f.Severity)]++ }
			fmt.Printf("🔒  Threat model: %d critical, %d high, %d medium, %d info\n",
				counts["critical"], counts["high"], counts["medium"], counts["info"])
		}

		// ── 5. Cost overlay (optional) ──────────────────────────────────────
		if serveCost != "" {
			costs, err := resolveCosts(serveCost)
			if err != nil {
				return fmt.Errorf("cost overlay: %w", err)
			}
			cost.AnnotateGraph(g, costs)

			total := cost.TotalMonthlyCost(costs)
			fmt.Printf("💰  Cost: %s/mo across %d resources\n", cost.FormatCost(total), len(costs))
		}

		// ── 6. Start server ───────────────────────────────────────────────────
		resolver := icons.NewResolver("") // icons not needed in serve mode
		srv := server.New(servePort, g, resolver)

		// Open browser after a short delay so the server is ready
		if !serveNoOpen {
			url := fmt.Sprintf("http://localhost:%d", servePort)
			go func() {
				time.Sleep(200 * time.Millisecond)
				openBrowser(url)
			}()
		}

		return srv.Serve()
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 7777, "Port for the local HTTP server")
	serveCmd.Flags().StringVar(&servePlan, "plan", "", "Path to terraform show -json plan output")
	serveCmd.Flags().StringVar(&serveState, "state", "", "Path to terraform.tfstate file")
	serveCmd.Flags().StringVar(&serveDiff, "diff", "", "Base plan/state to diff against (enables diff mode)")
	serveCmd.Flags().BoolVar(&serveNoOpen, "no-open", false, "Don't open the browser automatically")
	serveCmd.Flags().BoolVar(&serveThreat, "threat", false, "Run threat modelling analysis and overlay findings")
	serveCmd.Flags().StringVar(&serveCost, "cost", "",
		"Cost overlay: path to Infracost JSON file, or Terraform directory to auto-run infracost")
}

// openBrowser opens the given URL in the default browser.
// Works on macOS, Linux, and Windows.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		fmt.Printf("→ Open your browser at: %s\n", url)
		return
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("→ Open your browser at: %s\n", url)
	}
}