package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/hack-monk/tf-lens/internal/diff"
	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
	"github.com/hack-monk/tf-lens/internal/server"
	"github.com/spf13/cobra"
)

var (
	servePort  int
	servePlan  string
	serveState string
	serveDiff  string
	serveNoOpen bool
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

		// ── 4. Start server ───────────────────────────────────────────────────
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