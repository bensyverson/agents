// Package repo performs the file-level work on a managed repo — a directory
// holding a .agents.yaml — on top of the module, manifest, render, gotchas and
// registry packages. It is a library: it never prints and never exits, so the
// CLI owns every message and status code.
package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/render"
)

// The files a managed repo owns, all repo-relative.
const (
	// AgentsFile holds every inline module, between markers.
	AgentsFile = "AGENTS.md"
	// ClaudeFile is a symlink to AgentsFile, for harnesses that look for it.
	ClaudeFile = "CLAUDE.md"
	// GotchasFile is the project's own trap list, read by `status` and `diff`.
	// Which modules seed it is their business, not the tool's: see seed.go.
	GotchasFile = "project/gotchas.md"
	// HeadTemplate is the starting head for a brand-new AgentsFile, in the
	// templates FS. The head is init's special case, not a module's seed.
	HeadTemplate = "head.md"
)

// Failures a caller distinguishes: `status --all` reports them per repo rather
// than aborting the walk.
var (
	ErrMissingDir = errors.New("directory does not exist")
	ErrNotManaged = errors.New("not a managed repo")
)

// Repo is an open managed repo: its manifest, the sources it names, and the
// modules those sources supply.
type Repo struct {
	Dir      string
	Manifest manifest.Manifest
	// Sources are the loaded sources, keyed by the name the manifest gives
	// each. A module's seeds are read from the templates of the source that
	// module came from, so two sources may seed different bodies for one path.
	Sources Sources
	// Seeds says whether Sync creates the files the modules declare. Open
	// leaves it at SeedsSkipped; the verbs that may create files set it.
	Seeds SeedPolicy

	// mods is the manifest resolved to modules, in manifest order.
	mods []module.Module
	// origin maps an enabled module's name to the source it came from.
	origin map[string]string
}

// Open reads dir's manifest, loads the sources it names and resolves it
// against them. It does not read any rendered file; that is Targets' job.
func Open(dir string, opts Options) (*Repo, error) {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("%s: %w", dir, ErrMissingDir)
	case err != nil:
		return nil, fmt.Errorf("%s: %w", dir, err)
	case !info.IsDir():
		return nil, fmt.Errorf("%s: %w (not a directory)", dir, ErrMissingDir)
	}

	path := filepath.Join(dir, manifest.FileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w (no %s)", dir, ErrNotManaged, manifest.FileName)
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	m, err := manifest.Read(path)
	if err != nil {
		return nil, err
	}
	sources, err := LoadSources(dir, m, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	mods, origin, err := sources.resolve(m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	return &Repo{Dir: dir, Manifest: m, Sources: sources, mods: mods, origin: origin}, nil
}

// templatesFor is the templates of the source the named enabled module came
// from — where its seeds are read, and nothing else.
func (r *Repo) templatesFor(name string) fs.FS {
	src, ok := r.Sources[r.origin[name]]
	if !ok {
		return nil
	}
	return src.Source.Templates
}

// Target is one file the tool renders: AGENTS.md, holding every inline module
// of the manifest in order, or a kind:file module's own path.
type Target struct {
	// Path is repo-relative, with forward slashes.
	Path string
	// Modules are the modules this file holds, in manifest order.
	Modules []module.Module
	// Exists reports whether the file is on disk; an absent file reads as "".
	Exists bool
	// Current is the file as it stands, Rendered what sync would write.
	Current  string
	Rendered string
	// Report classifies the current file's regions against Modules.
	Report render.Report
}

// NeedsWrite reports whether syncing this target would change the file. It is
// the only staleness test that matters to sync: rendering is deterministic, so
// "the render differs" covers stale markers, missing regions and orphans alike.
func (t Target) NeedsWrite() bool { return t.Rendered != t.Current }

// moduleBody is the current body of the named module, or "" for an orphan
// region whose module has left the manifest.
func (t Target) moduleBody(name string) string {
	for _, m := range t.Modules {
		if m.Name == name {
			return m.Body
		}
	}
	return ""
}

// Targets returns AGENTS.md followed by one target per kind:file module, each
// read from disk, rendered and inspected.
func (r *Repo) Targets() ([]Target, error) {
	var inline, files []module.Module
	for _, m := range r.mods {
		if m.Kind == module.KindFile {
			files = append(files, m)
			continue
		}
		inline = append(inline, m)
	}
	// The index is a module in every respect but where it comes from, and it
	// goes last, so it renders after the last inline region and is checked,
	// diffed and removed by the machinery every other region already uses.
	if idx, ok := render.Index(r.mods); ok {
		inline = append(inline, idx)
	}

	targets := make([]Target, 0, 1+len(files))
	t, err := r.target(AgentsFile, inline)
	if err != nil {
		return nil, err
	}
	targets = append(targets, t)
	for _, m := range files {
		t, err := r.target(filepath.ToSlash(filepath.Clean(m.Path)), []module.Module{m})
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func (r *Repo) target(rel string, mods []module.Module) (Target, error) {
	current, exists, err := readIfPresent(filepath.Join(r.Dir, rel))
	if err != nil {
		return Target{}, err
	}
	rendered, err := render.Render(current, mods)
	if err != nil {
		return Target{}, fmt.Errorf("%s: %w", rel, err)
	}
	report, err := render.Inspect(current, mods)
	if err != nil {
		return Target{}, fmt.Errorf("%s: %w", rel, err)
	}
	return Target{
		Path:     rel,
		Modules:  mods,
		Exists:   exists,
		Current:  current,
		Rendered: rendered,
		Report:   report,
	}, nil
}

// readIfPresent reads path, treating a missing file as empty content rather
// than an error: an unrendered repo is the normal case, not a failure.
func readIfPresent(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading %s: %w", path, err)
	}
	return string(b), true, nil
}
