package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"green-pepper/internal/model"
	"green-pepper/internal/report"
	"green-pepper/internal/runner"
)

var (
	dataFile string
	envFile  string
	timeout  time.Duration
	format   string
)

var runCmd = &cobra.Command{
	Use:   "run <request-file>",
	Short: "Execute a request template, once per row of a CSV data file",
	Args:  cobra.ExactArgs(1),
	RunE:  runE,
}

func init() {
	runCmd.Flags().StringVar(&dataFile, "data", "", "CSV file supplying one variable set per row")
	runCmd.Flags().StringVar(&envFile, "env", "", "YAML file of default template variables (e.g. base_url)")
	runCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "per-request timeout")
	runCmd.Flags().StringVar(&format, "format", "table", "output format: table or json")
}

func runE(_ *cobra.Command, args []string) error {
	if format != "table" && format != "json" {
		return fmt.Errorf("invalid --format %q: must be \"table\" or \"json\"", format)
	}

	spec, err := model.LoadRequest(args[0])
	if err != nil {
		return err
	}

	env, err := model.LoadEnv(envFile)
	if err != nil {
		return err
	}

	var columns []string
	var rows []map[string]string
	if dataFile != "" {
		data, err := model.LoadCSV(dataFile)
		if err != nil {
			return err
		}
		columns = data.Columns
		rows = data.Rows
	}

	client := &http.Client{Timeout: timeout}
	results := runner.Run(client, spec, env, rows)

	if format == "json" {
		if err := report.PrintJSON(os.Stdout, results); err != nil {
			return err
		}
	} else {
		report.Print(os.Stdout, results, columns)
	}

	for _, r := range results {
		if !r.Ok() {
			return fmt.Errorf("one or more requests failed")
		}
	}
	return nil
}
