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
}

// Ok reports whether the request completed with a successful (2xx) status.
func (r Result) Ok() bool {
	return r.Err == nil && r.StatusCode >= 200 && r.StatusCode < 300
}

// Run executes spec once for every row in rows, merging env as the default
// variable set (a row value with the same name overrides env).
func Run(client *http.Client, spec *model.RequestSpec, env map[string]string, rows []map[string]string) []Result {
	if len(rows) == 0 {
		rows = []map[string]string{{}}
	}

	results := make([]Result, 0, len(rows))
	for _, row := range rows {
		results = append(results, runOne(client, spec, mergeVars(env, row), row))
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
func RunCollection(client *http.Client, specs []NamedSpec, env map[string]string, rows []map[string]string) []CollectionResult {
	if len(rows) == 0 {
		rows = []map[string]string{{}}
	}

	results := make([]CollectionResult, 0, len(specs)*len(rows))
	for rowIdx, row := range rows {
		vars := mergeVars(env, row)
		for _, ns := range specs {
			results = append(results, CollectionResult{
				RowIndex: rowIdx,
				Row:      row,
				Name:     ns.Name,
				Result:   runOne(client, ns.Spec, vars, row),
			})
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

	n, _ := io.Copy(io.Discard, resp.Body)

	return Result{
		Row:        row,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Duration:   duration,
		Bytes:      n,
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
}

// Ok reports whether the request completed with a successful (2xx) status.
func (r SendResult) Ok() bool {
	return r.Err == nil && r.StatusCode >= 200 && r.StatusCode < 300
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

	return SendResult{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Duration:   duration,
		Headers:    resp.Header,
		Body:       body,
	}
}
