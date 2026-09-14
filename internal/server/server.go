// Package server provides a local web UI for building an HTTP request,
// sending it once, or running it against a CSV data file.
package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"green-pepper/internal/model"
	"green-pepper/internal/report"
	"green-pepper/internal/runner"
)

//go:embed templates/*.html
var templateFS embed.FS

const (
	maxUploadSize      = 10 << 20 // 10 MiB
	maxSendPreviewBody = 1 << 20  // 1 MiB of response body shown in the UI
	maxHistoryEntries  = 50       // oldest single-send history entries are dropped past this

	// maxCSVHistoryEntries caps CSV/collection run history (issue #44). It's
	// lower than maxHistoryEntries because each entry holds one row per CSV
	// row/collection request, so a single entry can be much larger than one
	// single-send history entry.
	maxCSVHistoryEntries = 20

	// maxCSVRuns bounds memory for a long-running gp serve session's
	// in-flight/finished async CSV/collection run tracking (issue #45).
	maxCSVRuns = 20
)

var varPattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Server serves the request-builder UI: an editable request/env form that
// can send a single request, run it against an uploaded CSV, or download
// the current request as YAML.
type Server struct {
	RequestPath string
	EnvPath     string
	Timeout     time.Duration

	// CollectionDir, when non-empty, is a directory of "*.yaml"/"*.yml"
	// request files (see internal/model/collection.go) that the UI can list,
	// load from, and save into. Empty disables the collection feature
	// entirely (no sidebar, no save-as, matching single-file/no-arg usage).
	CollectionDir string

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

	// history holds past single-send actions, newest last, capped at
	// maxHistoryEntries. nextHistoryID increments forever (never reused) so
	// that a /history/{id} link to an entry evicted by the cap fails
	// clearly instead of silently resolving to a different entry that
	// happens to reuse its old slot.
	history       []historyEntry
	nextHistoryID int

	// csvHistory holds past CSV/collection run results, newest last, capped
	// at maxCSVHistoryEntries (issue #44). Like history/nextHistoryID above,
	// nextCSVHistoryID never resets or reuses IDs, so a /csv-history/{id}
	// link to an entry evicted by the cap fails clearly instead of silently
	// resolving to a different entry. Each entry stores only a lightweight
	// csvHistoryRow snapshot per row — deliberately not the full rowView —
	// so history memory stays bounded regardless of how much data rowView
	// itself carries (see csvHistoryRow's doc comment).
	csvHistory       []csvHistoryEntry
	nextCSVHistoryID int

	// runs tracks in-flight (or just-finished) async CSV/collection runs
	// started by handleRunCSV, polled by the browser via
	// GET /run-progress/{id} (issue #45). nextRunID never resets or reuses
	// IDs, and also (since IDs are assigned in increasing order) serves as
	// the insertion-order key pruneRunsLocked uses to evict the oldest DONE
	// entries once len(runs) exceeds maxCSVRuns.
	runs      map[int]*csvRunProgress
	nextRunID int

	// Each page gets its own template set (layout.html + that page's
	// content) so the "content" block each defines doesn't clash with
	// the other page's block of the same name.
	indexTmpl    *template.Template
	resultsTmpl  *template.Template
	progressTmpl *template.Template
}

