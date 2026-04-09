package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	servePort  int
	servePlan  string
	serveState string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a local HTTP server with an interactive diagram (Post-MVP)",
	Long: `Starts a local HTTP server and opens the diagram in your default browser.
Uses React + React Flow for a richer interactive experience.

Note: This feature is planned for Post-MVP Phase 1.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO(post-mvp): implement embedded HTTP server with React + React Flow
		fmt.Println("🚧  serve mode is coming in Post-MVP Phase 1.")
		fmt.Println("    Use 'tf-lens export' for a fully offline diagram today.")
		return nil
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 7777, "Port for the local HTTP server")
	serveCmd.Flags().StringVar(&servePlan, "plan", "", "Path to terraform show -json output")
	serveCmd.Flags().StringVar(&serveState, "state", "", "Path to terraform.tfstate file")
}
