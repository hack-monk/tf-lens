package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tf-lens",
	Short: "Terraform infrastructure visualisation — CLI-first, open source",
	Long: `TF-Lens parses Terraform plan and state files and renders them
as interactive infrastructure diagrams.

Single statically-linked binary. No runtime dependencies. No cloud account required.`,
}

// Execute is the entrypoint called from main.go
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
}