// csvRunProgress tracks one in-flight (or just-finished) async CSV/
// collection run started by handleRunCSV, polled by the browser via
// GET /run-progress/{id} (issue #45).
type csvRunProgress struct {
	Completed int
	Total     int
	Done      bool
	HistoryID int    // valid once Done; the /csv-history/{HistoryID} to redirect to
	Err       string // non-empty only if something went wrong after the goroutine started
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
	progressTmpl, err := template.ParseFS(templateFS, "templates/layout.html", "templates/progress.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	s := &Server{
		RequestPath:  requestPath,
		EnvPath:      envPath,
		Timeout:      timeout,
		spec:         *spec,
		env:          maps.Clone(env),
		indexTmpl:    indexTmpl,
		resultsTmpl:  resultsTmpl,
		progressTmpl: progressTmpl,
		runs:         make(map[int]*csvRunProgress),
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
	mux.HandleFunc("GET /history/{id}", s.handleHistoryRestore)
	mux.HandleFunc("GET /csv-history/{id}", s.handleCSVHistoryShow)
	mux.HandleFunc("GET /run-progress/{id}", s.handleRunProgress)
	mux.HandleFunc("GET /collection/{name}", s.handleLoadFromCollection)
	mux.HandleFunc("POST /api/send", s.handleAPISend)
	mux.HandleFunc("POST /api/run", s.handleAPIRun)
	return mux
}

type pageData struct {
	RequestPath string
	EnvPath     string
	Error       string
	Info        string
	Saved       string

	Method      string
	URL         string
	HeadersText string
	Body        string
	TestScript  string
	EnvText     string
	UsedVarsCSV string

	// EnvNames lists the available named environments (--env was a
	// directory); empty when --env was a single file or absent, in which
	// case the template hides the environment dropdown/save button.
	EnvNames      []string
	ActiveEnvName string

	CollectionActive bool
	CollectionNames  []string

	SendResult *sendResultView
	History    []historyRowView

	// CSVHistory lists past CSV/collection run results, newest first, for
	// the "CSV実行履歴" card (issue #44).
	CSVHistory []csvHistoryListView
}

// historyEntry is one past single-send action: what was sent (enough to
// restore the editing state) and what came back.
type historyEntry struct {
	ID       int
	Spec     model.RequestSpec
	Env      map[string]string
	OK       bool
	Status   string
	Duration string
	Bytes    int64
	Err      string
}

// historyRowView is the display-ready form of a historyEntry for the
// "履歴" card.
type historyRowView struct {
	ID       int
	Method   string
	URL      string
	OK       bool
	Status   string
	Duration string
	Bytes    int64
	Err      string
}

type sendResultView struct {
	StatusCode  int
	Status      string
	OK          bool
	Duration    string
	Bytes       int64
	Headers     []headerView
	Body        string
	Err         string
	TestResults []testResultView
	// ConsoleLogs holds any console.log/warn/error output the request's
	// test_script produced, if it has one; nil/empty otherwise, in which
	// case the response block's Console section is not rendered at all.
	ConsoleLogs []string
}

type headerView struct {
	Name  string
	Value string
}

// headerViews converts an http.Header into the display-ready, name-sorted
// form shared by the single-send response block (sendResultView.Headers) and
// the CSV/collection results table's per-row detail (rowView.Headers).
func headerViews(h http.Header) []headerView {
	if len(h) == 0 {
		return nil
	}
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)
	views := make([]headerView, 0, len(names))
	for _, name := range names {
		views = append(views, headerView{Name: name, Value: strings.Join(h[name], ", ")})
	}
	return views
}

// testResultView is the display-ready form of a runner.TestResult, for the
// single-send response block's "Tests" list.
type testResultView struct {
	Name   string
	Passed bool
	Error  string
}

func testResultViews(results []runner.TestResult) []testResultView {
	if len(results) == 0 {
		return nil
	}
	views := make([]testResultView, len(results))
	for i, t := range results {
		views[i] = testResultView{Name: t.Name, Passed: t.Passed, Error: t.Error}
	}
	return views
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	data := s.pageDataLocked(r.URL.Query().Get("error"))
	s.mu.Unlock()
	s.render(w, s.indexTmpl, data)
}

// pageDataLocked builds pageData from the current in-memory spec/env. Callers
// must hold s.mu.
func (s *Server) pageDataLocked(errMsg string) pageData {
	data := pageData{
		RequestPath:   s.RequestPath,
		EnvPath:       s.EnvPath,
		Error:         errMsg,
		Method:        s.spec.Method,
		URL:           s.spec.URL,
		HeadersText:   mapToLines(s.spec.Headers, ": "),
		Body:          s.spec.Body,
		TestScript:    s.spec.TestScript,
		EnvText:       mapToLines(s.env, "="),
		UsedVarsCSV:   strings.Join(usedVars(s.spec), ","),
		EnvNames:      append([]string(nil), s.envNames...),
		ActiveEnvName: s.activeEnvName,
		History:       s.historyViewsLocked(),
		CSVHistory:    s.csvHistoryViewsLocked(),
	}

	if s.CollectionDir != "" {
		data.CollectionActive = true
		names, err := model.ListCollection(s.CollectionDir)
		if err != nil {
			if data.Error == "" {
				data.Error = "コレクションの読み込みに失敗しました: " + err.Error()
			}
		} else {
			data.CollectionNames = names
		}
	}

	return data
}

