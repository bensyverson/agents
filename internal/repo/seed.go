package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bensyverson/agents/internal/module"
)

// seed creates every file the repo's modules declare as a seed and that the
// repo does not have yet, from templates/<seed path>, and returns the paths it
// created in manifest order.
//
// It never overwrites: a seeded file is the repo's own from the moment it
// exists, so a project can rewrite its gotchas or backlog freely. A declared
// seed with no template is a packaging mistake and an error — silently seeding
// nothing would leave every repo missing a file its rules refer to.
func seed(dir string, mods []module.Module, templates fs.FS) ([]string, error) {
	if templates == nil {
		return nil, nil
	}
	var created []string
	done := make(map[string]bool)
	for _, m := range mods {
		for _, rel := range m.Seeds {
			if done[rel] {
				continue
			}
			done[rel] = true
			body, err := fs.ReadFile(templates, rel)
			if err != nil {
				return created, fmt.Errorf("module %s seeds %s: reading template: %w", m.Name, rel, err)
			}
			wrote, err := createIfMissing(filepath.Join(dir, filepath.FromSlash(rel)), body)
			if err != nil {
				return created, err
			}
			if wrote {
				created = append(created, rel)
			}
		}
	}
	return created, nil
}

// createIfMissing writes content to path only if nothing is there yet, creating
// the parent directory, and reports whether it created the file.
func createIfMissing(path string, content []byte) (bool, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return false, fmt.Errorf("creating %s: %w", dir, err)
	}
	// O_EXCL rather than a Stat: never race another writer into an overwrite.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, newFileMode)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, fmt.Errorf("creating %s: %w", path, err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	return true, nil
}
