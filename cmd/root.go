// Package cmd wires up the gp CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "gp",
	Short:         "gp is a lightweight CLI API client",
	Long:          "gp sends HTTP requests defined in a template file, optionally driving them from a CSV data file (one request per row) — the CLI equivalent of Postman's Collection Runner.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and exits the process with an error code on failure.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
}