// historyViewsLocked returns the current history, newest first. Callers must
// hold s.mu.
func (s *Server) historyViewsLocked() []historyRowView {
	if len(s.history) == 0 {
		return nil
	}
	views := make([]historyRowView, 0, len(s.history))
	for i := len(s.history) - 1; i >= 0; i-- {
		h := s.history[i]
		views = append(views, historyRowView{
			ID:       h.ID,
			Method:   h.Spec.Method,
			URL:      h.Spec.URL,
			OK:       h.OK,
			Status:   h.Status,
			Duration: h.Duration,
			Bytes:    h.Bytes,
			Err:      h.Err,
		})
	}
	return views
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
		Method:     method,
		URL:        strings.TrimSpace(r.FormValue("url")),
		Headers:    linesToMap(r.FormValue("headers"), ":"),
		Body:       r.FormValue("body"),
		TestScript: r.FormValue("test_script"),
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
	case "save_as":
		s.handleSaveAs(w, r, spec)
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

// handleLoadFromCollection loads the named request from the collection
// directory into the current in-memory spec (env is left untouched — that is
// issue #10's concern) and redirects back to the index page. A no-op 404 when
// the collection feature is disabled or the request can't be found/loaded.
func (s *Server) handleLoadFromCollection(w http.ResponseWriter, r *http.Request) {
	if s.CollectionDir == "" {
		http.NotFound(w, r)
		return
	}

	name := r.PathValue("name")
	spec, err := model.LoadFromCollection(s.CollectionDir, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.mu.Lock()
	s.spec = *spec
	s.mu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleSaveAs saves the current (just-updated-from-the-form) spec into the
// collection directory under the "name" form field, then re-renders the
// index page with a success or error message.
func (s *Server) handleSaveAs(w http.ResponseWriter, r *http.Request, spec model.RequestSpec) {
	name := strings.TrimSpace(r.FormValue("name"))

	if s.CollectionDir == "" {
		s.mu.Lock()
		data := s.pageDataLocked("コレクションが指定されていません。`gp serve <collection-dir>` でディレクトリを指定して起動してください")
		s.mu.Unlock()
		s.render(w, s.indexTmpl, data)
		return
	}

	if err := model.SaveToCollection(s.CollectionDir, name, spec); err != nil {
		s.mu.Lock()
		data := s.pageDataLocked("保存に失敗しました: " + err.Error())
		s.mu.Unlock()
		s.render(w, s.indexTmpl, data)
		return
	}

	s.mu.Lock()
	data := s.pageDataLocked("")
	s.mu.Unlock()
	data.Saved = fmt.Sprintf("%q として保存しました", name)
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
		StatusCode:  result.StatusCode,
		Status:      result.Status,
		OK:          result.Ok(),
		Duration:    result.Duration.Round(time.Millisecond).String(),
		Bytes:       int64(len(result.Body)),
		Body:        string(result.Body),
		TestResults: testResultViews(result.TestResults),
		ConsoleLogs: result.ConsoleLogs,
	}
	if result.Err != nil {
		sr.Err = result.Err.Error()
	}
	sr.Headers = headerViews(result.Headers)

	data.SendResult = sr
	data.History = s.recordHistory(spec, env, sr)
	s.render(w, s.indexTmpl, data)
}

// recordHistory appends a single-send result to the in-memory history,
// evicting the oldest entry once maxHistoryEntries is exceeded, and returns
// the resulting history view list (newest first) for immediate rendering.
func (s *Server) recordHistory(spec model.RequestSpec, env map[string]string, sr *sendResultView) []historyRowView {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextHistoryID++
	s.history = append(s.history, historyEntry{
		ID:       s.nextHistoryID,
		Spec:     cloneSpec(spec),
		Env:      maps.Clone(env),
		OK:       sr.OK,
		Status:   sr.Status,
		Duration: sr.Duration,
		Bytes:    sr.Bytes,
		Err:      sr.Err,
	})
	if len(s.history) > maxHistoryEntries {
		s.history = s.history[len(s.history)-maxHistoryEntries:]
	}
	return s.historyViewsLocked()
}

// cloneSpec returns a copy of spec with its Headers map deep-copied, so the
// caller can retain a reference (e.g. in a history entry) independent of any
// later mutation of the original spec's Headers map.
func cloneSpec(spec model.RequestSpec) model.RequestSpec {
	spec.Headers = maps.Clone(spec.Headers)
	return spec
}

// handleHistoryRestore loads a past single-send entry's Method/URL/Headers/
// Body/Env back into the in-memory editing state (the same fields
// POST /execute writes to) and redirects to the index page. IDs are never
// reused, so a link to an entry evicted by the maxHistoryEntries cap fails
// with a clear error instead of silently restoring the wrong entry.
func (s *Server) handleHistoryRestore(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("履歴のIDが不正です"), http.StatusSeeOther)
		return
	}

	s.mu.Lock()
	var found *historyEntry
	for i := range s.history {
		if s.history[i].ID == id {
			found = &s.history[i]
			break
		}
	}
	if found != nil {
		s.spec = cloneSpec(found.Spec)
		s.env = maps.Clone(found.Env)
	}
	s.mu.Unlock()

	if found == nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("その履歴は見つかりませんでした（保持件数の上限を超えて破棄された可能性があります）"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// csvHistoryRow is a lightweight snapshot of one result row of a CSV/
// collection run, for history storage (issue #44). It deliberately mirrors
// only the small, bounded-size fields of rowView — never response headers,
// bodies, or any other data whose size scales with the response — so CSV
// history memory stays bounded regardless of what fields rowView carries
// (e.g. issue #42's per-row response detail). Fields are copied one at a
// time from rowView at the call site rather than via a wholesale struct
// copy, precisely so this stays correct even as rowView grows new fields.
type csvHistoryRow struct {
	Index        int
	Name         string // collection entry name, "" when not a collection run
	Status       string
	OK           bool
	Duration     string
	Bytes        int64
	Values       []string
	Err          string
	TestsSummary string
	// Headers and Body carry this row's captured response detail (issue
	// #42) through to the history redisplay. Since GET /csv-history/{id}
	// (via handleCSVHistoryShow) is now the *only* place a CSV/collection
	// run's results page is ever rendered — handleRunCSV responds with the
	// progress page instead (issue #45) — omitting these here would make
	// the per-row detail expando permanently unreachable. Already bounded
	// by runner.maxRowDetailBody per row and maxCSVHistoryEntries overall,
	// the same caps that already applied before #45 removed the
	// synchronous render.
	Headers []headerView
	Body    string
}

// csvHistoryEntry is one past CSV/collection run, holding just enough to
// redisplay its results page (GET /csv-history/{id}) — not enough to
// reconstruct the original runner.Result values (see csvHistoryRow).
type csvHistoryEntry struct {
	ID        int
	Timestamp time.Time
	// Target is s.RequestPath, or s.CollectionDir when a collection is
	// active, whichever describes what was run (see csvHistoryTarget).
	Target     string
	Collection bool
	HasTests   bool
	Columns    []string
	Rows       []csvHistoryRow
	Passed     int
	Total      int
}

// csvHistoryListView is the display-ready form of a csvHistoryEntry for the
// index page's "CSV実行履歴" card.
type csvHistoryListView struct {
	ID        int
	Timestamp string
	Target    string
	Passed    int
	Total     int
}

// csvHistoryTarget returns what to label a CSV/collection history entry
// with: the collection directory when one is active, else the request
// template path, else the same "新規リクエスト" placeholder results.html
// already uses for an empty RequestPath. RequestPath/CollectionDir are fixed
// at server startup, so this needs no locking.
func (s *Server) csvHistoryTarget() string {
	target := s.RequestPath
	if s.CollectionDir != "" {
		target = s.CollectionDir
	}
	if target == "" {
		target = "新規リクエスト"
	}
	return target
}

// csvHistoryViewsLocked returns the current CSV/collection run history,
// newest first. Callers must hold s.mu.
func (s *Server) csvHistoryViewsLocked() []csvHistoryListView {
	if len(s.csvHistory) == 0 {
		return nil
	}
	views := make([]csvHistoryListView, 0, len(s.csvHistory))
	for i := len(s.csvHistory) - 1; i >= 0; i-- {
		e := s.csvHistory[i]
		views = append(views, csvHistoryListView{
			ID:        e.ID,
			Timestamp: e.Timestamp.Format("2006-01-02 15:04:05"),
			Target:    e.Target,
			Passed:    e.Passed,
			Total:     e.Total,
		})
	}
	return views
}

// recordCSVHistory appends a just-built resultsView to the in-memory CSV
// history as a lightweight csvHistoryEntry, evicting the oldest entry once
// maxCSVHistoryEntries is exceeded, and returns the new entry's ID (issue
// #45's async handleRunCSV redirects the browser to /csv-history/{id} once
// the background run finishes).
func (s *Server) recordCSVHistory(view resultsView, target string) int {
	rows := make([]csvHistoryRow, len(view.Rows))
	for i, rv := range view.Rows {
		rows[i] = csvHistoryRow{
			Index:        rv.Index,
			Name:         rv.Name,
			Status:       rv.Status,
			OK:           rv.OK,
			Duration:     rv.Duration,
			Bytes:        rv.Bytes,
			Values:       append([]string(nil), rv.Values...),
			Err:          rv.Err,
			TestsSummary: rv.TestsSummary,
			Headers:      append([]headerView(nil), rv.Headers...),
			Body:         rv.Body,
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextCSVHistoryID++
	s.csvHistory = append(s.csvHistory, csvHistoryEntry{
		ID:         s.nextCSVHistoryID,
		Timestamp:  time.Now(),
		Target:     target,
		Collection: view.Collection,
		HasTests:   view.HasTests,
		Columns:    append([]string(nil), view.Columns...),
		Rows:       rows,
		Passed:     view.Passed,
		Total:      view.Total,
	})
	if len(s.csvHistory) > maxCSVHistoryEntries {
		s.csvHistory = s.csvHistory[len(s.csvHistory)-maxCSVHistoryEntries:]
	}
	return s.nextCSVHistoryID
}

// csvHistoryJSONRow is the JSON shape of one row of a redisplayed CSV/
// collection history entry, for the results page's "結果をダウンロード
// (JSON)" button on GET /csv-history/{id}. It's a separate, self-contained
// shape (not report.jsonResult) since the full runner.Result values behind a
// history entry were never retained — only the lightweight csvHistoryRow
// fields are available to encode.
type csvHistoryJSONRow struct {
	Index    int               `json:"index"`
	Name     string            `json:"name,omitempty"`
	Status   string            `json:"status"`
	Ok       bool              `json:"ok"`
	Duration string            `json:"duration"`
	Bytes    int64             `json:"bytes"`
	Row      map[string]string `json:"row,omitempty"`
	Error    string            `json:"error"`
	Tests    string            `json:"testsSummary,omitempty"`
}

// csvHistoryEntryToView reconstructs a resultsView from a stored
// csvHistoryEntry, for redisplaying a past CSV/collection run
// (GET /csv-history/{id}) — the only place this ever gets rendered, since
// issue #45. Includes each row's captured response detail (issue #42) so
// the per-row expando keeps working through the history-based redisplay.
func csvHistoryEntryToView(e csvHistoryEntry) resultsView {
	view := resultsView{
		Columns:    append([]string(nil), e.Columns...),
		Passed:     e.Passed,
		Total:      e.Total,
		Collection: e.Collection,
		HasTests:   e.HasTests,
	}

	jsonRows := make([]csvHistoryJSONRow, len(e.Rows))
	for i, hr := range e.Rows {
		view.Rows = append(view.Rows, rowView{
			Index:        hr.Index,
			Name:         hr.Name,
			Status:       hr.Status,
			OK:           hr.OK,
			Duration:     hr.Duration,
			Bytes:        hr.Bytes,
			Values:       hr.Values,
			Err:          hr.Err,
			TestsSummary: hr.TestsSummary,
			Headers:      hr.Headers,
			Body:         hr.Body,
		})

		row := make(map[string]string, len(e.Columns))
		for j, col := range e.Columns {
			if j < len(hr.Values) {
				row[col] = hr.Values[j]
			}
		}
		jsonRows[i] = csvHistoryJSONRow{
			Index:    hr.Index,
			Name:     hr.Name,
			Status:   hr.Status,
			Ok:       hr.OK,
			Duration: hr.Duration,
			Bytes:    hr.Bytes,
			Row:      row,
			Error:    hr.Err,
			Tests:    hr.TestsSummary,
		}
	}
	if b, err := json.Marshal(jsonRows); err == nil {
		view.ResultsJSON = template.JS(b)
	}
	view.ColSpan = resultsColSpan(view)
	return view
}

// handleCSVHistoryShow implements GET /csv-history/{id}: it redisplays a
// past CSV/collection run's results page from the stored lightweight
// snapshot (see csvHistoryEntryToView). Unlike handleHistoryRestore (which
// loads a past single-send entry back into the editable s.spec/s.env
// state), this is read-only — it never touches s.spec/s.env. IDs are never
// reused, so a link to an entry evicted by the maxCSVHistoryEntries cap
// fails with a clear error instead of silently showing a different entry.
func (s *Server) handleCSVHistoryShow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("履歴のIDが不正です"), http.StatusSeeOther)
		return
	}

	s.mu.Lock()
	var found *csvHistoryEntry
	for i := range s.csvHistory {
		if s.csvHistory[i].ID == id {
			found = &s.csvHistory[i]
			break
		}
	}
	var view resultsView
	if found != nil {
		view = csvHistoryEntryToView(*found)
	}
	view.RequestPath = s.RequestPath
	view.EnvPath = s.EnvPath
	s.mu.Unlock()

	if found == nil {
		http.Redirect(w, r, "/?error="+url.QueryEscape("その履歴は見つかりませんでした（保持件数の上限を超えて破棄された可能性があります）"), http.StatusSeeOther)
		return
	}
	s.render(w, s.resultsTmpl, view)
}

// handleRunCSV implements the "CSVで実行" action: it validates the upload and
// run options synchronously (exactly as before, same renderErr calls on bad
// input), then launches the actual request execution in a background
// goroutine and immediately responds with a progress page (issue #45). The
// browser polls GET /run-progress/{id} for completion, then navigates itself
// to the finished run's /csv-history/{id} page — the results/history/
// row-detail rendering itself is completely unchanged, only how the run is
// kicked off and observed is new.
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

	iterations := 1
	if v := strings.TrimSpace(r.FormValue("iterations")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			renderErr("反復回数は1以上の整数で指定してください")
			return
		}
		iterations = n
	}

	delay := time.Duration(0)
	if v := strings.TrimSpace(r.FormValue("delay")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			renderErr("遅延の形式が不正です（例: 500ms）: " + err.Error())
			return
		}
		delay = d
	}

	stopOnError := r.FormValue("stop_on_error") != ""

	collectionRun := s.CollectionDir != ""

	// rowCount/total mirror buildRowSequence's own simple arithmetic (not
	// called directly since it's unexported in another package): the number
	// of rows actually executed may end up lower than this if StopOnError
	// cuts the run short, which OnProgress's total parameter already
	// accounts for.
	rowCount := len(data.Rows)
	if rowCount == 0 {
		rowCount = 1
	}
	total := rowCount * iterations
	var specs []runner.NamedSpec
	if collectionRun {
		named, err := model.LoadAllFromCollection(s.CollectionDir)
		if err != nil {
			renderErr("コレクションの読み込みに失敗しました: " + err.Error())
			return
		}
		specs = make([]runner.NamedSpec, len(named))
		for i, n := range named {
			specs[i] = runner.NamedSpec{Name: n.Name, Spec: n.Spec}
		}
		total = len(specs) * rowCount * iterations
	}

	s.mu.Lock()
	s.nextRunID++
	id := s.nextRunID
	s.runs[id] = &csvRunProgress{Total: total}
	s.mu.Unlock()

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				s.mu.Lock()
				if p := s.runs[id]; p != nil {
					p.Done = true
					p.Err = fmt.Sprintf("CSV実行中に予期しないエラーが発生しました: %v", rec)
				}
				s.mu.Unlock()
			}
		}()

		// CaptureResponses is always true here (unlike CLI `gp run`, which
		// never sets it): the results table lets each row expand into its
		// response headers/body (issue #42).
		opts := runner.RunOptions{
			Iterations:       iterations,
			Delay:            delay,
			StopOnError:      stopOnError,
			CaptureResponses: true,
			OnProgress: func(completed, total int) {
				s.mu.Lock()
				if p := s.runs[id]; p != nil {
					p.Completed = completed
					p.Total = total
				}
				s.mu.Unlock()
			},
		}

		client := &http.Client{Timeout: s.Timeout}

		var view resultsView
		if collectionRun {
			results := runner.RunCollectionOpts(client, specs, env, data.Rows, opts)

			resultsJSON, err := report.CollectionResultsToJSON(results)
			if err != nil {
				s.mu.Lock()
				if p := s.runs[id]; p != nil {
					p.Done = true
					p.Err = "結果のJSON変換に失敗しました: " + err.Error()
				}
				s.mu.Unlock()
				return
			}

			view = resultsView{RequestPath: s.RequestPath, EnvPath: s.EnvPath, Columns: data.Columns, Total: len(results), Collection: true, ResultsJSON: template.JS(resultsJSON)}
			for _, cr := range results {
				res := cr.Result
				if res.Ok() {
					view.Passed++
				}
				if len(res.TestResults) > 0 {
					view.HasTests = true
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
					// Index is the 1-based iteration (CSV row) number,
					// matching report.PrintCollection's "#" column: it
					// repeats across the requests belonging to the same row
					// rather than counting each execution.
					Index:        cr.RowIndex + 1,
					Name:         cr.Name,
					Status:       status,
					OK:           res.Ok(),
					Duration:     res.Duration.Round(time.Millisecond).String(),
					Bytes:        res.Bytes,
					Values:       values,
					Err:          errMsg,
					TestsSummary: report.TestsSummary(res.TestResults),
					Headers:      headerViews(res.Headers),
					Body:         string(res.Body),
				})
			}
		} else {
			results := runner.RunOpts(client, &spec, env, data.Rows, opts)

			resultsJSON, err := report.ResultsToJSON(results)
			if err != nil {
				s.mu.Lock()
				if p := s.runs[id]; p != nil {
					p.Done = true
					p.Err = "結果のJSON変換に失敗しました: " + err.Error()
				}
				s.mu.Unlock()
				return
			}

			view = resultsView{RequestPath: s.RequestPath, EnvPath: s.EnvPath, Columns: data.Columns, Total: len(results), ResultsJSON: template.JS(resultsJSON)}
			for i, res := range results {
				if res.Ok() {
					view.Passed++
				}
				if len(res.TestResults) > 0 {
					view.HasTests = true
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
					Index:        i + 1,
					Status:       status,
					OK:           res.Ok(),
					Duration:     res.Duration.Round(time.Millisecond).String(),
					Bytes:        res.Bytes,
					Values:       values,
					Err:          errMsg,
					TestsSummary: report.TestsSummary(res.TestResults),
					Headers:      headerViews(res.Headers),
					Body:         string(res.Body),
				})
			}
		}

		view.ColSpan = resultsColSpan(view)
		historyID := s.recordCSVHistory(view, s.csvHistoryTarget())

		s.mu.Lock()
		if p := s.runs[id]; p != nil {
			p.Done = true
			p.HistoryID = historyID
		}
		s.pruneRunsLocked()
		s.mu.Unlock()
	}()

	s.render(w, s.progressTmpl, progressView{RunID: id})
}

