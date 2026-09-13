package model

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// collectionNamePattern is the allow-list for a collection entry name: it
// becomes part of a filesystem path built from user-supplied web form input,
// so only a conservative character set is accepted.
var collectionNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// sanitizeCollectionName validates name against a strict allow-list before it
// is used to build a path under a collection directory, to guard against
// path traversal (e.g. "../../etc/passwd") and absolute-path escapes. It
// returns the cleaned name (equal to name when valid) or a clear error.
func sanitizeCollectionName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("request name is required")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("invalid request name %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("request name %q must not contain a path separator", name)
	}
	if !collectionNamePattern.MatchString(name) {
		return "", fmt.Errorf("request name %q contains unsupported characters (allowed: letters, digits, '-', '_', '.')", name)
	}
	// filepath.Base as a defense-in-depth check: for a name that already
	// passed the checks above, this should be a no-op.
	base := filepath.Base(name)
	if base != name {
		return "", fmt.Errorf("invalid request name %q", name)
	}
	return name, nil
}

// requestFileExts are the extensions recognized as request files within a
// collection directory, tried in order when loading a named entry.
var requestFileExts = []string{".yaml", ".yml"}

// ListCollection returns the sorted, de-duplicated display names of the
// requests found directly inside dir (a flat collection directory of
// "*.yaml"/"*.yml" files, one RequestSpec each; nested directories are
// ignored — nested collections are out of scope).
func ListCollection(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading collection directory: %w", err)
	}

	seen := map[string]bool{}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ext)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// LoadFromCollection loads the named request from the collection directory
// dir. name is sanitized first, then resolved against the recognized request
// file extensions.
func LoadFromCollection(dir, name string) (*RequestSpec, error) {
	safeName, err := sanitizeCollectionName(name)
	if err != nil {
		return nil, err
	}

	for _, ext := range requestFileExts {
		path := filepath.Join(dir, safeName+ext)
		if _, statErr := os.Stat(path); statErr == nil {
			return LoadRequest(path)
		}
	}
	return nil, fmt.Errorf("request %q not found in collection", name)
}

// NamedRequestSpec pairs a loaded RequestSpec with its collection entry name.
type NamedRequestSpec struct {
	Name string
	Spec *RequestSpec
}

// LoadAllFromCollection returns every request in the collection directory
// dir, loaded and ordered by ListCollection's deterministic (sorted) name
// order. It returns an error if dir has no requests, so callers don't need a
// separate empty-collection check.
func LoadAllFromCollection(dir string) ([]NamedRequestSpec, error) {
	names, err := ListCollection(dir)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("collection %q has no requests", dir)
	}

	specs := make([]NamedRequestSpec, 0, len(names))
	for _, name := range names {
		spec, err := LoadFromCollection(dir, name)
		if err != nil {
			return nil, err
		}
		specs = append(specs, NamedRequestSpec{Name: name, Spec: spec})
	}
	return specs, nil
}

// SaveToCollection marshals spec as YAML (matching the format produced by the
// "YAMLをダウンロード" download path) and writes it into dir under name+".yaml",
// creating or overwriting the file. name is sanitized first to prevent path
// traversal, since it originates from a web form field.
func SaveToCollection(dir, name string, spec RequestSpec) error {
	safeName, err := sanitizeCollectionName(name)
	if err != nil {
		return err
	}

	out, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	path := filepath.Join(dir, safeName+".yaml")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing request file: %w", err)
	}
	return nil
}
