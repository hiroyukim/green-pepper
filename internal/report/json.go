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
		}
	}
	return out
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
