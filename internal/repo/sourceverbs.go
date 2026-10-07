package repo

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/source"
)

// ErrSourceInUse means a source cannot be removed because the manifest enables
// modules that come from it. Removing rules is `agents remove`'s decision, not
// a side effect of dropping the place they came from.
var ErrSourceInUse = errors.New("source has enabled modules")

// pinSource resolves a git source's ref against the remote, fetches that
// commit into the cache and records its sha in s. It is how a pin is chosen by
// init and by `source add` alike: asking the remote, never the cache, whose
// record of a branch is only the commit it last fetched. A source that is
// already pinned to a full sha, and a path source, are left as they are.
func pinSource(loader *source.Loader, s *manifest.Source) error {
	if s.Git == "" || source.IsFullSha(s.Ref) {
		return nil
	}
	if loader == nil {
		return fmt.Errorf("%w for source %q", ErrNoLoader, s.Name)
	}
	sha, err := pinRemote(loader, s.Git, s.Ref)
	if err != nil {
		return err
	}
	s.Ref = sha
	return nil
}

// AddSourceResult is what AddSource did.
type AddSourceResult struct {
	// Source is the entry written, with a git ref resolved to its sha.
	Source manifest.Source
	// Removed are the example modules the swap took out, in manifest order;
	// empty unless the manifest named no sources before.
	Removed []string
	// Regions is one entry per removed module, in the same order.
	Regions []RemovedRegion
	// Enabled are the defaults the new source declares, enabled in place of
	// the examples; empty unless the swap happened and the source declares
	// some. Sync is the render that enabled them.
	Enabled []string
	Sync    SyncResult
}

// AddSource appends a source to the manifest and enables no module from it.
//
// A manifest with no sources renders from the example modules the binary
// embeds, which the first real source replaces: they are removed, regions and
// files exactly as Remove would take them, and the result names them — and
// the defaults the new source declares are enabled in their place, so the repo
// is not left with nothing to render. A source added beside others enables
// nothing: that repo has already chosen its modules. The
// manifest is written last, after the source has loaded and the new manifest
// has validated, so a source that cannot be reached or a name already in use
// changes nothing.
func (r *Repo) AddSource(src manifest.Source, opts Options) (AddSourceResult, error) {
	var res AddSourceResult
	if _, taken := r.Manifest.SourceByName(src.Name); taken {
		return res, fmt.Errorf("%w: %q", manifest.ErrDuplicateSource, src.Name)
	}
	if err := pinSource(opts.Loader, &src); err != nil {
		return res, fmt.Errorf("%s: %w", r.Dir, err)
	}
	res.Source = src

	next := manifest.Manifest{
		Sources: append(slices.Clone(r.Manifest.Sources), src),
		Modules: r.Manifest.Modules,
	}
	swap := len(r.Manifest.Sources) == 0
	if swap {
		next.Modules = nil
	}
	if _, err := manifest.Marshal(next); err != nil {
		return res, err
	}

	loaded, err := LoadSources(r.Dir, manifest.Manifest{Sources: []manifest.Source{src}}, opts)
	if err != nil {
		return res, fmt.Errorf("%s: %w", r.Dir, err)
	}
	sources := loaded
	if !swap {
		sources = maps.Clone(r.Sources)
		maps.Copy(sources, loaded)
	}

	after := &Repo{Dir: r.Dir, Manifest: next, Sources: sources, Seeds: r.Seeds, mods: r.mods, origin: r.origin}
	if swap {
		after.mods, after.origin = nil, map[string]string{}
		for _, mod := range r.mods {
			removed, err := r.removeRegion(mod)
			res.Removed = append(res.Removed, mod.Name)
			res.Regions = append(res.Regions, removed)
			if err != nil {
				return res, err
			}
		}
		if err := after.updateIndex(); err != nil {
			return res, err
		}
	}
	if err := manifest.Write(filepath.Join(r.Dir, manifest.FileName), next); err != nil {
		return res, err
	}
	r.adopt(after)
	r.Sources = sources
	if defaults := sources[src.Name].Source.Defaults; swap && len(defaults) > 0 {
		added, err := r.Add(defaults)
		res.Enabled, res.Sync = added.Added, added.Sync
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

// RemoveSourceResult is what RemoveSource did.
type RemoveSourceResult struct {
	// Source is the entry dropped.
	Source manifest.Source
}

// RemoveSource drops a source from the manifest. It refuses while an enabled
// module comes from the source, naming those modules: it touches no region.
func (r *Repo) RemoveSource(name string) (RemoveSourceResult, error) {
	src, ok := r.Manifest.SourceByName(name)
	if !ok {
		return RemoveSourceResult{}, fmt.Errorf("%w: %q", manifest.ErrUnknownSource, name)
	}
	var using []string
	entries := r.Manifest.Entries()
	for i, ref := range r.Manifest.Modules {
		if ref.Source == name {
			using = append(using, entries[i])
		}
	}
	if len(using) > 0 {
		return RemoveSourceResult{}, fmt.Errorf("%w: %q supplies %s; `agents remove` them first",
			ErrSourceInUse, name, strings.Join(using, ", "))
	}

	next := manifest.Manifest{
		Sources: slices.DeleteFunc(slices.Clone(r.Manifest.Sources), func(s manifest.Source) bool { return s.Name == name }),
		Modules: r.Manifest.Modules,
	}
	if err := manifest.Write(filepath.Join(r.Dir, manifest.FileName), next); err != nil {
		return RemoveSourceResult{}, err
	}
	r.Manifest = next
	delete(r.Sources, name)
	return RemoveSourceResult{Source: src}, nil
}