// pruneRunsLocked evicts the oldest DONE entries from s.runs once it exceeds
// maxCSVRuns, bounding memory for a long-running gp serve session (issue
// #45). In-progress runs are never evicted. IDs are assigned in strictly
// increasing order, so the lowest IDs are always the oldest. Callers must
// hold s.mu.
func (s *Server) pruneRunsLocked() {
	for len(s.runs) > maxCSVRuns {
		oldestID := 0
		for id, p := range s.runs {
			if !p.Done {
				continue
			}
			if oldestID == 0 || id < oldestID {
				oldestID = id
			}
		}
		if oldestID == 0 {
			return // nothing evictable left (every remaining run is in-progress)
		}
		delete(s.runs, oldestID)
	}
}

// progressView renders the progress page for one async CSV/collection run
// (issue #45).
type progressView struct {
	RunID int
}

// runProgressJSON is the GET /run-progress/{id} JSON response shape, polled
// by the progress page's inline script.
type runProgressJSON struct {
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Done      bool   `json:"done"`
	HistoryID int    `json:"historyId,omitempty"`
	Error     string `json:"error,omitempty"`
}

// handleRunProgress implements GET /run-progress/{id}: it reports the
// current progress of an async CSV/collection run started by handleRunCSV,
// as JSON (this endpoint is polled by fetch(), never by a browser
// navigation, so every response — including "not found" — is JSON, never an
// HTML error page, mirroring the /api/send and /api/run convention).
func (s *Server) handleRunProgress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "invalid run id")
		return
	}

	s.mu.Lock()
	p, ok := s.runs[id]
	var resp runProgressJSON
	if ok {
		resp = runProgressJSON{Completed: p.Completed, Total: p.Total, Done: p.Done, HistoryID: p.HistoryID, Error: p.Err}
	}
	s.mu.Unlock()

	if !ok {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("unknown run id %d", id))
		return
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// resultsColSpan returns the total number of columns the results table
// renders for view, given its optional Request/Tests columns — see
// resultsView.ColSpan.
func resultsColSpan(view resultsView) int {
	// Fixed columns: #, Status, Time, Size, Error.
	n := 5 + len(view.Columns)
	if view.Collection {
		n++
	}
	if view.HasTests {
		n++
	}
	return n
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

	// Collection is true when this run executed every request in a
	// collection directory (see Server.CollectionDir) once per CSV row,
	// rather than a single request template. The results template shows
	// an extra "Request" column only in that case.
	Collection bool

	// HasTests is true when at least one row's underlying result actually
	// has test_script results, in which case the results template shows an
	// extra "Tests" column. Mirrors internal/report's Print/PrintCollection,
	// which likewise only add their TESTS column when it isn't all-empty.
	HasTests bool

	// ColSpan is the total number of columns in the results table (fixed
	// columns plus the CSV's own columns, plus Request/Tests when Collection/
	// HasTests add them), computed once here rather than in the template so
	// each row's expandable response-detail <tr> (issue #42) can span the
	// full table width regardless of which optional columns are showing.
	ColSpan int

	// ResultsJSON is the same results, JSON-encoded (see
	// report.ResultsToJSON / report.CollectionResultsToJSON), for the
	// "結果をダウンロード(JSON)" button (issue #36). It's embedded verbatim
	// as the body of a <script type="application/json"> element rather than
	// through a normal {{.}} string interpolation: html/template treats
	// application/json script bodies as a JS context (see
	// html/template's isJSType), so a plain string field would be
	// re-encoded as a quoted+escaped JS string literal instead of being
	// written as-is. template.JS opts out of that re-encoding; it's safe
	// here specifically because encoding/json's default Marshal behavior
	// HTML-escapes '<', '>' and '&' in string values (e.g. "<" becomes
	// "<"), so the encoded bytes can never contain a literal
	// "</script" that would break out of the tag.
	ResultsJSON template.JS
}

