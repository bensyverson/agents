package repo

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/bensyverson/agents/internal/gotchas"
)

// Gotchas reads and parses the repo's gotchas file. It is the single read that
// status, diff and sync share; a missing file is not an error, and reports
// exists false.
func (r *Repo) Gotchas() (gotchas.File, bool, error) {
	return gotchas.Read(filepath.Join(r.Dir, GotchasFile))
}

// ReseedGotchas replaces the gotchas file's preamble with the one its source
// ships, and reports whether it wrote. The entries below the first rule line
// are copied byte for byte — this only ever refreshes the instructions, which
// are the template's to own.
//
// The template is the one the seed came from: whichever enabled module
// declares the gotchas file, read from that module's source. A repo no enabled
// module seeds a gotchas file for has nothing to refresh and is left alone,
// and so is a repo with no gotchas file — creating one is init's seeding step.
func (r *Repo) ReseedGotchas() (bool, error) {
	templates := r.templatesSeeding(GotchasFile)
	if templates == nil {
		return false, nil
	}
	path := filepath.Join(r.Dir, GotchasFile)
	current, exists, err := readIfPresent(path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	template, err := fs.ReadFile(templates, GotchasFile)
	if err != nil {
		return false, fmt.Errorf("reading template %s: %w", GotchasFile, err)
	}
	next, changed := gotchas.Reseed(current, string(template))
	if !changed {
		return false, nil
	}
	if err := writeFile(path, next); err != nil {
		return false, err
	}
	return true, nil
}

// templatesSeeding is the templates of the source that supplies rel, found
// through the enabled module that declares it as a seed.
func (r *Repo) templatesSeeding(rel string) fs.FS {
	for _, m := range r.mods {
		if slices.Contains(m.Seeds, rel) {
			return r.templatesFor(m.Name)
		}
	}
	return nil
}
