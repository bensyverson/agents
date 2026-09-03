package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// SeedPolicy says whether a sync creates the files the repo's modules declare
// as seeds. The read-only verbs open a repo that creates nothing at all, so
// `agents check` can never leave a file behind.
type SeedPolicy int

const (
	// SeedsSkipped leaves declared seeds alone.
	SeedsSkipped SeedPolicy = iota
	// SeedsCreated creates every declared seed the repo does not have yet.
	SeedsCreated
)

// seed creates every file the repo's modules declare as a seed and that the
// repo does not have yet, and returns the paths it created in manifest order.
// Each seed's body comes from the templates of the source its module came
// from: a source ships its rules and the files those rules refer to together.
//
// It never overwrites: a seeded file is the repo's own from the moment it
// exists, so a project can rewrite its gotchas or backlog freely. A declared
// seed with no template is a packaging mistake and an error — silently seeding
// nothing would leave every repo missing a file its rules refer to.
func (r *Repo) seed() ([]string, error) {
	if r.Seeds != SeedsCreated {
		return nil, nil
	}
	var created []string
	done := make(map[string]bool)
	for _, m := range r.mods {
		templates := r.templatesFor(m.Name)
		for _, rel := range m.Seeds {
			if done[rel] {
				continue
			}
			done[rel] = true
			body, err := readTemplate(templates, rel)
			if err != nil {
				return created, fmt.Errorf("module %s seeds %s: reading template: %w", m.Name, rel, err)
			}
			wrote, err := createIfMissing(filepath.Join(r.Dir, filepath.FromSlash(rel)), body)
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

// readTemplate reads one seed's body, treating a source with no templates at
// all as a source missing that template.
func readTemplate(templates fs.FS, rel string) ([]byte, error) {
	if templates == nil {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(templates, rel)
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