type rowView struct {
	Index int
	// Name is the collection entry name this row's request came from; only
	// meaningful (and only rendered) when the enclosing resultsView.
	// Collection is true.
	Name     string
	Status   string
	OK       bool
	Duration string
	Bytes    int64
	Values   []string
	Err      string
	// TestsSummary is the compact "passed/total[: first failure]" form of
	// this row's test_script results (see report.TestsSummary); "" when the
	// request has no test_script. Only rendered when the enclosing
	// resultsView.HasTests is true.
	TestsSummary string
	// Headers and Body are this row's captured response headers/body (issue
	// #42), populated whenever CaptureResponses was set on the run (always
	// true for handleRunCSV). Body is "" when nothing was captured (e.g. an
	// error before any response arrived), in which case the results
	// template shows no expandable detail row for it, matching the
	// empty-means-don't-render convention used by Name/TestsSummary above.
	Headers []headerView
	Body    string
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

// --- JSON API (issue #7): POST /api/send and POST /api/run ---
//
// These give AI agents/scripts a JSON-in-JSON-out alternative to the
// HTML-form-based POST /execute. Unlike /execute, which persists the
// submitted request/env into s.spec/s.env as the new baseline for the next
// page load, these two handlers are deliberately stateless: they execute
// exactly what's in the request and never touch s.spec/s.env. That makes
// them safe to call repeatedly/concurrently from a script without disturbing
// whatever a human has open in the browser at the same time. They also never
// write an HTML error page — every failure, including a malformed request
// body, comes back as a JSON object so a script never has to sniff the
// response body to find out whether it got JSON or HTML.

// apiSendRequest is the POST /api/send request body.
type apiSendRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Env     map[string]string `json:"env"`
}

