package report

import (
	"encoding/json"
	"io"

	"green-pepper/internal/runner"
)

// jsonResult is the JSON shape of one runner.Result, for --format json output.
type jsonResult struct {
	Index      int               `json:"index"`
	Status     string            `json:"status"`
	StatusCode int               `json:"statusCode"`
	Ok         bool              `json:"ok"`
	DurationMs float64           `json:"durationMs"`
	Bytes      int64             `json:"bytes"`
	Row        map[string]string `json:"row,omitempty"`
	Error      string            `json:"error"`
	// Tests holds the request's test_script results, if it has one. Omitted
	// entirely (not even an empty array) when the request has no
	// test_script, so JSON output for the common case is unchanged from
	// before this field existed.
	Tests []jsonTestResult `json:"tests,omitempty"`
	// Logs holds any console.log/warn/error output the test_script produced,
	// if it has one. Omitted entirely when empty/absent, same as Tests.
	Logs []string `json:"logs,omitempty"`
}

// jsonTestResult is the JSON shape of one runner.TestResult.
type jsonTestResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

// testResultsToJSON converts a runner.TestResult slice into the
// jsonTestResult shape shared by jsonResult and jsonCollectionResult. Returns
// nil (which json.Marshal with omitempty renders as an absent field) for an
// empty/nil input.
func testResultsToJSON(results []runner.TestResult) []jsonTestResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]jsonTestResult, len(results))
	for i, t := range results {
		out[i] = jsonTestResult{Name: t.Name, Passed: t.Passed, Error: t.Error}
	}
	return out
}

// resultsToJSON converts results into the jsonResult shape shared by
// PrintJSON and ResultsToJSON.
func resultsToJSON(results []runner.Result) []jsonResult {
	out := make([]jsonResult, len(results))
	for i, r := range results {
		errMsg := ""
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		out[i] = jsonResult{
			Index:      i + 1,
			Status:     r.Status,
			StatusCode: r.StatusCode,
			Ok:         r.Ok(),
			DurationMs: float64(r.Duration.Microseconds()) / 1000.0,
			Bytes:      r.Bytes,
			Row:        r.Row,
			Error:      errMsg,
			Tests:      testResultsToJSON(r.TestResults),
			Logs:       consoleLogsToJSON(r.ConsoleLogs),
		}
	}
	return out
}

// consoleLogsToJSON returns logs unchanged unless it's empty, in which case
// it returns nil (which json.Marshal with omitempty renders as an absent
// field) — mirroring testResultsToJSON's empty-input handling for the
// parallel "logs" field.
func consoleLogsToJSON(logs []string) []string {
	if len(logs) == 0 {
		return nil
	}
	return logs
}

// PrintJSON writes results to w as a JSON array, one object per row/request.
func PrintJSON(w io.Writer, results []runner.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resultsToJSON(results))
}

// ResultsToJSON marshals results into the same JSON shape PrintJSON writes
// (one object per row/request, field names matching jsonResult), for callers
// such as internal/server that need the encoded bytes directly rather than
// writing to an io.Writer. This keeps the Web UI's exported JSON consistent
// with `gp run --format json` without duplicating the field-mapping logic.
func ResultsToJSON(results []runner.Result) ([]byte, error) {
	return json.Marshal(resultsToJSON(results))
}

// jsonCollectionResult is the JSON shape of one runner.CollectionResult, for
// --format json output of a collection run.
type jsonCollectionResult struct {
	Iteration  int               `json:"iteration"`
	Request    string            `json:"request"`
	Status     string            `json:"status"`
	StatusCode int               `json:"statusCode"`
	Ok         bool              `json:"ok"`
	DurationMs float64           `json:"durationMs"`
	Bytes      int64             `json:"bytes"`
	Row        map[string]string `json:"row,omitempty"`
	Error      string            `json:"error"`
	// Tests holds the request's test_script results, if it has one; see
	// jsonResult.Tests.
	Tests []jsonTestResult `json:"tests,omitempty"`
	// Logs holds the request's test_script console output, if it has one;
	// see jsonResult.Logs.
	Logs []string `json:"logs,omitempty"`
}

// collectionResultsToJSON converts results into the jsonCollectionResult
// shape shared by PrintCollectionJSON and CollectionResultsToJSON.
func collectionResultsToJSON(results []runner.CollectionResult) []jsonCollectionResult {
	out := make([]jsonCollectionResult, len(results))
	for i, cr := range results {
		r := cr.Result
		errMsg := ""
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		out[i] = jsonCollectionResult{
			Iteration:  cr.RowIndex + 1,
			Request:    cr.Name,
			Status:     r.Status,
			StatusCode: r.StatusCode,
			Ok:         r.Ok(),
			DurationMs: float64(r.Duration.Microseconds()) / 1000.0,
			Bytes:      r.Bytes,
			Row:        r.Row,
			Error:      errMsg,
			Tests:      testResultsToJSON(r.TestResults),
			Logs:       consoleLogsToJSON(r.ConsoleLogs),
		}
	}
	return out
}

// PrintCollectionJSON writes results to w as a JSON array, one object per
// request execution (every named request, once per CSV row).
func PrintCollectionJSON(w io.Writer, results []runner.CollectionResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(collectionResultsToJSON(results))
}

// CollectionResultsToJSON marshals results into the same JSON shape
// PrintCollectionJSON writes (field names matching jsonCollectionResult), for
// callers such as internal/server that need the encoded bytes directly
// rather than writing to an io.Writer.
func CollectionResultsToJSON(results []runner.CollectionResult) ([]byte, error) {
	return json.Marshal(collectionResultsToJSON(results))
}
