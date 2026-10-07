package repo

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// explainEmbedded turns the entries a manifest with sources: lists but no
// source supplies into the explanation they deserve when the binary's embedded
// example source does supply them: the manifest was moved to real sources and
// still names the demo modules. Any other error comes back unchanged.
func explainEmbedded(err error, m manifest.Manifest, opts Options) error {
	var unknown *manifest.UnknownModulesError
	if !errors.As(err, &unknown) || len(m.Sources) == 0 || opts.Embedded == nil {
		return err
	}
	embedded, loadErr := LoadSource(manifest.ExampleSource, opts.Embedded)
	if loadErr != nil {
		return err
	}
	out := &manifest.UnknownModulesError{DefaultSource: m.DefaultSource()}
	for _, ref := range unknown.Refs {
		if _, ok := embedded.Set.Get(ref.Name); ok {
			out.Embedded = append(out.Embedded, ref)
			continue
		}
		out.Refs = append(out.Refs, ref)
	}
	if len(out.Embedded) == 0 {
		return err
	}
	return out
}

// OpenForRemove opens a repo for `agents remove`, which must work on the
// manifest an unknown-module error tells the user to fix: a module cannot be
// required to resolve in order to be removed. Entries no source supplies stay
// in the manifest, unresolved, until Remove drops them.
//
// The binary's embedded modules join the sources as a fallback, so removing
// one that was a kind:file module still finds the file it rendered.
func OpenForRemove(dir string, opts Options) (*Repo, error) {
	r, err := Open(dir, opts)
	if _, ok := errors.AsType[*manifest.UnknownModulesError](err); !ok {
		return r, err
	}
	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	if err != nil {
		return nil, err
	}
	sources, err := LoadSources(dir, m, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	if _, taken := sources[manifest.ExampleSource]; !taken {
		if embedded, err := LoadSource(manifest.ExampleSource, opts.Embedded); err == nil {
			sources[manifest.ExampleSource] = embedded
		}
	}
	mods, origin := sources.resolveAvailable(m)
	return &Repo{Dir: dir, Manifest: m, Sources: sources, mods: mods, origin: origin}, nil
}

// resolveAvailable is resolve for a manifest some of whose entries no source
// supplies: those are left out rather than refused.
func (s Sources) resolveAvailable(m manifest.Manifest) ([]module.Module, map[string]string) {
	origin := make(map[string]string, len(m.Modules))
	var mods []module.Module
	for _, ref := range m.Modules {
		if mod, ok := s.module(ref); ok {
			mods = append(mods, mod)
			origin[ref.Name] = ref.Source
		}
	}
	return mods, origin
}

// removalModule is the module whose region a removal deletes. A listed entry
// that no source supplies falls back to the embedded module of that name, and
// failing that to an inline module known only by name — the region is found by
// its marker either way.
func (r *Repo) removalModule(ref manifest.ModuleRef) module.Module {
	if mod, ok := r.Sources.module(ref); ok {
		return mod
	}
	if embedded, ok := r.Sources[manifest.ExampleSource]; ok {
		if mod, ok := embedded.Set.Get(ref.Name); ok {
			return mod
		}
	}
	return module.Module{Name: ref.Name}
}
