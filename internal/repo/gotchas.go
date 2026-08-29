package repo

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/bensyverson/agents/internal/gotchas"
)

// Gotchas reads and parses the repo's gotchas file. It is the single read that
// status, diff and sync share; a missing file is not an error, and reports
// exists false.
func (r *Repo) Gotchas() (gotchas.File, bool, error) {
	return gotchas.Read(filepath.Join(r.Dir, GotchasFile))
}

// ReseedGotchas replaces the gotchas file's preamble with the one in
// templates, and reports whether it wrote. The entries below the first rule
// line are copied byte for byte — this only ever refreshes the instructions,
// which are the template's to own.
//
// A repo with no gotchas file is left alone: creating one is init's seeding
// step, not this. A nil template FS skips the step, as it does in Init.
func (r *Repo) ReseedGotchas(templates fs.FS) (bool, error) {
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
