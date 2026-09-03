package repo

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/render"
	"github.com/bensyverson/agents/internal/source"
)

// UpdateOptions is what update takes from its caller.
type UpdateOptions struct {
	// Sources are the sources to move, by manifest name; empty means every
	// git source the manifest names. A source with no pin — a path source, or
	// the source the binary embeds — is skipped whether it is named or not.
	Sources []string
	// Ref is what each source is moved to: a branch, a tag or a sha. Empty
	// means the remote's current default branch.
	Ref string
}

// ModuleChange is one enabled module whose body differs between the commit its
// source was pinned at and the commit it is moving to.
type ModuleChange struct {
	Module string
	// Diff reads from the old body to the new, so a "-" line is what the rules
	// used to say and a "+" line what they will say once this is rendered.
	Diff string
}

// SourceUpdate is one source's move.
type SourceUpdate struct {
	Name string
	// From is what the manifest held: a sha, or the branch or tag a human
	// wrote there by hand.
	From string
	// FromCommit is the commit From named in the cache; empty when the cache
	// held nothing at that ref, which is the one case with no diff to show.
	FromCommit string
	// To is the full sha now written to the manifest.
	To string
	// Changes are the enabled modules whose bodies changed, in manifest order.
	Changes []ModuleChange
}

// Moved reports whether the source's commit changed, as opposed to only the
// spelling of its pin: rewriting `ref: main` to the sha it already named is a
// re-pin (Decision 7), not a new set of rules.
func (u SourceUpdate) Moved() bool { return u.FromCommit != u.To }

// UpdateResult is what an update did.
type UpdateResult struct {
	// Updated is one entry per source whose ref changed, in manifest order.
	// Empty means the manifest was not touched and nothing was rendered.
	Updated []SourceUpdate
	// Sync is the render that followed; the zero value when nothing moved.
	Sync SyncResult
}

// Update moves each selected git source's pin to the commit its ref names now,
// records what changed in the rules, rewrites the manifest and re-renders.
//
// It is the one verb that moves a ref: `sync` fetches a source the cache lacks
// and otherwise renders whatever the manifest pins (Decision 8), so the model's
// instructions never change without this call and the diff it returns.
//
// Nothing moved means nothing is written at all — not the manifest, not a
// rendered file. A repo that is stale for some other reason is `agents sync`'s
// business, not this verb's.
func (r *Repo) Update(opts Options, up UpdateOptions) (UpdateResult, error) {
	var res UpdateResult
	if opts.Loader == nil {
		return res, fmt.Errorf("%s: %w", r.Dir, ErrNoLoader)
	}
	selected, err := selectSources(r.Manifest, up.Sources)
	if err != nil {
		return res, fmt.Errorf("%s: %w", r.Dir, err)
	}

	sources := slices.Clone(r.Manifest.Sources)
	for i, entry := range sources {
		if !selected[entry.Name] || entry.Git == "" {
			continue
		}
		moved, err := r.updateSource(opts.Loader, entry, up.Ref)
		if err != nil {
			return res, fmt.Errorf("%s: %w", r.Dir, err)
		}
		if moved == nil {
			continue
		}
		sources[i].Ref = moved.To
		res.Updated = append(res.Updated, *moved)
	}
	if len(res.Updated) == 0 {
		return res, nil
	}

	m := r.Manifest
	m.Sources = sources
	if err := manifest.Write(filepath.Join(r.Dir, manifest.FileName), m); err != nil {
		return res, err
	}
	// Re-opened rather than re-resolved in place: this repo's loaded sources
	// still hold the trees of the commits it was pinned at, and the render
	// must come from the ones the manifest now names.
	pinned, err := Open(r.Dir, opts)
	if err != nil {
		return res, err
	}
	pinned.Seeds = r.Seeds
	res.Sync, err = pinned.Sync(false)
	return res, err
}

// updateSource moves one source, returning nil when the manifest already reads
// exactly the sha the ref resolves to.
func (r *Repo) updateSource(loader *source.Loader, entry manifest.Source, ref string) (*SourceUpdate, error) {
	// The old tree is read before the fetch, not after: fetching the same ref
	// name overwrites the cache's record of the commit that name pointed at,
	// so a manifest pinned to a branch would lose the body to diff against.
	old, err := loader.Git(entry.Name, entry.Git, entry.Ref)
	if err != nil && !errors.Is(err, source.ErrNotFetched) {
		return nil, err
	}

	wanted := ref
	if wanted == "" {
		// The remote's HEAD, not the cached mirror's: see source.RemoteHeadRef.
		wanted, err = source.RemoteHeadRef(entry.Git)
		if err != nil {
			return nil, err
		}
	}
	sha, err := loader.Fetch(entry.Git, wanted)
	if err != nil {
		return nil, err
	}
	if sha == entry.Ref {
		return nil, nil
	}

	moved := &SourceUpdate{Name: entry.Name, From: entry.Ref, FromCommit: old.Commit, To: sha}
	if !moved.Moved() || old.Commit == "" {
		return moved, nil
	}
	next, err := loader.Git(entry.Name, entry.Git, sha)
	if err != nil {
		return nil, err
	}
	if moved.Changes, err = r.changedModules(entry.Name, old, next); err != nil {
		return nil, err
	}
	return moved, nil
}

// changedModules diffs the bodies of the modules this repo enables from one
// source, between two of its commits. A module the repo does not enable is not
// this repo's business: what update prints is what its own rules will say.
func (r *Repo) changedModules(name string, old, next source.Source) ([]ModuleChange, error) {
	before, err := loadCommit(name, old)
	if err != nil {
		return nil, err
	}
	after, err := loadCommit(name, next)
	if err != nil {
		return nil, err
	}
	var out []ModuleChange
	for _, ref := range r.Manifest.Modules {
		if ref.Source != name {
			continue
		}
		// A module missing from either commit reads as an empty body, so
		// adding or dropping one shows up as the diff it is.
		was, _ := before.Get(ref.Name)
		is, _ := after.Get(ref.Name)
		if was.Body == is.Body {
			continue
		}
		out = append(out, ModuleChange{Module: ref.Name, Diff: render.Diff(was.Body, is.Body)})
	}
	return out, nil
}

// loadCommit reads one commit's modules, naming the commit in any error: an
// unloadable tree is the source's problem, and the sha says which.
func loadCommit(name string, src source.Source) (module.Set, error) {
	set, err := module.Load(src.Modules)
	if err != nil {
		return module.Set{}, fmt.Errorf("source %q at %s: %w", name, src.Commit, err)
	}
	return set, nil
}

// selectSources is the set of source names to visit. No names means every
// source the manifest lists; a name it does not list is the user's typo and
// stops the command, except the embedded source, which is implicit and simply
// has no pin to move.
func selectSources(m manifest.Manifest, names []string) (map[string]bool, error) {
	out := make(map[string]bool, len(m.Sources))
	if len(names) == 0 {
		for _, s := range m.Sources {
			out[s.Name] = true
		}
		return out, nil
	}
	for _, name := range names {
		if _, ok := m.SourceByName(name); !ok {
			if name == manifest.ExampleSource && len(m.Sources) == 0 {
				continue
			}
			return nil, fmt.Errorf("%w: %q", manifest.ErrUnknownSource, name)
		}
		out[name] = true
	}
	return out, nil
}
