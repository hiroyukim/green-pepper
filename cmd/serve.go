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
	Use:   "serve <request-file>",
	Short: "Serve a local web UI for uploading a CSV data file and running it",
	Args:  cobra.ExactArgs(1),
	RunE:  serveE,
}

func init() {
	serveCmd.Flags().StringVar(&serveEnvFile, "env", "", "YAML file of default template variables (e.g. base_url)")
	serveCmd.Flags().DurationVar(&serveTimeout, "timeout", 30*time.Second, "per-request timeout")
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "port to listen on")
}

func serveE(_ *cobra.Command, args []string) error {
	requestPath := args[0]

	spec, err := model.LoadRequest(requestPath)
	if err != nil {
		return err
	}

	env, err := model.LoadEnv(serveEnvFile)
	if err != nil {
		return err
	}

	srv, err := server.New(spec, env, requestPath, serveEnvFile, serveTimeout)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf(":%d", servePort)
	fmt.Printf("gp serve listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, srv.Handler())
}
