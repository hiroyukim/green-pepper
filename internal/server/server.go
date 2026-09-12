// Package server provides a local web UI for building an HTTP request,
// sending it once, or running it against a CSV data file.
package server

import (
	"embed"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"green-pepper/internal/model"
	"green-pepper/internal/runner"
)

//go:embed templates/*.html
var templateFS embed.FS

const (
	maxUploadSize      = 10 << 20 // 10 MiB
	maxSendPreviewBody = 1 << 20  // 1 MiB of response body shown in the UI
)

var varPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Server serves the request-builder UI: an editable request/env form that
// can send a single request, run it against an uploaded CSV, or download
// the current request as YAML.
type Server struct {
	RequestPath string
	EnvPath     string
	Timeout     time.Duration

	mu   sync.Mutex
	spec model.RequestSpec
	env  map[string]string

	// Named environments loaded from a directory passed to --env (issue
	// #10). envDir/envFiles are empty when --env was a single file (or
	// absent), in which case the environment dropdown/save button are not
	// shown at all.
	envDir        string                       // directory environments were loaded from
	envFiles      map[string]string            // name -> on-disk file name (with extension)
	envs          map[string]map[string]string // name -> vars, kept in sync with saves
	envNames      []string                     // sorted, for stable dropdown order
	activeEnvName string

	// Each page gets its own template set (layout.html + that page's
	// content) so the "content" block each defines doesn't clash with
	// the other page's block of the same name.
	indexTmpl   *template.Template
	resultsTmpl *template.Template
}

// New builds a Server, parsing the embedded HTML templates. spec and env are
// the initial values loaded from disk; the UI edits an in-memory copy from
// there on, it never writes back to requestPath/envPath. envDir carries the
// full set of named environments when --env resolved to a directory (nil
// when it was a single file or absent).
func New(spec *model.RequestSpec, env map[string]string, requestPath, envPath string, envDir *model.EnvDir, timeout time.Duration) (*Server, error) {
	indexTmpl, err := template.ParseFS(templateFS, "templates/layout.html", "templates/index.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	resultsTmpl, err := template.ParseFS(templateFS, "templates/layout.html", "templates/results.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	s := &Server{
		RequestPath: requestPath,
		EnvPath:     envPath,
		Timeout:     timeout,
		spec:        *spec,
		env:         maps.Clone(env),
		indexTmpl:   indexTmpl,
		resultsTmpl: resultsTmpl,
	}
	if envDir != nil {
		s.envDir = envDir.Dir
		s.envFiles = maps.Clone(envDir.Files)
		s.envs = make(map[string]map[string]string, len(envDir.Envs))
		for name, vars := range envDir.Envs {
			s.envs[name] = maps.Clone(vars)
		}
		s.envNames = append([]string(nil), envDir.Names...)
		if len(s.envNames) > 0 {
			s.activeEnvName = s.envNames[0]
		}
	}
	return s, nil
}

// Handler returns the http.Handler serving the UI.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /execute", s.handleExecute)
	mux.HandleFunc("GET /environment", s.handleSwitchEnv)
	return mux
}

type pageData struct {
	RequestPath string
	EnvPath     string
	Error       string
	Info        string

	Method      string
	URL         string
	HeadersText string
	Body        string
	EnvText     string
	UsedVarsCSV string

	// EnvNames lists the available named environments (--env was a
	// directory); empty when --env was a single file or absent, in which
	// case the template hides the environment dropdown/save button.
	EnvNames      []string
	ActiveEnvName string

	SendResult *sendResultView
}

type sendResultView struct {
	StatusCode int
	Status     string
	OK         bool
	Duration   string
	Bytes      int64
	Headers    []headerView
	Body       string
	Err        string
}

type headerView struct {
	Name  string
	Value string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	data := s.pageDataLocked("")
	s.mu.Unlock()
	s.render(w, s.indexTmpl, data)
}

// pageDataLocked builds pageData from the current in-memory spec/env. Callers
// must hold s.mu.
func (s *Server) pageDataLocked(errMsg string) pageData {
	return pageData{
		RequestPath:   s.RequestPath,
		EnvPath:       s.EnvPath,
		Error:         errMsg,
		Method:        s.spec.Method,
		URL:           s.spec.URL,
		HeadersText:   mapToLines(s.spec.Headers, ": "),
		Body:          s.spec.Body,
		EnvText:       mapToLines(s.env, "="),
		UsedVarsCSV:   strings.Join(usedVars(s.spec), ","),
		EnvNames:      append([]string(nil), s.envNames...),
		ActiveEnvName: s.activeEnvName,
	}
}

// handleExecute applies the edited request/env fields from the form, then
// dispatches on the pressed button ("send", "run" or "download").
func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		s.mu.Lock()
		data := s.pageDataLocked("フォームの読み取りに失敗しました: " + err.Error())
		s.mu.Unlock()
		s.render(w, s.indexTmpl, data)
		return
	}

	method := strings.TrimSpace(r.FormValue("method"))
	if method == "" {
		method = "GET"
	}

	s.mu.Lock()
	s.spec = model.RequestSpec{
		Method:  method,
		URL:     strings.TrimSpace(r.FormValue("url")),
		Headers: linesToMap(r.FormValue("headers"), ":"),
		Body:    r.FormValue("body"),
	}
	s.env = linesToMap(r.FormValue("env"), "=")
	spec := s.spec
	env := maps.Clone(s.env)
	s.mu.Unlock()

	switch r.FormValue("action") {
	case "download":
		s.handleDownload(w, spec)
	case "run":
		s.handleRunCSV(w, r, spec, env)
	case "save-env":
		s.handleSaveEnv(w, env)
	default:
		s.handleSend(w, spec, env)
	}
}

