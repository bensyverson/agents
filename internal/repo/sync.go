package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bensyverson/agents/internal/render"
)

const (
	// newFileMode is what a file the tool creates gets; an existing file keeps
	// whatever mode the project gave it.
	newFileMode fs.FileMode = 0o644
	dirMode     fs.FileMode = 0o755
	// tempPattern names the temp file the atomic write renames from. The dot
	// keeps it out of the way if a crash ever leaves one behind.
	tempPattern = ".agents-*.tmp"
)

// ErrHandEdited is returned when sync would overwrite a region an agent or a
// human changed by hand. It is the whole point of the marker hashes.
var ErrHandEdited = errors.New("hand-edited regions would be overwritten")

// EditedRegion is one region that differs from its module, with the diff
// between the two. Sync and Diff render the diff from opposite sides — see
// their doc comments.
type EditedRegion struct {
	// Target is the repo-relative file the region lives in.
	Target string
	Region string
	Diff   string
}

// SyncResult is what a sync did, or refused to do.
type SyncResult struct {
	// Written and Fresh are repo-relative paths, in target order.
	Written []string
	Fresh   []string
	// Seeded is each file created from a module's declared seed, in manifest
	// order. Only ever files that were missing: seeds are never overwritten.
	Seeded []string
	// Edited is non-empty only when the sync refused.
	Edited []EditedRegion
}

// Refused reports whether hand edits stopped the sync.
func (r SyncResult) Refused() bool { return len(r.Edited) > 0 }

// Sync renders every target that needs writing.
//
// A hand-edited region anywhere stops the whole sync — no file is written, not
// even an untouched one — unless force is set; the result then carries each
// edited region with a diff from the current body to the module body, so the
// caller can show what --force would throw away. Nothing stale means no writes
// and an empty result.
func (r *Repo) Sync(force bool) (SyncResult, error) {
	targets, err := r.Targets()
	if err != nil {
		return SyncResult{}, err
	}

	var res SyncResult
	var pending []Target
	for _, t := range targets {
		if !t.NeedsWrite() {
			res.Fresh = append(res.Fresh, t.Path)
			continue
		}
		pending = append(pending, t)
		if force {
			continue
		}
		for _, region := range t.Report.Regions {
			if !region.Edited {
				continue
			}
			res.Edited = append(res.Edited, EditedRegion{
				Target: t.Path,
				Region: region.Name,
				Diff:   render.Diff(region.Body, t.moduleBody(region.Name)),
			})
		}
	}
	if res.Refused() {
		return res, fmt.Errorf("%s: %w", r.Dir, ErrHandEdited)
	}

	for _, t := range pending {
		if err := writeFile(filepath.Join(r.Dir, t.Path), t.Rendered); err != nil {
			return res, err
		}
		res.Written = append(res.Written, t.Path)
	}

	// Seeding after the render, and on every sync rather than on init alone,
	// so a repo picks up a seed its modules gained since it was set up.
	seeded, err := r.seed()
	res.Seeded = seeded
	if err != nil {
		return res, fmt.Errorf("%s: %w", r.Dir, err)
	}
	return res, nil
}

// writeFile replaces path atomically: a temp file in the same directory, then a
// rename, so a crash never leaves a half-written AGENTS.md. A path that is
// itself a symlink is resolved first, so the write goes through the link
// instead of replacing it with a regular file.
func writeFile(path, content string) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	mode := newFileMode
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	// Harmless after a successful rename; the safety net for every early return.
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("setting mode on %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
