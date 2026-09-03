// Package source loads a source — a directory holding modules/ and templates/
// at its root — from a git repository, a local path, or an embedded fs.FS.
//
// A git source is never read over the network at load time. Fetch populates a
// cache under the loader's cache directory; Git reads that cache and reports
// ErrNotFetched when it is absent, so `agents check` runs offline.
package source

import (
	"errors"
	"fmt"
	"io/fs"
)

// Directory names a source must use at its root.
const (
	ModulesDir   = "modules"
	TemplatesDir = "templates"
)

// Errors reported by the loaders. Each is wrapped with the source's name.
var (
	// ErrNotFetched means a git source is not in the cache; the caller tells
	// the user to run `agents sync`.
	ErrNotFetched = errors.New("source is not in the cache")
	// ErrUnknownRef means the ref names no commit in the source repository.
	ErrUnknownRef = errors.New("unknown ref")
	// ErrNoModules means the directory is not a source: it has no modules/.
	ErrNoModules = errors.New("source has no modules directory")
)

// Source is a loaded source: the modules and templates it holds and the commit
// they came from.
//
// Templates is the templates/ subtree. A source need not have one — git cannot
// track an empty directory, so a source with no seeds carries no templates/ —
// in which case reads from it report fs.ErrNotExist.
type Source struct {
	// Name is the source's name in the manifest, used in errors.
	Name string
	// Modules holds one "<name>.md" per module at its root.
	Modules fs.FS
	// Templates holds the seed files modules declare.
	Templates fs.FS
	// Commit is the full sha a git source was loaded at; empty for a path or
	// embedded source, which have no pin.
	Commit string
}

// FromFS opens a source rooted at fsys — what the binary's embedded example
// source is loaded through.
func FromFS(name string, fsys fs.FS) (Source, error) {
	return open(name, fsys, "")
}

// open splits a source root into its two subtrees, insisting only on modules/.
func open(name string, root fs.FS, commit string) (Source, error) {
	info, err := fs.Stat(root, ModulesDir)
	if err != nil || !info.IsDir() {
		return Source{}, fmt.Errorf("source %q: %w", name, ErrNoModules)
	}
	modules, err := fs.Sub(root, ModulesDir)
	if err != nil {
		return Source{}, fmt.Errorf("source %q: %w", name, err)
	}
	templates, err := fs.Sub(root, TemplatesDir)
	if err != nil {
		return Source{}, fmt.Errorf("source %q: %w", name, err)
	}
	return Source{Name: name, Modules: modules, Templates: templates, Commit: commit}, nil
}
