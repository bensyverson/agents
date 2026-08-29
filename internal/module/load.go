package module

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Extension is the file extension of a module file; the name is the rest.
const Extension = ".md"

// delimiter opens and closes the frontmatter block.
const delimiter = "---"

// Errors returned by Parse. Load wraps them with the offending file name.
var (
	ErrUnterminatedFrontmatter = errors.New("frontmatter is never closed")
	ErrUnknownKind             = errors.New("unknown kind")
	ErrPathRequired            = errors.New("kind: file requires a path")
	ErrPathNotAllowed          = errors.New("path is only valid on kind: file")
	ErrEmptyBody               = errors.New("module has no body")
	ErrSeedPathInvalid         = errors.New("seed path must be relative to the repo")
)

// String renders a Kind as it is written in frontmatter.
func (k Kind) String() string {
	switch k {
	case KindInline:
		return "inline"
	case KindFile:
		return "file"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

// frontmatter is the wire shape of a module's optional YAML header.
type frontmatter struct {
	Kind  string   `yaml:"kind"`
	Path  string   `yaml:"path"`
	Seeds []string `yaml:"seeds"`
}

// Parse splits a module file into its optional leading frontmatter and its
// body, and content-addresses the body.
func Parse(name, raw string) (Module, error) {
	front, body, err := splitFrontmatter(raw)
	if err != nil {
		return Module{}, err
	}
	m, err := parseFrontmatter(front)
	if err != nil {
		return Module{}, err
	}
	if strings.TrimSpace(body) == "" {
		return Module{}, ErrEmptyBody
	}
	// Every rendered region ends on a newline; guarantee it here so the hash
	// covers exactly what is written out.
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	m.Name, m.Body, m.Hash = name, body, Hash(body)
	return m, nil
}

// splitFrontmatter recognises frontmatter only when the very first line is the
// delimiter — a "---" pair further down the file is content (a horizontal rule
// or a YAML sample), not a header.
func splitFrontmatter(raw string) (front, body string, err error) {
	open := delimiter + "\n"
	if !strings.HasPrefix(raw, open) {
		return "", raw, nil
	}
	rest := raw[len(open):]
	for offset := 0; offset <= len(rest); {
		line, next, more := nextLine(rest, offset)
		if line == delimiter {
			return rest[:offset], rest[next:], nil
		}
		if !more {
			break
		}
		offset = next
	}
	return "", "", ErrUnterminatedFrontmatter
}

// nextLine returns the line starting at offset, the offset of the line after
// it, and whether the line was newline-terminated.
func nextLine(s string, offset int) (line string, next int, terminated bool) {
	end := strings.IndexByte(s[offset:], '\n')
	if end < 0 {
		return s[offset:], len(s), false
	}
	return s[offset : offset+end], offset + end + 1, true
}

// parseFrontmatter validates the header and returns the module fields it
// carries; the body fields are the caller's to fill in.
func parseFrontmatter(front string) (Module, error) {
	var fm frontmatter
	if strings.TrimSpace(front) != "" {
		dec := yaml.NewDecoder(strings.NewReader(front))
		dec.KnownFields(true)
		if err := dec.Decode(&fm); err != nil {
			return Module{}, fmt.Errorf("frontmatter: %w", err)
		}
	}

	var kind Kind
	switch fm.Kind {
	case "", KindInline.String():
		kind = KindInline
	case KindFile.String():
		kind = KindFile
	default:
		return Module{}, fmt.Errorf("%w: %q", ErrUnknownKind, fm.Kind)
	}

	switch {
	case kind == KindFile && fm.Path == "":
		return Module{}, ErrPathRequired
	case kind == KindInline && fm.Path != "":
		return Module{}, fmt.Errorf("%w: %q", ErrPathNotAllowed, fm.Path)
	}

	seeds, err := cleanSeeds(fm.Seeds)
	if err != nil {
		return Module{}, err
	}
	return Module{Kind: kind, Path: fm.Path, Seeds: seeds}, nil
}

// cleanSeeds normalises each declared seed and refuses any that would write
// outside the repo — a module is shared code, and a seed is a path it hands to
// every repo that enables it.
func cleanSeeds(seeds []string) ([]string, error) {
	if len(seeds) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		trimmed := strings.TrimSpace(seed)
		clean := path.Clean(trimmed)
		switch {
		case trimmed == "":
			return nil, fmt.Errorf("%w: seed is empty", ErrSeedPathInvalid)
		case path.IsAbs(trimmed) || filepath.IsAbs(trimmed):
			return nil, fmt.Errorf("%w: %q is absolute", ErrSeedPathInvalid, seed)
		case clean == ".." || strings.HasPrefix(clean, "../"):
			return nil, fmt.Errorf("%w: %q escapes the repo", ErrSeedPathInvalid, seed)
		case clean == ".":
			return nil, fmt.Errorf("%w: %q is not a file", ErrSeedPathInvalid, seed)
		}
		out = append(out, clean)
	}
	return out, nil
}

// Set is the modules available to render, keyed by name.
type Set struct {
	byName map[string]Module
}

// Get returns the named module.
func (s Set) Get(name string) (Module, bool) {
	m, ok := s.byName[name]
	return m, ok
}

// Names lists every module in the set, sorted.
func (s Set) Names() []string {
	return slices.Sorted(maps.Keys(s.byName))
}

// Load parses every "*.md" at the root of fsys — the embedded modules or a
// --modules directory. Anything else, including subdirectories, is ignored.
func Load(fsys fs.FS) (Set, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return Set{}, fmt.Errorf("reading modules: %w", err)
	}
	byName := make(map[string]Module, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, Extension) {
			continue
		}
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return Set{}, fmt.Errorf("%s: %w", name, err)
		}
		m, err := Parse(strings.TrimSuffix(name, Extension), string(raw))
		if err != nil {
			return Set{}, fmt.Errorf("%s: %w", name, err)
		}
		byName[m.Name] = m
	}
	return Set{byName: byName}, nil
}

// LoadDir loads the modules in a directory — what `--modules <dir>` passes.
func LoadDir(dir string) (Set, error) {
	set, err := Load(os.DirFS(dir))
	if err != nil {
		return Set{}, fmt.Errorf("%s: %w", dir, err)
	}
	return set, nil
}
