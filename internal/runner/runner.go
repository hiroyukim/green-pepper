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

func mergeVars(env, row map[string]string) map[string]string {
	vars := make(map[string]string, len(env)+len(row))
	maps.Copy(vars, env)
	maps.Copy(vars, row)
	return vars
}

func runOne(client *http.Client, spec *model.RequestSpec, vars, row map[string]string) Result {
	method, err := tmpl.Render(spec.Method, vars)
	if err != nil {
		return Result{Row: row, Err: err}
	}
	url, err := tmpl.Render(spec.URL, vars)
	if err != nil {
		return Result{Row: row, Err: err}
	}
	body, err := tmpl.Render(spec.Body, vars)
	if err != nil {
		return Result{Row: row, Err: err}
	}

	req, err := http.NewRequest(strings.ToUpper(method), url, strings.NewReader(body))
	if err != nil {
		return Result{Row: row, Err: err}
	}
	for name, value := range spec.Headers {
		headerValue, err := tmpl.Render(value, vars)
		if err != nil {
			return Result{Row: row, Err: err}
		}
		req.Header.Set(name, headerValue)
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
