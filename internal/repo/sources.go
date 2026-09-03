package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/source"
)

// FetchPolicy says whether opening a repo may populate the source cache.
//
// It is the whole of Decision 8 in one value: `sync`, `add` and `init` fetch a
// source the cache lacks and nothing else; every other verb runs offline, so a
// pre-commit `agents check` never reaches the network and never depends on it.
type FetchPolicy int

const (
	// FetchNever runs no git at all. A git source the cache lacks is
	// ErrNotFetched, which the caller turns into "run `agents sync`".
	FetchNever FetchPolicy = iota
	// FetchMissing fetches a git source the cache lacks, and only then: a ref
	// already cached is never moved, which is `agents update`'s business.
	FetchMissing
)

// Options is what opening a repo needs beyond its directory.
type Options struct {
	// Loader loads and caches the git and path sources a manifest names. A
	// manifest that names none needs no loader.
	Loader *source.Loader
	// Embedded is the source the binary carries: modules/ and templates/ at
	// its root. A manifest with no sources: block picks every module from it,
	// under the name manifest.ExampleSource.
	Embedded fs.FS
	// Fetch says whether a git source missing from the cache may be fetched.
	Fetch FetchPolicy
}

// Sourced is one loaded source and the modules it holds.
type Sourced struct {
	Source source.Source
	Set    module.Set
}

// Sources are a repo's loaded sources, keyed by the name its manifest gives
// each. The embedded source's key is manifest.ExampleSource.
type Sources map[string]Sourced

// Failures loading the sources a manifest names.
var (
	// ErrNoEmbeddedSource means a manifest named no sources and the caller
	// supplied no embedded source to fall back on.
	ErrNoEmbeddedSource = errors.New("no embedded source")
	// ErrNoLoader means a manifest named a git or path source and the caller
	// supplied no loader.
	ErrNoLoader = errors.New("no source loader")
)

// LoadSources loads every source m names, or the embedded one when it names
// none. dir is the repo the manifest belongs to: a relative path source is
// resolved against it, so a manifest can point at a sibling checkout.
func LoadSources(dir string, m manifest.Manifest, opts Options) (Sources, error) {
	if len(m.Sources) == 0 {
		s, err := LoadSource(manifest.ExampleSource, opts.Embedded)
		if err != nil {
			return nil, err
		}
		return Sources{manifest.ExampleSource: s}, nil
	}
	out := make(Sources, len(m.Sources))
	for _, entry := range m.Sources {
		s, err := loadEntry(dir, entry, opts)
		if err != nil {
			return nil, err
		}
		out[entry.Name] = s
	}
	return out, nil
}

// LoadSource opens a source held in an fs.FS and loads its modules — how the
// source the binary embeds is read.
func LoadSource(name string, fsys fs.FS) (Sourced, error) {
	if fsys == nil {
		return Sourced{}, fmt.Errorf("%w for %q", ErrNoEmbeddedSource, name)
	}
	src, err := source.FromFS(name, fsys)
	if err != nil {
		return Sourced{}, err
	}
	return withModules(src)
}

// loadEntry loads one manifest source. A git source is read from the cache;
// only FetchMissing may populate it, and only when it is not there at all.
func loadEntry(dir string, entry manifest.Source, opts Options) (Sourced, error) {
	if opts.Loader == nil {
		return Sourced{}, fmt.Errorf("%w for source %q", ErrNoLoader, entry.Name)
	}
	if entry.Path != "" {
		src, err := opts.Loader.Path(entry.Name, resolvePath(dir, entry.Path))
		if err != nil {
			return Sourced{}, err
		}
		return withModules(src)
	}

	src, err := opts.Loader.Git(entry.Name, entry.Git, entry.Ref)
	if errors.Is(err, source.ErrNotFetched) && opts.Fetch == FetchMissing {
		if _, err := opts.Loader.Fetch(entry.Git, entry.Ref); err != nil {
			return Sourced{}, err
		}
		src, err = opts.Loader.Git(entry.Name, entry.Git, entry.Ref)
		if err != nil {
			return Sourced{}, err
		}
	} else if errors.Is(err, source.ErrNotFetched) {
		// One line, naming the source and the verb that fixes it: this is what
		// a pre-commit `agents check` prints on a machine that has never
		// fetched, and it must not read like a network failure.
		return Sourced{}, fmt.Errorf("%w: %q at %q — run `agents sync`", source.ErrNotFetched, entry.Name, entry.Ref)
	} else if err != nil {
		return Sourced{}, err
	}
	return withModules(src)
}

// withModules pairs a source with the modules it holds.
func withModules(src source.Source) (Sourced, error) {
	set, err := module.Load(src.Modules)
	if err != nil {
		return Sourced{}, fmt.Errorf("source %q: %w", src.Name, err)
	}
	return Sourced{Source: src, Set: set}, nil
}

// resolvePath places a manifest's path source: relative to the repo that names
// it, so the same manifest works from any working directory.
func resolvePath(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, filepath.FromSlash(p))
}

// module finds the module a manifest entry names, in the source it names.
func (s Sources) module(ref manifest.ModuleRef) (module.Module, bool) {
	src, ok := s[ref.Source]
	if !ok {
		return module.Module{}, false
	}
	return src.Set.Get(ref.Name)
}

// resolve turns a manifest into the modules it enables, in order, and a record
// of the source each came from. A module's name is enough to key that record:
// the manifest refuses two enabled modules that render to the same name.
func (s Sources) resolve(m manifest.Manifest) ([]module.Module, map[string]string, error) {
	origin := make(map[string]string, len(m.Modules))
	mods, err := manifest.ResolveWith(m, func(ref manifest.ModuleRef) (module.Module, bool) {
		mod, ok := s.module(ref)
		if ok {
			origin[ref.Name] = ref.Source
		}
		return mod, ok
	})
	if err != nil {
		return nil, nil, err
	}
	return mods, origin, nil
}
