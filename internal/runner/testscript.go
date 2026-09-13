package runner

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// maxTestScriptBody caps how much of a response body is read into memory to
// hand to a test script (via pm.response.body / pm.response.json()). This
// mirrors internal/server's maxSendPreviewBody but stays local to this
// package since internal/runner does not otherwise depend on internal/server.
const maxTestScriptBody = 1 << 20 // 1 MiB

// testScriptTimeout bounds how long a single request's test script may run,
// guarding against an accidental (or malicious) infinite loop. This is local
// developer tooling rather than a hosted service, so a generous timeout is
// fine as long as it's finite.
const testScriptTimeout = 5 * time.Second

// maxConsoleLogLines caps how many console.log/warn/error lines a single
// test script run may record, so a pathological script (e.g. an infinite
// loop logging on every iteration) can't consume unbounded memory before
// testScriptTimeout fires. Once the cap is hit, further calls are silently
// dropped except for one final marker line.
const maxConsoleLogLines = 200

// consoleLogLimitMarker is appended once, after the last real log line, when
// a script logs past maxConsoleLogLines.
const consoleLogLimitMarker = "...(log limit reached)"

// TestResult is the outcome of one pm.test(...) call within a request's test
// script. A script that never calls pm.test (or fails before reaching one)
// still surfaces as a single synthetic TestResult (see runTestScript) so a
// broken script is never silently invisible.
type TestResult struct {
	Name   string
	Passed bool
	Error  string // empty when Passed is true
}

// runTestScript runs script (a goja/ECMAScript snippet) against one
// request's response, returning the pm.test(...) results it recorded along
// with any console.log/warn/error output the script produced. It returns nil
// for both immediately when script is empty, without constructing a goja
// runtime at all — the fast path for the overwhelming majority of requests
// that don't use this feature.
//
// The script gets a "pm" object exposing a small, purpose-built test API:
//
//   - pm.response.code (int), pm.response.status (string)
//   - pm.response.body (string, the raw response body)
//   - pm.response.json() — JSON.parse of the body; throws on invalid JSON
//   - pm.variables.get(name) — looks up vars, "" when absent
//   - pm.test(name, fn) — runs fn; an exception means a failed test, a
//     normal return means a passed one; either way it's recorded under name
//     and does not stop the rest of the script from running
//
// It also gets a top-level "console" object (not nested under pm, matching
// how console is a real global distinct from any particular API surface in
// actual JS environments), for debugging the script itself:
//
//   - console.log/warn/error(...args) — each formats its arguments (see
//     formatConsoleArg) and joins them with a single space, matching how
//     console.log("a", "b") prints "a b"; the resulting line is appended to
//     the returned logs. warn/error prefix the line with "[warn]"/"[error]"
//     so the three are distinguishable in the flat []string output; there is
//     no separate level field.
//
// Logging is capped at maxConsoleLogLines lines (see its doc comment) and is
// captured regardless of how the script ends — a later panic, thrown
// exception, or timeout does not discard log lines already recorded.
//
// The script runs under a timeout (testScriptTimeout): if it doesn't finish
// in time, goja is interrupted and that shows up as a failure like any other
// script error. A script that throws outside of any pm.test(...) call (a
// syntax error, an uncaught exception, or a timeout) is recorded as a single
// synthetic TestResult named "test_script", appended after whatever
// pm.test(...) results were already recorded — so a broken script always
// shows up as *some* visible failure. A top-level recover() is a last line
// of defense so a bug in this function can never crash the calling
// gp run/gp serve process.
func runTestScript(script string, statusCode int, status string, body []byte, vars map[string]string) (results []TestResult, logs []string) {
	if script == "" {
		return nil, nil
	}

	defer func() {
		if rec := recover(); rec != nil {
			results = append(results, TestResult{
				Name:   "test_script",
				Passed: false,
				Error:  fmt.Sprintf("test script panicked: %v", rec),
			})
		}
	}()

	appendLog := func(prefix string, args []goja.Value) {
		if len(logs) > maxConsoleLogLines {
			return // already hit the cap and appended the marker
		}
		if len(logs) == maxConsoleLogLines {
			logs = append(logs, consoleLogLimitMarker)
			return
		}
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = formatConsoleArg(a)
		}
		line := strings.Join(parts, " ")
		if prefix != "" {
			line = prefix + " " + line
		}
		logs = append(logs, line)
	}

	vm := goja.New()

	responseObj := vm.NewObject()
	_ = responseObj.Set("code", statusCode)
	_ = responseObj.Set("status", status)
	_ = responseObj.Set("body", string(body))
	_ = responseObj.Set("json", func() (any, error) {
		var parsed any
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("pm.response.json(): %w", err)
		}
		return parsed, nil
	})

	variablesObj := vm.NewObject()
	_ = variablesObj.Set("get", func(name string) string {
		return vars[name]
	})

	pmObj := vm.NewObject()
	_ = pmObj.Set("response", responseObj)
	_ = pmObj.Set("variables", variablesObj)
	_ = pmObj.Set("test", func(name string, fn goja.Callable) {
		_, err := fn(goja.Undefined())
		if err != nil {
			results = append(results, TestResult{Name: name, Passed: false, Error: err.Error()})
		} else {
			results = append(results, TestResult{Name: name, Passed: true})
		}
	})

	consoleObj := vm.NewObject()
	_ = consoleObj.Set("log", func(args ...goja.Value) { appendLog("", args) })
	_ = consoleObj.Set("warn", func(args ...goja.Value) { appendLog("[warn]", args) })
	_ = consoleObj.Set("error", func(args ...goja.Value) { appendLog("[error]", args) })

	if err := vm.Set("pm", pmObj); err != nil {
		results = append(results, TestResult{Name: "test_script", Passed: false, Error: err.Error()})
		return results, logs
	}
	if err := vm.Set("console", consoleObj); err != nil {
		results = append(results, TestResult{Name: "test_script", Passed: false, Error: err.Error()})
		return results, logs
	}

	timer := time.AfterFunc(testScriptTimeout, func() {
		vm.Interrupt(fmt.Sprintf("test script timed out after %s", testScriptTimeout))
	})
	_, err := vm.RunString(script)
	timer.Stop()

	if err != nil {
		results = append(results, TestResult{
			Name:   "test_script",
			Passed: false,
			Error:  err.Error(),
		})
	}

	return results, logs
}

// formatConsoleArg renders one console.log/warn/error argument as a string
// for the log line. Strings and other primitives are formatted via their
// native Go string conversion (goja's Value.String() already matches JS's
// own ToString for numbers/booleans/undefined/null); objects and arrays are
// best-effort JSON-stringified (matching how Node/browser devtools show
// structured console arguments) and fall back to Value.String() if that
// fails (e.g. a value JSON can't represent, such as one containing a
// function).
func formatConsoleArg(v goja.Value) string {
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return v.String()
	}
	if obj, ok := v.(*goja.Object); ok {
		if b, err := json.Marshal(obj.Export()); err == nil {
			return string(b)
		}
	}
	return v.String()
}
