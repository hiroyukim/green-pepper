package model

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RequestSpec is a single HTTP request template. Fields may contain
// "{{var}}" placeholders that get substituted per CSV row / env values.
type RequestSpec struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	// TestScript is a JavaScript snippet (executed with goja, see
	// internal/runner's runTestScript) run against the response after the
	// request completes, to verify it (via a pm.test(...)-style API). Unlike
	// Method/URL/Headers/Body, it is deliberately NOT run through
	// tmpl.Render/"{{var}}" substitution — it reads variables via the
	// pm.variables.get(name) API instead. Empty (the common case) means no
	// test script runs at all.
	TestScript string `yaml:"test_script,omitempty"`
}

// LoadRequest reads and parses a request template YAML file.
func LoadRequest(path string) (*RequestSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading request file: %w", err)
	}

	var spec RequestSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing request file: %w", err)
	}
	if spec.Method == "" {
		spec.Method = "GET"
	}
	if spec.URL == "" {
		return nil, fmt.Errorf("request file %s: url is required", path)
	}
	return &spec, nil
}

// LoadEnv reads a flat key/value YAML file used as default template variables.
func LoadEnv(path string) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading env file: %w", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing env file: %w", err)
	}

	vars := make(map[string]string, len(raw))
	for k, v := range raw {
		vars[k] = fmt.Sprint(v)
	}
	return vars, nil
}
