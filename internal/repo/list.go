package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// ModuleInfo is one module the binary knows, as `agents list` reports it.
type ModuleInfo struct {
	Name string
	// Enabled is true when the repo's manifest lists the module.
	Enabled bool
	// Path is a kind:file module's render target, empty for an inline module.
	Path string
}

// List describes every module in set, sorted by name, marking the ones enabled
// names. Enabled may be nil — outside a managed repo there is nothing to mark,
// and the question "what could I enable?" still has an answer.
func List(set module.Set, enabled []string) []ModuleInfo {
	names := set.Names()
	out := make([]ModuleInfo, 0, len(names))
	for _, name := range names {
		m, ok := set.Get(name)
		if !ok {
			continue
		}
		info := ModuleInfo{Name: name, Enabled: slices.Contains(enabled, name)}
		if m.Kind == module.KindFile {
			info.Path = filepath.ToSlash(filepath.Clean(m.Path))
		}
		out = append(out, info)
	}
	return out
}

// EnabledModules is the module list of dir's manifest, or nil when dir holds
// no manifest at all. A manifest that is there but unreadable is an error: a
// broken .agents.yaml is worth saying out loud, even to `list`.
func EnabledModules(dir string) ([]string, error) {
	path := filepath.Join(dir, manifest.FileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	m, err := manifest.Read(path)
	if err != nil {
		return nil, err
	}
	return m.Modules, nil
}
