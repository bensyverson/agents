// Package manifest reads and writes .agents.yaml, the per-repo list of
// modules to render, in order.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bensyverson/agents/internal/module"
)

// FileName is the manifest's name in a managed repo.
const FileName = ".agents.yaml"

// indent keeps the rendered manifest at two spaces, matching the files
// already checked in (yaml.v3 defaults to four).
const indent = 2

// Manifest is the contents of .agents.yaml.
type Manifest struct {
	Modules []string `yaml:"modules"`
}

// Validation failures. Each is wrapped with the offending name where there is one.
var (
	ErrNoModules         = errors.New("no modules listed")
	ErrDuplicateModule   = errors.New("duplicate module")
	ErrInvalidModuleName = errors.New("invalid module name")
)

// moduleNamePattern is the set of names that can appear both as a file name in
// modules/ and as a token in a region marker.
var moduleNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// UnknownModulesError reports every manifest entry with no matching module, so
// a typo-ridden manifest takes one round trip to fix rather than several.
type UnknownModulesError struct {
	// Names are the unknown module names, in manifest order.
	Names []string
}

func (e *UnknownModulesError) Error() string {
	return fmt.Sprintf("unknown modules in %s: %s", FileName, strings.Join(e.Names, ", "))
}

// Parse reads a manifest and validates it.
func Parse(data []byte) (Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return Manifest{}, ErrNoModules
		}
		return Manifest{}, fmt.Errorf("parsing %s: %w", FileName, err)
	}
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Marshal renders a manifest in canonical form. An invalid manifest is an
// error rather than a file someone has to clean up later.
func Marshal(m Manifest) ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indent)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("encoding %s: %w", FileName, err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encoding %s: %w", FileName, err)
	}
	return buf.Bytes(), nil
}

// Read parses the manifest at path.
func Read(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}
	m, err := Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Write renders the manifest to path.
func Write(path string, m Manifest) error {
	data, err := Marshal(m)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Resolve looks the manifest's modules up in set, in manifest order.
func Resolve(m Manifest, set module.Set) ([]module.Module, error) {
	resolved := make([]module.Module, 0, len(m.Modules))
	var unknown []string
	for _, name := range m.Modules {
		mod, ok := set.Get(name)
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		resolved = append(resolved, mod)
	}
	if len(unknown) > 0 {
		return nil, &UnknownModulesError{Names: unknown}
	}
	return resolved, nil
}

func (m Manifest) validate() error {
	if len(m.Modules) == 0 {
		return ErrNoModules
	}
	seen := make(map[string]bool, len(m.Modules))
	for _, name := range m.Modules {
		if !moduleNamePattern.MatchString(name) {
			return fmt.Errorf("%w: %q", ErrInvalidModuleName, name)
		}
		if seen[name] {
			return fmt.Errorf("%w: %q", ErrDuplicateModule, name)
		}
		seen[name] = true
	}
	return nil
}
