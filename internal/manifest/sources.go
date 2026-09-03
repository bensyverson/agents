package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExampleSource is the name of the source the binary embeds. A manifest with
// no sources: block picks every module from it, and it is the only source that
// is never listed: it has neither a git URL nor a path.
const ExampleSource = "example"

// Source is one place modules come from: a git repo or a local directory
// holding modules/ and templates/. Exactly one of Git and Path is set; Ref
// pins a git source to a sha, branch or tag, and whether it is required is the
// fetcher's business, not the manifest's.
type Source struct {
	Name string `yaml:"name"`
	Git  string `yaml:"git,omitempty"`
	Path string `yaml:"path,omitempty"`
	Ref  string `yaml:"ref,omitempty"`
}

// ModuleRef is one manifest entry: a module and the source it comes from. In
// the file an entry is a plain string — `name` for the default source,
// `source/name` for any other — and Parse fills Source in, so a ref that has
// been through Parse always names its source.
type ModuleRef struct {
	Source string
	Name   string
}

// String is the qualified form, `source/name`, and the bare name for a ref
// whose source has not been filled in yet.
func (r ModuleRef) String() string {
	if r.Source == "" {
		return r.Name
	}
	return r.Source + "/" + r.Name
}

// entry is the ref's form in the file: bare when it comes from the default
// source, qualified otherwise.
func (r ModuleRef) entry(defaultSource string) string {
	if r.Source == "" || r.Source == defaultSource {
		return r.Name
	}
	return r.String()
}

// namePattern is the set of names that can appear both as a file name in a
// source's modules/ and as a token in a region marker. Sources share it: a
// source name qualifies a module name in the same string.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ParseRef reads one manifest entry, `name` or `source/name`, and validates
// both halves. The source of a bare name is left empty for the manifest that
// holds it to fill in.
func ParseRef(s string) (ModuleRef, error) {
	ref, err := splitRef(s)
	if err != nil {
		return ModuleRef{}, err
	}
	if ref.Source != "" && !namePattern.MatchString(ref.Source) {
		return ModuleRef{}, fmt.Errorf("%w: %q", ErrInvalidSourceName, ref.Source)
	}
	if !namePattern.MatchString(ref.Name) {
		return ModuleRef{}, fmt.Errorf("%w: %q", ErrInvalidModuleName, ref.Name)
	}
	return ref, nil
}

// splitRef takes a manifest entry apart. It judges neither half — validation
// reports an invalid name, where every other invalid name is reported, rather
// than the YAML decoder — but a qualifier with nothing before the slash is a
// shape it cannot represent: an empty source would read as a bare name and
// silently take the default.
func splitRef(s string) (ModuleRef, error) {
	source, name, ok := strings.Cut(s, "/")
	switch {
	case !ok:
		return ModuleRef{Name: s}, nil
	case source == "":
		return ModuleRef{}, fmt.Errorf("%w: %q names no source before the slash", ErrInvalidSourceName, s)
	}
	return ModuleRef{Source: source, Name: name}, nil
}

// UnmarshalYAML reads the string form of an entry.
func (r *ModuleRef) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("%w: a module entry is `name` or `source/name`", ErrInvalidModuleName)
	}
	ref, err := splitRef(s)
	if err != nil {
		return err
	}
	*r = ref
	return nil
}

// DefaultSource is the source a bare entry comes from: the first source
// listed, or the embedded example source when none are.
func (m Manifest) DefaultSource() string {
	if len(m.Sources) == 0 {
		return ExampleSource
	}
	return m.Sources[0].Name
}

// Ref reads one entry — a command-line argument, say — against this manifest:
// a bare name comes from the default source, and the source a qualified name
// gives must be one the manifest lists. The result is comparable with the
// manifest's own entries, which is what makes "already enabled" answerable.
func (m Manifest) Ref(entry string) (ModuleRef, error) {
	ref, err := ParseRef(entry)
	if err != nil {
		return ModuleRef{}, err
	}
	if ref.Source == "" {
		ref.Source = m.DefaultSource()
	}
	if !m.knows(ref.Source) {
		return ModuleRef{}, fmt.Errorf("%w: %q names source %q", ErrUnknownSource, ref, ref.Source)
	}
	return ref, nil
}

// SourceByName finds a listed source. The embedded example source is implicit
// and never listed, so it is not found here.
func (m Manifest) SourceByName(name string) (Source, bool) {
	for _, s := range m.Sources {
		if s.Name == name {
			return s, true
		}
	}
	return Source{}, false
}

// Entries renders the manifest's modules in the form they take in the file:
// bare for the default source, qualified for any other.
func (m Manifest) Entries() []string {
	def := m.DefaultSource()
	out := make([]string, len(m.Modules))
	for i, ref := range m.Modules {
		out[i] = ref.entry(def)
	}
	return out
}

// knows reports whether a module entry may name this source.
func (m Manifest) knows(source string) bool {
	if len(m.Sources) == 0 {
		return source == ExampleSource
	}
	_, ok := m.SourceByName(source)
	return ok
}

func (m Manifest) validateSources() error {
	seen := make(map[string]bool, len(m.Sources))
	for _, s := range m.Sources {
		if !namePattern.MatchString(s.Name) {
			return fmt.Errorf("%w: %q", ErrInvalidSourceName, s.Name)
		}
		if seen[s.Name] {
			return fmt.Errorf("%w: %q", ErrDuplicateSource, s.Name)
		}
		seen[s.Name] = true
		if (s.Git == "") == (s.Path == "") {
			if s.Name == ExampleSource && s.Git == "" && s.Path == "" {
				return fmt.Errorf("%w: source %q is the source the binary embeds; leave it out of %s rather than listing it", ErrSourceLocation, s.Name, FileName)
			}
			return fmt.Errorf("%w: source %q needs exactly one of git and path", ErrSourceLocation, s.Name)
		}
	}
	return nil
}
