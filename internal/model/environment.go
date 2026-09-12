package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// EnvDir holds multiple named environments loaded from a directory passed to
// --env: one per *.yaml/*.yml file found directly inside it (no recursion).
// The file name without its extension is the environment name.
type EnvDir struct {
	// Dir is the directory the environments were loaded from.
	Dir string
	// Names is the sorted list of environment names available.
	Names []string
	// Files maps environment name -> on-disk file name (with extension).
	Files map[string]string
	// Envs maps environment name -> variables.
	Envs map[string]map[string]string
}

// IsEnvDir reports whether path exists and is a directory (as opposed to a
// single env file). An empty path returns (false, nil).
func IsEnvDir(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("checking env path: %w", err)
	}
	return info.IsDir(), nil
}

// LoadEnvDir reads every *.yaml/*.yml file directly inside dir (no
// recursion) as a named environment.
func LoadEnvDir(dir string) (*EnvDir, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading env directory: %w", err)
	}

	files := map[string]string{}
	envs := map[string]map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ext)
		vars, err := LoadEnv(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("loading env %q: %w", entry.Name(), err)
		}
		files[name] = entry.Name()
		envs[name] = vars
	}
	if len(envs) == 0 {
		return nil, fmt.Errorf("no *.yaml/*.yml environment files found in %s", dir)
	}

	names := make([]string, 0, len(envs))
	for name := range envs {
		names = append(names, name)
	}
	sort.Strings(names)

	return &EnvDir{Dir: dir, Names: names, Files: files, Envs: envs}, nil
}

// SaveEnv writes vars as a flat key/value YAML file (mirroring LoadEnv's
// format) to filename inside dir. filename is expected to be one of the
// on-disk names produced by LoadEnvDir (e.g. from EnvDir.Files), but the
// resulting path is validated to stay inside dir regardless.
func SaveEnv(dir, filename string, vars map[string]string) error {
	path := filepath.Join(dir, filename)

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving env directory: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving env file path: %w", err)
	}
	if absPath != absDir && !strings.HasPrefix(absPath, absDir+string(os.PathSeparator)) {
		return fmt.Errorf("invalid environment file path %q", filename)
	}

	out, err := yaml.Marshal(vars)
	if err != nil {
		return fmt.Errorf("marshaling env: %w", err)
	}
	if err := os.WriteFile(absPath, out, 0o644); err != nil {
		return fmt.Errorf("writing env file: %w", err)
	}
	return nil
}