// apiSendResponse is the POST /api/send response body: the JSON shape of one
// runner.SendResult.
type apiSendResponse struct {
	Status     string              `json:"status"`
	StatusCode int                 `json:"statusCode"`
	Ok         bool                `json:"ok"`
	DurationMs float64             `json:"durationMs"`
	Bytes      int64               `json:"bytes"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	Error      string              `json:"error"`
}

// apiRunResult is one element of the POST /api/run response array: the JSON
// shape of one runner.Result. Field names/JSON tags are kept identical to
// report.jsonResult (internal/report/json.go, used by `gp run --format
// json`) so the two JSON representations stay consistent; that type is
// unexported so this is a separate, parallel definition rather than a shared
// one.
type apiRunResult struct {
	Index      int               `json:"index"`
	Status     string            `json:"status"`
	StatusCode int               `json:"statusCode"`
	Ok         bool              `json:"ok"`
	DurationMs float64           `json:"durationMs"`
	Bytes      int64             `json:"bytes"`
	Row        map[string]string `json:"row,omitempty"`
	Error      string            `json:"error"`
}

// apiErrorBody is the JSON shape of an API error response.
type apiErrorBody struct {
	Error string `json:"error"`
}

// writeAPIJSON writes v as the JSON response body with the given status code.
func writeAPIJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeAPIError writes a {"error": msg} JSON body with the given status
// code. Used for every failure path in the JSON API handlers, so a script
// never receives an HTML error page from these endpoints.
func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeAPIJSON(w, status, apiErrorBody{Error: msg})
}

