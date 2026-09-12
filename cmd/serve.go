package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"green-pepper/internal/model"
	"green-pepper/internal/server"
)

var (
	serveEnvFile string
	serveTimeout time.Duration
	servePort    int
)

var serveCmd = &cobra.Command{
	Use:   "serve [request-file]",
	Short: "Serve a local web UI for building/sending a request and running it against a CSV data file",
	Args:  cobra.MaximumNArgs(1),
	RunE:  serveE,
}

func init() {
	serveCmd.Flags().StringVar(&serveEnvFile, "env", "", "YAML file of default template variables (e.g. base_url), or a directory of named environment YAML files")
	serveCmd.Flags().DurationVar(&serveTimeout, "timeout", 30*time.Second, "per-request timeout")
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "port to listen on")
}

func serveE(_ *cobra.Command, args []string) error {
	var requestPath string
	var spec *model.RequestSpec

	if len(args) == 1 {
		requestPath = args[0]

		var err error
		spec, err = model.LoadRequest(requestPath)
		if err != nil {
			return err
		}
	} else {
		spec = &model.RequestSpec{Method: "GET"}
	}

	isDir, err := model.IsEnvDir(serveEnvFile)
	if err != nil {
		return err
	}

	var env map[string]string
	var envDir *model.EnvDir
	if isDir {
		envDir, err = model.LoadEnvDir(serveEnvFile)
		if err != nil {
			return err
		}
		env = envDir.Envs[envDir.Names[0]]
	} else {
		env, err = model.LoadEnv(serveEnvFile)
		if err != nil {
			return err
		}
	}

	srv, err := server.New(spec, env, requestPath, serveEnvFile, envDir, serveTimeout)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf(":%d", servePort)
	fmt.Printf("gp serve listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, srv.Handler())
}
