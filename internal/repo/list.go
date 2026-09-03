package repo

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// ModuleInfo is one module a source holds, as `agents list` reports it.
type ModuleInfo struct {
	// Name is the entry a manifest would use for this module: the bare name
	// for a module of the default source, source/name for any other. It is
	// what `agents add` takes back, so what is listed can be copied.
	Name string
	// Enabled is true when the repo's manifest lists the module.
	Enabled bool
	// Path is a kind:file module's render target, empty for an inline module.
	Path string
}

// List describes every module every source holds, sorted by the name it would
// take in a manifest, marking the ones enabled lists.
//
// enabled may be nil — outside a managed repo there is nothing to mark, and
// the question "what could I enable?" still has an answer.
func List(sources Sources, defaultSource string, enabled []manifest.ModuleRef) []ModuleInfo {
	var out []ModuleInfo
	for source, src := range sources {
		for _, name := range src.Set.Names() {
			m, ok := src.Set.Get(name)
			if !ok {
				continue
			}
			ref := manifest.ModuleRef{Source: source, Name: name}
			info := ModuleInfo{Name: entryFor(defaultSource, ref), Enabled: slices.Contains(enabled, ref)}
			if m.Kind == module.KindFile {
				info.Path = filepath.ToSlash(filepath.Clean(m.Path))
			}
			out = append(out, info)
		}
	}
	slices.SortFunc(out, func(a, b ModuleInfo) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// entryFor is the form a ref takes in a manifest file: bare when it comes from
// the default source, qualified otherwise.
func entryFor(defaultSource string, ref manifest.ModuleRef) string {
	if ref.Source == defaultSource {
		return ref.Name
	}
	return ref.String()
}
