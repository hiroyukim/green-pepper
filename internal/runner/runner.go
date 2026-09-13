// Package runner executes a request template once per variable set and
// reports the outcome of each execution.
package runner

import (
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"green-pepper/internal/model"
	"green-pepper/internal/tmpl"
)

// Result is the outcome of executing the request template for one row.
type Result struct {
	Row        map[string]string
	StatusCode int
	Status     string
	Duration   time.Duration
	Bytes      int64
	Err        error
	// TestResults holds the outcome of each pm.test(...) call from the
	// request's test_script, if it has one. Empty/nil when the request has
	// no test_script (the common case) — Ok() below is unaffected by
	// TestResults in that case, so this is a behavior-preserving addition.
	TestResults []TestResult
}

// Ok reports whether the request completed with a successful (2xx) status
// and, if it has a test_script, every one of its tests also passed. With no
// test_script (TestResults empty), this is exactly the original status-code-
// only check.
func (r Result) Ok() bool {
	if r.Err != nil || r.StatusCode < 200 || r.StatusCode >= 300 {
		return false
	}
	for _, t := range r.TestResults {
		if !t.Passed {
			return false
		}
	}
	return true
}

// Run executes spec once for every row in rows, merging env as the default
// variable set (a row value with the same name overrides env). It is a thin
// wrapper around RunOpts with the zero-value RunOptions (one pass over rows,
// no delay, never stopping early on failure) — its behavior is unchanged by
// the addition of RunOpts.
func Run(client *http.Client, spec *model.RequestSpec, env map[string]string, rows []map[string]string) []Result {
	return RunOpts(client, spec, env, rows, RunOptions{})
}

// RunOptions controls how a run iterates and reacts to failures. The zero
// value reproduces the original Run/RunCollection behavior exactly:
// Iterations <= 1 is treated as a single pass over rows, Delay 0 means no
// wait between requests, and StopOnError false means every row/spec is
// executed regardless of earlier failures.
type RunOptions struct {
	// Iterations is how many times the full set of rows (or, with no CSV,
	// the single implicit request) is repeated. <= 1 means exactly once.
	Iterations int
	// Delay is how long to wait before each request execution after the
	// very first one overall. Zero means no delay.
	Delay time.Duration
	// StopOnError, when true, stops the run at the first failed request
	// (Result.Ok() false) instead of continuing through the rest.
	StopOnError bool
}

// buildRowSequence expands rows into the effective, in-order sequence of rows
// to execute for the given iteration count: rows itself (or a single
// implicit empty row when rows is empty) repeated iterations times. An
// iterations of <= 1 returns rows (or the implicit row) unchanged, exactly
// matching the pre-iteration behavior of Run/RunCollection.
func buildRowSequence(rows []map[string]string, iterations int) []map[string]string {
	if len(rows) == 0 {
		rows = []map[string]string{{}}
	}
	if iterations <= 1 {
		return rows
	}

	seq := make([]map[string]string, 0, len(rows)*iterations)
	for i := 0; i < iterations; i++ {
		seq = append(seq, rows...)
	}
	return seq
}

// RunOpts is Run with explicit RunOptions: it can repeat the row sequence
// multiple times (opts.Iterations), wait between requests (opts.Delay), and
// stop at the first failure (opts.StopOnError). See RunOptions for the exact
// semantics of each field.
func RunOpts(client *http.Client, spec *model.RequestSpec, env map[string]string, rows []map[string]string, opts RunOptions) []Result {
	seq := buildRowSequence(rows, opts.Iterations)

	results := make([]Result, 0, len(seq))
	for i, row := range seq {
		if i > 0 && opts.Delay > 0 {
			time.Sleep(opts.Delay)
		}
		res := runOne(client, spec, mergeVars(env, row), row)
		results = append(results, res)
		if opts.StopOnError && !res.Ok() {
			break
		}
	}
	return results
}

// NamedSpec pairs a request template with its display name (a collection
// entry's filename without extension), for running multiple requests per
// CSV row ("iteration").
type NamedSpec struct {
	Name string
	Spec *model.RequestSpec
}

// CollectionResult is the outcome of one named request within one iteration
// (CSV row, or the single implicit iteration when there's no CSV) of a
// collection run.
type CollectionResult struct {
	RowIndex int
	Row      map[string]string
	Name     string
	Result   Result
}

// RunCollection executes every spec in specs, in order, once per row in rows
// (or once with an empty row if rows is empty — same convention as Run),
// merging env as the default variable set per row exactly like Run does.
// Results are ordered iteration-by-iteration: all of specs for row 1, then
// all of specs for row 2, etc. — this is the deterministic order the issue
// asks for (specs themselves should already be sorted by name by the
// caller, e.g. via model.ListCollection's sorted output).
// It is a thin wrapper around RunCollectionOpts with the zero-value
// RunOptions — its behavior is unchanged by the addition of
// RunCollectionOpts.
func RunCollection(client *http.Client, specs []NamedSpec, env map[string]string, rows []map[string]string) []CollectionResult {
	return RunCollectionOpts(client, specs, env, rows, RunOptions{})
}

