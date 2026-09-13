// Package report renders runner.Result slices as a terminal table.
package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"green-pepper/internal/runner"
)

// Print writes one row per result to w, followed by a pass/fail summary.
// columns lists the CSV variable names to show, in order.
func Print(w io.Writer, results []runner.Result, columns []string) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	header := []string{"#", "STATUS", "TIME", "SIZE"}
	header = append(header, columns...)
	header = append(header, "ERROR")
	fmt.Fprintln(tw, joinTab(header))

	passed := 0
	for i, r := range results {
		if r.Ok() {
			passed++
		}

		status := r.Status
		if status == "" {
			status = "-"
		}
		errMsg := ""
		if r.Err != nil {
			errMsg = r.Err.Error()
		}

		row := []string{
			fmt.Sprintf("%d", i+1),
			status,
			r.Duration.Round(time.Millisecond).String(),
			fmt.Sprintf("%d", r.Bytes),
		}
		for _, col := range columns {
			row = append(row, r.Row[col])
		}
		row = append(row, errMsg)
		fmt.Fprintln(tw, joinTab(row))
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%d/%d passed\n", passed, len(results))
}

func joinTab(fields []string) string {
	return strings.Join(fields, "\t")
}

// PrintCollection writes one row per request execution to w (a collection
// run: every named request, once per CSV row), followed by a pass/fail
// summary. columns lists the CSV variable names to show, in order. The "#"
// column is the 1-based iteration (CSV row) number, so it repeats across the
// requests belonging to the same row.
func PrintCollection(w io.Writer, results []runner.CollectionResult, columns []string) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	header := []string{"#", "REQUEST", "STATUS", "TIME", "SIZE"}
	header = append(header, columns...)
	header = append(header, "ERROR")
	fmt.Fprintln(tw, joinTab(header))

	passed := 0
	for _, cr := range results {
		r := cr.Result
		if r.Ok() {
			passed++
		}

		status := r.Status
		if status == "" {
			status = "-"
		}
		errMsg := ""
		if r.Err != nil {
			errMsg = r.Err.Error()
		}

		row := []string{
			fmt.Sprintf("%d", cr.RowIndex+1),
			cr.Name,
			status,
			r.Duration.Round(time.Millisecond).String(),
			fmt.Sprintf("%d", r.Bytes),
		}
		for _, col := range columns {
			row = append(row, r.Row[col])
		}
		row = append(row, errMsg)
		fmt.Fprintln(tw, joinTab(row))
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%d/%d passed\n", passed, len(results))
}
