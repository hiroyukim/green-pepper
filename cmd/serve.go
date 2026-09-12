package cmd

import (
	"fmt"
	"net/http"
	"os"
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
	Use:   "serve [request-file|collection-dir]",
	Short: "Serve a local web UI for building/sending a request and running it against a CSV data file",
	Long: "Serve a local web UI for building/sending a request and running it against a CSV data file.\n\n" +
		"The argument may be a single request YAML file (today's behavior), a\n" +
		"directory of request YAML files (a flat collection — enables the\n" +
		"collection sidebar and \"save as\" in the UI), or omitted entirely to\n" +
		"start from a blank request.",
	Args: cobra.MaximumNArgs(1),
	RunE: serveE,
}

func init() {
	serveCmd.Flags().StringVar(&serveEnvFile, "env", "", "YAML file of default template variables (e.g. base_url), or a directory of named environment YAML files")
	serveCmd.Flags().DurationVar(&serveTimeout, "timeout", 30*time.Second, "per-request timeout")
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "port to listen on")
}

func serveE(_ *cobra.Command, args []string) error {
	var requestPath string
	var collectionDir string
	spec := &model.RequestSpec{Method: "GET"}

	if len(args) == 1 {
		info, err := os.Stat(args[0])
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Collection directory: start from a blank request; the user
			// picks one from the sidebar to load it (GET /collection/{name}).
			collectionDir = args[0]
		} else {
			requestPath = args[0]
			spec, err = model.LoadRequest(requestPath)
			if err != nil {
				return err
			}
		}
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
	srv.CollectionDir = collectionDir

	addr := fmt.Sprintf(":%d", servePort)
	fmt.Printf("gp serve listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, srv.Handler())
}
