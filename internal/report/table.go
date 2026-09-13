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
// columns lists the CSV variable names to show, in order. A TESTS column
// (right before ERROR) is included only when at least one result actually
// has TestResults (i.e. some request in this run has a test_script) — this
// keeps table output byte-for-byte unchanged for the overwhelming majority
// of runs that don't use the feature at all, rather than adding an
// always-empty column that's pure noise for them.
func Print(w io.Writer, results []runner.Result, columns []string) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	showTests := anyHasTests(results)

	header := []string{"#", "STATUS", "TIME", "SIZE"}
	header = append(header, columns...)
	if showTests {
		header = append(header, "TESTS")
	}
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
		if showTests {
			row = append(row, TestsSummary(r.TestResults))
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

// anyHasTests reports whether any result in results has a non-empty
// TestResults, i.e. whether this run involved at least one request with a
// test_script.
func anyHasTests(results []runner.Result) bool {
	for _, r := range results {
		if len(r.TestResults) > 0 {
			return true
		}
	}
	return false
}

// anyCollectionHasTests is anyHasTests for a collection run.
func anyCollectionHasTests(results []runner.CollectionResult) bool {
	for _, cr := range results {
		if len(cr.Result.TestResults) > 0 {
			return true
		}
	}
	return false
}

// TestsSummary renders results as a compact "passed/total" string for a
// fixed-width table cell, e.g. "2/2" when every test passed, or
// "1/2: assert status is 200" naming the first failing test otherwise. It
// returns "" for an empty/nil results (the common case: the request has no
// test_script), and is shared by the CLI table (Print/PrintCollection) and
// the Web UI's CSV-run results table for a consistent compact form.
func TestsSummary(results []runner.TestResult) string {
	if len(results) == 0 {
		return ""
	}
	passed := 0
	firstFailure := ""
	for _, t := range results {
		if t.Passed {
			passed++
		} else if firstFailure == "" {
			firstFailure = t.Name
		}
	}
	summary := fmt.Sprintf("%d/%d", passed, len(results))
	if passed < len(results) {
		summary += ": " + firstFailure
	}
	return summary
}

// PrintCollection writes one row per request execution to w (a collection
// run: every named request, once per CSV row), followed by a pass/fail
// summary. columns lists the CSV variable names to show, in order. The "#"
// column is the 1-based iteration (CSV row) number, so it repeats across the
// requests belonging to the same row. Like Print, the TESTS column only
// appears when at least one request execution actually has test results.
func PrintCollection(w io.Writer, results []runner.CollectionResult, columns []string) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)

	showTests := anyCollectionHasTests(results)

	header := []string{"#", "REQUEST", "STATUS", "TIME", "SIZE"}
	header = append(header, columns...)
	if showTests {
		header = append(header, "TESTS")
	}
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
		if showTests {
			row = append(row, TestsSummary(r.TestResults))
		}
		row = append(row, errMsg)
		fmt.Fprintln(tw, joinTab(row))
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%d/%d passed\n", passed, len(results))
}
