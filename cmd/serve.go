package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/hack-monk/tf-lens/internal/cost"
	"github.com/hack-monk/tf-lens/internal/diff"
	"github.com/hack-monk/tf-lens/internal/drift"
	"github.com/hack-monk/tf-lens/internal/flow"
	"github.com/hack-monk/tf-lens/internal/graph"
	"github.com/hack-monk/tf-lens/internal/icons"
	"github.com/hack-monk/tf-lens/internal/server"
	"github.com/hack-monk/tf-lens/internal/threat"
	"github.com/spf13/cobra"
)

var (
	servePort      int
	servePlan      string
	serveState     string
	serveDiff      string
	serveNoOpen    bool
	serveThreat    bool
	serveCost      string
	serveDriftPath string
	serveWatch     bool
	serveFlow      bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a local HTTP server with an interactive diagram",
	Long: `Starts a local HTTP server and opens the diagram in your default browser.
The diagram is loaded dynamically from the server — click Refresh to reload
after changing your Terraform files.

Use --watch to automatically reload the diagram when input files change.

Examples:
  tf-lens serve --plan plan.json
  tf-lens serve --plan plan.json --watch
  tf-lens serve --state terraform.tfstate --port 8080
  tf-lens serve --plan new.json --diff old.json
  tf-lens serve --plan plan.json --no-open`,

	RunE: func(cmd *cobra.Command, args []string) error {
		// ── 1. Build initial graph ────────────────────────────────────────────
		g, err := buildServeGraph()
		if err != nil {
			return err
		}

		// ── 2. Start server ───────────────────────────────────────────────────
		resolver := icons.NewResolver("") // icons not needed in serve mode
		srv := server.New(servePort, g, resolver)

		// ── 3. Watch mode (optional) ──────────────────────────────────────────
		if serveWatch {
			watchPaths := collectWatchPaths()
			if len(watchPaths) == 0 {
				return fmt.Errorf("--watch requires --plan or --state to specify files to watch")
			}
			go watchFiles(srv, watchPaths)
		}

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
	serveCmd.Flags().StringVar(&serveDriftPath, "drift", "",
		"Drift detection: refresh-only plan JSON, or Terraform dir to auto-run terraform plan -refresh-only")
	serveCmd.Flags().BoolVar(&serveWatch, "watch", false,
		"Watch input files for changes and auto-reload the diagram")
	serveCmd.Flags().BoolVar(&serveFlow, "flow", false,
		"Infer and overlay runtime traffic/data flow paths")
}

// buildServeGraph runs the full pipeline: parse → build → diff → threat → cost → drift.
func buildServeGraph() (*graph.Graph, error) {
	resources, err := parseInput(servePlan, serveState)
	if err != nil {
		return nil, fmt.Errorf("parsing input: %w", err)
	}
	fmt.Printf("📋  Parsed %d resources\n", len(resources))

	g := graph.Build(resources)

	if serveDiff != "" {
		baseResources, err := parseInputAuto(serveDiff)
		if err != nil {
			return nil, fmt.Errorf("parsing diff input: %w", err)
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

	if serveThreat {
		findings := threat.Analyse(resources)
		threat.AnnotateGraph(g, findings)
		counts := map[string]int{}
		for _, f := range findings {
			counts[string(f.Severity)]++
		}
		fmt.Printf("🔒  Threat model: %d critical, %d high, %d medium, %d info\n",
			counts["critical"], counts["high"], counts["medium"], counts["info"])
	}

	if serveCost != "" {
		costs, err := resolveCosts(serveCost)
		if err != nil {
			return nil, fmt.Errorf("cost overlay: %w", err)
		}
		cost.AnnotateGraph(g, costs)

		total := cost.TotalMonthlyCost(costs)
		fmt.Printf("💰  Cost: %s/mo across %d resources\n", cost.FormatCost(total), len(costs))
	}

	if serveDriftPath != "" {
		drifted, err := resolveDrift(serveDriftPath)
		if err != nil {
			return nil, fmt.Errorf("drift detection: %w", err)
		}
		drift.AnnotateGraph(g, drifted)
		fmt.Printf("🔀  Drift: %d resources drifted from state\n", len(drifted))
	}

	if serveFlow {
		flows := flow.Infer(g)
		flow.AnnotateGraph(g, flows)
		fmt.Printf("🔀  Flow: %d traffic paths inferred\n", len(flows))
	}

	return g, nil
}

// collectWatchPaths returns all input file paths that should be watched.
func collectWatchPaths() []string {
	var paths []string
	if servePlan != "" {
		paths = append(paths, servePlan)
	}
	if serveState != "" {
		paths = append(paths, serveState)
	}
	if serveDiff != "" {
		paths = append(paths, serveDiff)
	}
	return paths
}

// watchFiles polls the given file paths for mtime changes and triggers
// a full graph rebuild + server reload when any file changes.
func watchFiles(srv *server.Server, paths []string) {
	// Record initial modification times
	mtimes := make(map[string]time.Time)
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil {
			mtimes[p] = info.ModTime()
		}
	}

	fmt.Printf("👁  Watching %d file(s) for changes\n", len(paths))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		changed := false
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			if !info.ModTime().Equal(mtimes[p]) {
				mtimes[p] = info.ModTime()
				changed = true
			}
		}
		if !changed {
			continue
		}

		fmt.Printf("\n🔄  File change detected — rebuilding graph…\n")
		g, err := buildServeGraph()
		if err != nil {
			log.Printf("⚠  Rebuild failed: %v (keeping previous graph)\n", err)
			continue
		}
		srv.Reload(g)
		fmt.Printf("✅  Graph reloaded — browser will refresh automatically\n")
	}
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