// parseJSONStringMap decodes s (a JSON object of string->string, e.g.
// `{"Accept":"application/json"}`) into a map. An empty/blank s yields a nil
// map and no error, so the "headers"/"env" form fields of POST /api/run can
// be omitted entirely.
func parseJSONStringMap(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// handleAPISend implements POST /api/send: decodes a JSON request body,
// sends it via runner.Send (the same function handleSend uses), and responds
// with the result as JSON. It does not read or modify s.spec/s.env at all —
// every field needed to build the request comes from the request body.
func (s *Server) handleAPISend(w http.ResponseWriter, r *http.Request) {
	var req apiSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = "GET" // matches model.LoadRequest's default-method handling
	}
	spec := model.RequestSpec{
		Method:  method,
		URL:     strings.TrimSpace(req.URL),
		Headers: req.Headers,
		Body:    req.Body,
	}

	client := &http.Client{Timeout: s.Timeout}
	result := runner.Send(client, &spec, req.Env, maxSendPreviewBody)

	resp := apiSendResponse{
		Status:     result.Status,
		StatusCode: result.StatusCode,
		Ok:         result.Ok(),
		DurationMs: float64(result.Duration.Microseconds()) / 1000.0,
		Bytes:      int64(len(result.Body)),
		Headers:    result.Headers,
		Body:       string(result.Body),
	}
	if result.Err != nil {
		resp.Error = result.Err.Error()
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

// handleAPIRun implements POST /api/run: decodes a multipart/form-data
// request (method/url/body as plain fields, headers/env as JSON-object
// strings, csv as a file field), runs it via runner.Run (the same function
// handleRunCSV uses), and responds with one JSON object per CSV row. Like
// handleAPISend, it does not read or modify s.spec/s.env.
func (s *Server) handleAPIRun(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}

	method := strings.TrimSpace(r.FormValue("method"))
	if method == "" {
		method = "GET"
	}

	headers, err := parseJSONStringMap(r.FormValue("headers"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid \"headers\" field: must be a JSON object string: "+err.Error())
		return
	}
	env, err := parseJSONStringMap(r.FormValue("env"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid \"env\" field: must be a JSON object string: "+err.Error())
		return
	}

	spec := model.RequestSpec{
		Method:  method,
		URL:     strings.TrimSpace(r.FormValue("url")),
		Headers: headers,
		Body:    r.FormValue("body"),
	}

	file, _, err := r.FormFile("csv")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "missing or unreadable \"csv\" file field: "+err.Error())
		return
	}
	defer file.Close()

	data, err := model.ParseCSV(file)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "failed to parse csv: "+err.Error())
		return
	}

	client := &http.Client{Timeout: s.Timeout}
	results := runner.Run(client, &spec, env, data.Rows)

	out := make([]apiRunResult, len(results))
	for i, res := range results {
		errMsg := ""
		if res.Err != nil {
			errMsg = res.Err.Error()
		}
		out[i] = apiRunResult{
			Index:      i + 1,
			Status:     res.Status,
			StatusCode: res.StatusCode,
			Ok:         res.Ok(),
			DurationMs: float64(res.Duration.Microseconds()) / 1000.0,
			Bytes:      res.Bytes,
			Row:        res.Row,
			Error:      errMsg,
		}
	}

	writeAPIJSON(w, http.StatusOK, out)
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
