// Package tmpl renders "{{var}}" placeholders against a variable map.
package tmpl

import (
	"fmt"
	"regexp"
)

var placeholder = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// Render substitutes every {{var}} in s with vars[var]. It returns an error
// naming the first variable that has no value, so a bad row fails loudly
// instead of sending a request with a literal "{{var}}" in it.
func Render(s string, vars map[string]string) (string, error) {
	var missing string
	result := placeholder.ReplaceAllStringFunc(s, func(match string) string {
		name := placeholder.FindStringSubmatch(match)[1]
		val, ok := vars[name]
		if !ok && missing == "" {
			missing = name
		}
		return val
	})
	if missing != "" {
		return "", fmt.Errorf("undefined variable %q", missing)
	}
	return result, nil
}
