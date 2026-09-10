// Package server provides a local web UI for uploading a CSV data file and
// running it against a fixed request template.
package server

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"green-pepper/internal/model"
	"green-pepper/internal/runner"
)

//go:embed templates/*.html
var templateFS embed.FS

const maxUploadSize = 10 << 20 // 10 MiB

// Server serves the CSV upload UI for a fixed request template and env.
type Server struct {
	Spec        *model.RequestSpec
	Env         map[string]string
	RequestPath string
	EnvPath     string
	Timeout     time.Duration

	// Each page gets its own template set (layout.html + that page's
	// content) so the "content" block each defines doesn't clash with
	// the other page's block of the same name.
	indexTmpl   *template.Template
	resultsTmpl *template.Template
}

// New builds a Server, parsing the embedded HTML templates.
func New(spec *model.RequestSpec, env map[string]string, requestPath, envPath string, timeout time.Duration) (*Server, error) {
	indexTmpl, err := template.ParseFS(templateFS, "templates/layout.html", "templates/index.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	resultsTmpl, err := template.ParseFS(templateFS, "templates/layout.html", "templates/results.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return &Server{
		Spec:        spec,
		Env:         env,
		RequestPath: requestPath,
		EnvPath:     envPath,
		Timeout:     timeout,
		indexTmpl:   indexTmpl,
		resultsTmpl: resultsTmpl,
	}, nil
}

// Handler returns the http.Handler serving the UI.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /run", s.handleRun)
	return mux
}

type pageData struct {
	RequestPath string
	EnvPath     string
	Error       string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, s.indexTmpl, pageData{RequestPath: s.RequestPath, EnvPath: s.EnvPath})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		s.render(w, s.indexTmpl, pageData{RequestPath: s.RequestPath, EnvPath: s.EnvPath, Error: "アップロードの読み取りに失敗しました: " + err.Error()})
		return
	}

	file, _, err := r.FormFile("csv")
	if err != nil {
		s.render(w, s.indexTmpl, pageData{RequestPath: s.RequestPath, EnvPath: s.EnvPath, Error: "CSVファイルを選択してください"})
		return
	}
	defer file.Close()

	data, err := model.ParseCSV(file)
	if err != nil {
		s.render(w, s.indexTmpl, pageData{RequestPath: s.RequestPath, EnvPath: s.EnvPath, Error: "CSVの解析に失敗しました: " + err.Error()})
		return
	}

	client := &http.Client{Timeout: s.Timeout}
	results := runner.Run(client, s.Spec, s.Env, data.Rows)

	view := resultsView{RequestPath: s.RequestPath, EnvPath: s.EnvPath, Columns: data.Columns, Total: len(results)}
	for i, res := range results {
		if res.Ok() {
			view.Passed++
		}
		status := res.Status
		if status == "" {
			status = "-"
		}
		errMsg := ""
		if res.Err != nil {
			errMsg = res.Err.Error()
		}
		values := make([]string, len(data.Columns))
		for j, col := range data.Columns {
			values[j] = res.Row[col]
		}
		view.Rows = append(view.Rows, rowView{
			Index:    i + 1,
			Status:   status,
			OK:       res.Ok(),
			Duration: res.Duration.Round(time.Millisecond).String(),
			Bytes:    res.Bytes,
			Values:   values,
			Err:      errMsg,
		})
	}

	s.render(w, s.resultsTmpl, view)
}

type resultsView struct {
	RequestPath string
	EnvPath     string
	Columns     []string
	Rows        []rowView
	Passed      int
	Total       int
}

type rowView struct {
	Index    int
	Status   string
	OK       bool
	Duration string
	Bytes    int64
	Values   []string
	Err      string
}

func (s *Server) render(w http.ResponseWriter, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