// RunCollectionOpts is RunCollection with explicit RunOptions: it can repeat
// the row sequence multiple times (opts.Iterations), wait between every
// request execution across all specs and rows (opts.Delay), and stop at the
// first failure (opts.StopOnError), abandoning the rest of that row's specs
// and any remaining rows. See RunOptions for the exact semantics of each
// field.
func RunCollectionOpts(client *http.Client, specs []NamedSpec, env map[string]string, rows []map[string]string, opts RunOptions) []CollectionResult {
	seq := buildRowSequence(rows, opts.Iterations)

	results := make([]CollectionResult, 0, len(specs)*len(seq))
	n := 0
outer:
	for rowIdx, row := range seq {
		vars := mergeVars(env, row)
		for _, ns := range specs {
			if n > 0 && opts.Delay > 0 {
				time.Sleep(opts.Delay)
			}
			n++
			res := runOne(client, ns.Spec, vars, row)
			results = append(results, CollectionResult{
				RowIndex: rowIdx,
				Row:      row,
				Name:     ns.Name,
				Result:   res,
			})
			if opts.StopOnError && !res.Ok() {
				break outer
			}
		}
	}
	return results
}

func mergeVars(env, row map[string]string) map[string]string {
	vars := make(map[string]string, len(env)+len(row))
	maps.Copy(vars, env)
	maps.Copy(vars, row)
	return vars
}

func buildRequest(spec *model.RequestSpec, vars map[string]string) (*http.Request, error) {
	method, err := tmpl.Render(spec.Method, vars)
	if err != nil {
		return nil, err
	}
	url, err := tmpl.Render(spec.URL, vars)
	if err != nil {
		return nil, err
	}
	body, err := tmpl.Render(spec.Body, vars)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(strings.ToUpper(method), url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	for name, value := range spec.Headers {
		headerValue, err := tmpl.Render(value, vars)
		if err != nil {
			return nil, err
		}
		req.Header.Set(name, headerValue)
	}
	return req, nil
}

func runOne(client *http.Client, spec *model.RequestSpec, vars, row map[string]string) Result {
	req, err := buildRequest(spec, vars)
	if err != nil {
		return Result{Row: row, Err: err}
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return Result{Row: row, Duration: duration, Err: err}
	}
	defer resp.Body.Close()

	if spec.TestScript == "" {
		// Fast path, unchanged from before test scripts existed: the body
		// isn't needed for anything, so just count its bytes.
		n, _ := io.Copy(io.Discard, resp.Body)
		return Result{
			Row:        row,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Duration:   duration,
			Bytes:      n,
		}
	}

	// The test script needs the actual body (for pm.response.body/json()),
	// capped at maxTestScriptBody; anything beyond that is still drained (not
	// left unread, which would prevent connection reuse) and counted towards
	// Bytes so the reported size matches the fast path's.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxTestScriptBody))
	extra, _ := io.Copy(io.Discard, resp.Body)

	testResults := runTestScript(spec.TestScript, resp.StatusCode, resp.Status, body, vars)

	return Result{
		Row:         row,
		StatusCode:  resp.StatusCode,
		Status:      resp.Status,
		Duration:    duration,
		Bytes:       int64(len(body)) + extra,
		TestResults: testResults,
	}
}

// SendResult is the outcome of a single ad-hoc request, capturing the full
// response headers and body (unlike Result, which only records size, for
// compact display in a batch table).
type SendResult struct {
	StatusCode int
	Status     string
	Duration   time.Duration
	Headers    http.Header
	Body       []byte
	Err        error
	// TestResults holds the outcome of each pm.test(...) call from the
	// request's test_script, if it has one. Empty/nil when the request has
	// no test_script.
	TestResults []TestResult
}

// Ok reports whether the request completed with a successful (2xx) status
// and, if it has a test_script, every one of its tests also passed. See
// Result.Ok for the exact (behavior-preserving) semantics.
func (r SendResult) Ok() bool {
	if r.Err != nil || r.StatusCode < 200 || r.StatusCode >= 300 {
		return false
	}
	for _, t := range r.TestResults {
		if !t.Passed {
			return false
		}
	}
	return true
}

// Send executes spec once against vars, capturing at most maxBody bytes of
// the response body for display.
func Send(client *http.Client, spec *model.RequestSpec, vars map[string]string, maxBody int64) SendResult {
	req, err := buildRequest(spec, vars)
	if err != nil {
		return SendResult{Err: err}
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return SendResult{Duration: duration, Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return SendResult{StatusCode: resp.StatusCode, Status: resp.Status, Duration: duration, Headers: resp.Header, Err: err}
	}

	var testResults []TestResult
	if spec.TestScript != "" {
		testResults = runTestScript(spec.TestScript, resp.StatusCode, resp.Status, body, vars)
	}

	return SendResult{
		StatusCode:  resp.StatusCode,
		Status:      resp.Status,
		Duration:    duration,
		Headers:     resp.Header,
		Body:        body,
		TestResults: testResults,
	}
}