// handleSwitchEnv handles GET /environment?name=<name>: it loads the named
// environment's variables into the current in-memory env (replacing
// whatever is in the #env textarea, exactly like picking a different saved
// request would replace the request spec) and redirects back to "/".
func (s *Server) handleSwitchEnv(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	s.mu.Lock()
	vars, ok := s.envs[name]
	if ok {
		s.activeEnvName = name
		s.env = maps.Clone(vars)
	}
	s.mu.Unlock()

	if !ok {
		http.Error(w, fmt.Sprintf("unknown environment %q", name), http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleSaveEnv writes the current #env textarea contents back to the
// currently active named environment's file on disk. Only meaningful when
// --env resolved to a directory (s.activeEnvName is set); otherwise it's an
// error, though the UI only shows the "この環境を保存" button in that case.
func (s *Server) handleSaveEnv(w http.ResponseWriter, env map[string]string) {
	s.mu.Lock()
	name := s.activeEnvName
	filename, ok := s.envFiles[name]
	dir := s.envDir
	s.mu.Unlock()

	var errMsg, info string
	switch {
	case !ok || name == "":
		errMsg = "保存対象の環境が選択されていません"
	default:
		if err := model.SaveEnv(dir, filename, env); err != nil {
			errMsg = "環境の保存に失敗しました: " + err.Error()
		} else {
			s.mu.Lock()
			s.envs[name] = maps.Clone(env)
			s.mu.Unlock()
			info = fmt.Sprintf("環境 %q を保存しました", name)
		}
	}

	s.mu.Lock()
	data := s.pageDataLocked(errMsg)
	s.mu.Unlock()
	data.Info = info
	s.render(w, s.indexTmpl, data)
}

func (s *Server) handleSend(w http.ResponseWriter, spec model.RequestSpec, env map[string]string) {
	s.mu.Lock()
	data := s.pageDataLocked("")
	s.mu.Unlock()

	if spec.URL == "" {
		data.Error = "URLを入力してください"
		s.render(w, s.indexTmpl, data)
		return
	}

	client := &http.Client{Timeout: s.Timeout}
	result := runner.Send(client, &spec, env, maxSendPreviewBody)

	sr := &sendResultView{
		StatusCode: result.StatusCode,
		Status:     result.Status,
		OK:         result.Ok(),
		Duration:   result.Duration.Round(time.Millisecond).String(),
		Bytes:      int64(len(result.Body)),
		Body:       string(result.Body),
	}
	if result.Err != nil {
		sr.Err = result.Err.Error()
	}
	names := make([]string, 0, len(result.Headers))
	for name := range result.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		sr.Headers = append(sr.Headers, headerView{Name: name, Value: strings.Join(result.Headers[name], ", ")})
	}

	data.SendResult = sr
	s.render(w, s.indexTmpl, data)
}

func (s *Server) handleRunCSV(w http.ResponseWriter, r *http.Request, spec model.RequestSpec, env map[string]string) {
	renderErr := func(msg string) {
		s.mu.Lock()
		data := s.pageDataLocked(msg)
		s.mu.Unlock()
		s.render(w, s.indexTmpl, data)
	}

	file, _, err := r.FormFile("csv")
	if err != nil {
		renderErr("CSVファイルを選択してください")
		return
	}
	defer file.Close()

	data, err := model.ParseCSV(file)
	if err != nil {
		renderErr("CSVの解析に失敗しました: " + err.Error())
		return
	}

	client := &http.Client{Timeout: s.Timeout}
	results := runner.Run(client, &spec, env, data.Rows)

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

func (s *Server) handleDownload(w http.ResponseWriter, spec model.RequestSpec) {
	out, err := yaml.Marshal(spec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="request.yaml"`)
	w.Write(out)
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

// mapToLines renders m as "key<sep>value" lines, sorted by key, for display
// in a textarea.
func mapToLines(m map[string]string, sep string) string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, name+sep+m[name])
	}
	return strings.Join(lines, "\n")
}

// linesToMap parses "key<sep>value" lines (blank lines and lines starting
// with "#" are ignored) back into a map.
func linesToMap(text, sep string) map[string]string {
	m := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, sep)
		if !ok {
			continue
		}
		m[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	return m
}

// usedVars returns the sorted, de-duplicated list of "{{var}}" names
// referenced anywhere in spec.
func usedVars(spec model.RequestSpec) []string {
	seen := map[string]bool{}
	var names []string
	add := func(s string) {
		for _, m := range varPattern.FindAllStringSubmatch(s, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				names = append(names, m[1])
			}
		}
	}
	add(spec.Method)
	add(spec.URL)
	add(spec.Body)
	for _, v := range spec.Headers {
		add(v)
	}
	sort.Strings(names)
	return names
}
