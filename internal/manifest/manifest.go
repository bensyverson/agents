// Package manifest reads and writes .agents.yaml, the per-repo list of the
// sources modules come from and the modules to render from them, in order.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
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
	// Sources are the places modules come from, the first being the default.
	// Empty means the one source the binary embeds.
	Sources []Source `yaml:"sources,omitempty"`
	// Modules are the enabled modules, in render order.
	Modules []ModuleRef `yaml:"modules"`
}

// manifestYAML is the manifest as the file holds it: a module entry is a
// string there, `name` or `source/name`, not a pair.
type manifestYAML struct {
	Sources []Source `yaml:"sources,omitempty"`
	Modules []string `yaml:"modules"`
}

// Validation failures. Each is wrapped with the offending name where there is one.
var (
	ErrNoModules         = errors.New("no modules listed")
	ErrDuplicateModule   = errors.New("duplicate module")
	ErrInvalidModuleName = errors.New("invalid module name")
	// ErrModuleCollision means two sources supply a module of the same name.
	// A rendered path and a region marker carry the module's name alone, so
	// there is nothing to tell the two regions apart: the manifest refuses.
	ErrModuleCollision = errors.New("two sources supply the same module")
	// ErrUnknownSource means a qualified entry names a source the manifest
	// does not list.
	ErrUnknownSource     = errors.New("unknown source")
	ErrInvalidSourceName = errors.New("invalid source name")
	ErrDuplicateSource   = errors.New("duplicate source")
	// ErrSourceLocation means a source set both git and path, or neither.
	ErrSourceLocation = errors.New("a source is either git or path")
)

// Lookup finds the module a manifest entry names. It is how a caller holding
// one module set per source answers for all of them at once.
type Lookup func(ModuleRef) (module.Module, bool)

// UnknownModulesError reports every manifest entry with no matching module, so
// a typo-ridden manifest takes one round trip to fix rather than several.
type UnknownModulesError struct {
	// Refs are the unknown entries, in manifest order.
	Refs []ModuleRef
}

func (e *UnknownModulesError) Error() string {
	names := make([]string, len(e.Refs))
	for i, ref := range e.Refs {
		names[i] = ref.String()
	}
	return fmt.Sprintf("unknown modules in %s: %s", FileName, strings.Join(names, ", "))
}

// Parse reads a manifest and validates it. Every entry comes back with its
// source filled in, whether the file named one or not.
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
	m = m.withDefaults()
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Marshal renders a manifest in canonical form. An invalid manifest is an
// error rather than a file someone has to clean up later.
func Marshal(m Manifest) ([]byte, error) {
	m = m.withDefaults()
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

// MarshalYAML writes each entry in the form it takes in the file.
func (m Manifest) MarshalYAML() (any, error) {
	return manifestYAML{Sources: m.Sources, Modules: m.Entries()}, nil
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

// ResolveWith looks each entry up through lookup, in manifest order.
func ResolveWith(m Manifest, lookup Lookup) ([]module.Module, error) {
	m = m.withDefaults()
	resolved := make([]module.Module, 0, len(m.Modules))
	var unknown []ModuleRef
	for _, ref := range m.Modules {
		mod, ok := lookup(ref)
		if !ok {
			unknown = append(unknown, ref)
			continue
		}
		resolved = append(resolved, mod)
	}
	if len(unknown) > 0 {
		return nil, &UnknownModulesError{Refs: unknown}
	}
	return resolved, nil
}

// Resolve looks the manifest's modules up in one set, whatever source each
// entry names. Two enabled modules cannot share a name, so a single set
// answers unambiguously; a caller with a set per source uses ResolveWith.
func Resolve(m Manifest, set module.Set) ([]module.Module, error) {
	return ResolveWith(m, func(ref ModuleRef) (module.Module, bool) {
		return set.Get(ref.Name)
	})
}

// withDefaults is the manifest with every bare entry attributed to the default
// source. It copies the entries rather than filling them in place: a caller's
// manifest is its own.
func (m Manifest) withDefaults() Manifest {
	def := m.DefaultSource()
	out := m
	out.Modules = slices.Clone(m.Modules)
	for i := range out.Modules {
		if out.Modules[i].Source == "" {
			out.Modules[i].Source = def
		}
	}
	return out
}

// validate assumes every entry names its source; Parse and Marshal both fill
// them in first.
func (m Manifest) validate() error {
	if len(m.Modules) == 0 {
		return ErrNoModules
	}
	if err := m.validateSources(); err != nil {
		return err
	}
	seen := make(map[ModuleRef]bool, len(m.Modules))
	byName := make(map[string]ModuleRef, len(m.Modules))
	for _, ref := range m.Modules {
		if !namePattern.MatchString(ref.Name) {
			return fmt.Errorf("%w: %q", ErrInvalidModuleName, ref.Name)
		}
		if !m.knows(ref.Source) {
			return fmt.Errorf("%w: %q names source %q", ErrUnknownSource, ref, ref.Source)
		}
		if seen[ref] {
			return fmt.Errorf("%w: %q", ErrDuplicateModule, ref)
		}
		if other, ok := byName[ref.Name]; ok {
			return fmt.Errorf("%w: %s and %s both render as %q", ErrModuleCollision, other, ref, ref.Name)
		}
		seen[ref] = true
		byName[ref.Name] = ref
	}
	return nil
}
