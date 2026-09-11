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

// PrintJSON writes results to w as a JSON array, one object per row/request.
func PrintJSON(w io.Writer, results []runner.Result) error {
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

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
