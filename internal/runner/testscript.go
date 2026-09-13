package runner

import (
	"encoding/json"
	"fmt"
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
// request's response, returning the pm.test(...) results it recorded. It
// returns nil immediately when script is empty, without constructing a goja
// runtime at all — the fast path for the overwhelming majority of requests
// that don't use this feature.
//
// The script gets a "pm" object modeled loosely on Postman's test-script API:
//
//   - pm.response.code (int), pm.response.status (string)
//   - pm.response.body (string, the raw response body)
//   - pm.response.json() — JSON.parse of the body; throws on invalid JSON
//   - pm.variables.get(name) — looks up vars, "" when absent
//   - pm.test(name, fn) — runs fn; an exception means a failed test, a
//     normal return means a passed one; either way it's recorded under name
//     and does not stop the rest of the script from running
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
func runTestScript(script string, statusCode int, status string, body []byte, vars map[string]string) (results []TestResult) {
	if script == "" {
		return nil
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

	vm := goja.New()

	responseObj := vm.NewObject()
	_ = responseObj.Set("code", statusCode)
	_ = responseObj.Set("status", status)
	_ = responseObj.Set("body", string(body))
	_ = responseObj.Set("json", func() (interface{}, error) {
		var parsed interface{}
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

	if err := vm.Set("pm", pmObj); err != nil {
		results = append(results, TestResult{Name: "test_script", Passed: false, Error: err.Error()})
		return results
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

	return results
}
