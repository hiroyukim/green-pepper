package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"green-pepper/internal/model"
	"green-pepper/internal/report"
	"green-pepper/internal/runner"
)

var (
	dataFile    string
	envFile     string
	envName     string
	timeout     time.Duration
	format      string
	iterations  int
	delay       time.Duration
	stopOnError bool
)

var runCmd = &cobra.Command{
	Use:   "run <request-file>",
	Short: "Execute a request template, once per row of a CSV data file",
	Args:  cobra.ExactArgs(1),
	RunE:  runE,
}

func init() {
	runCmd.Flags().StringVar(&dataFile, "data", "", "CSV file supplying one variable set per row")
	runCmd.Flags().StringVar(&envFile, "env", "", "YAML file of default template variables (e.g. base_url), or a directory of named environment YAML files")
	runCmd.Flags().StringVar(&envName, "env-name", "", "when --env is a directory, the named environment to use (required if the directory has more than one)")
	runCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "per-request timeout")
	runCmd.Flags().StringVar(&format, "format", "table", "output format: table or json")
	runCmd.Flags().IntVar(&iterations, "iterations", 1, "number of times to repeat the run (with --data, repeats the whole CSV this many times, cycling through its rows)")
	runCmd.Flags().DurationVar(&delay, "delay", 0, "delay before each request after the first one (e.g. 500ms)")
	runCmd.Flags().BoolVar(&stopOnError, "stop-on-error", false, "stop at the first failed request instead of continuing through all requests")
}

func runE(_ *cobra.Command, args []string) error {
	if format != "table" && format != "json" {
		return fmt.Errorf("invalid --format %q: must be \"table\" or \"json\"", format)
	}
	if iterations < 1 {
		return fmt.Errorf("invalid --iterations %d: must be >= 1", iterations)
	}

	env, err := resolveRunEnv(envFile, envName)
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
	opts := runner.RunOptions{Iterations: iterations, Delay: delay, StopOnError: stopOnError}

	info, err := os.Stat(args[0])
	if err != nil {
		return err
	}

	if info.IsDir() {
		named, err := model.LoadAllFromCollection(args[0])
		if err != nil {
			return err
		}
		specs := make([]runner.NamedSpec, len(named))
		for i, n := range named {
			specs[i] = runner.NamedSpec{Name: n.Name, Spec: n.Spec}
		}

		results := runner.RunCollectionOpts(client, specs, env, rows, opts)

		if format == "json" {
			if err := report.PrintCollectionJSON(os.Stdout, results); err != nil {
				return err
			}
		} else {
			report.PrintCollection(os.Stdout, results, columns)
		}

		for _, r := range results {
			if !r.Result.Ok() {
				return fmt.Errorf("one or more requests failed")
			}
		}
		return nil
	}

	spec, err := model.LoadRequest(args[0])
	if err != nil {
		return err
	}

	results := runner.RunOpts(client, spec, env, rows, opts)

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

// resolveRunEnv resolves the --env flag (a single YAML file, a directory of
// named environment YAML files, or empty) plus --env-name into the single
// variable map a non-interactive run needs.
func resolveRunEnv(path, name string) (map[string]string, error) {
	if path == "" {
		if name != "" {
			return nil, fmt.Errorf("--env-name requires --env to be set")
		}
		return map[string]string{}, nil
	}

	isDir, err := model.IsEnvDir(path)
	if err != nil {
		return nil, err
	}
	if !isDir {
		if name != "" {
			return nil, fmt.Errorf("--env-name is only valid when --env is a directory (got file %q)", path)
		}
		return model.LoadEnv(path)
	}

	dir, err := model.LoadEnvDir(path)
	if err != nil {
		return nil, err
	}

	if name == "" {
		if len(dir.Names) == 1 {
			return dir.Envs[dir.Names[0]], nil
		}
		return nil, fmt.Errorf("multiple environments found in %s, specify one with --env-name: %s", path, strings.Join(dir.Names, ", "))
	}

	vars, ok := dir.Envs[name]
	if !ok {
		return nil, fmt.Errorf("environment %q not found in %s, available: %s", name, path, strings.Join(dir.Names, ", "))
	}
	return vars, nil
}
