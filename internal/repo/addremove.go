package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/render"
)

// What add and remove refuse to do. Each is wrapped with the offending names.
var (
	// ErrUnknownModule means no module of that name exists to enable or disable.
	ErrUnknownModule = errors.New("unknown module")
	// ErrNotEnabled means the manifest does not list the module, so there is
	// nothing to remove.
	ErrNotEnabled = errors.New("module is not enabled")
	// ErrNoModulesLeft means the removal would empty the manifest, which is not
	// a manifest the tool can write.
	ErrNoModulesLeft = errors.New("that would leave no modules")
	// ErrNoModulesGiven means the caller named nothing to do.
	ErrNoModulesGiven = errors.New("no modules given")
)

// AddResult is what Add did.
type AddResult struct {
	// Added are the modules appended to the manifest, in the order given.
	Added []string
	// Already are arguments the manifest already listed. They are a message,
	// not an error: asking for a module you have is not a mistake worth an
	// exit code.
	Already []string
	// Sync is the render that followed. It is the zero value when nothing was
	// added, and carries the blocking diffs when Add returns ErrHandEdited.
	Sync SyncResult
}

// Add appends modules to the manifest and renders them.
//
// The order given is the order they take in the manifest, and the manifest's
// order is the order regions take in the file — a module added now renders
// after every region already there, above whatever tail the project wrote.
// Nothing above the first region is ever touched.
//
// The render is the ordinary Sync, so seeds are created (when Templates is
// set) and a hand-edited region refuses the whole thing. A refusal writes
// nothing at all, the manifest included: the repo is exactly as it was, and
// the result's Sync holds the diffs the caller should show.
//
// The manifest is written last, so the one state an interrupted add can leave
// is a rendered region no manifest entry claims — which `check` calls orphaned
// and `sync` removes.
func (r *Repo) Add(names []string) (AddResult, error) {
	if len(names) == 0 {
		return AddResult{}, ErrNoModulesGiven
	}
	var (
		res     AddResult
		unknown []string
	)
	modules := slices.Clone(r.Manifest.Modules)
	for _, name := range names {
		ref, err := r.Manifest.Ref(name)
		switch {
		case err != nil || !r.knows(ref.Name):
			unknown = append(unknown, name)
		case slices.Contains(modules, ref):
			res.Already = append(res.Already, name)
		default:
			modules = append(modules, ref)
			res.Added = append(res.Added, name)
		}
	}
	if len(unknown) > 0 {
		return AddResult{}, wrapNames(ErrUnknownModule, unknown)
	}
	if len(res.Added) == 0 {
		return res, nil
	}

	next, err := r.with(modules)
	if err != nil {
		return res, err
	}
	// Render before the manifest is written, so a refusal leaves no trace.
	res.Sync, err = next.Sync(false)
	if err != nil {
		return res, err
	}
	if err := manifest.Write(filepath.Join(r.Dir, manifest.FileName), next.Manifest); err != nil {
		return res, err
	}
	r.adopt(next)
	return res, nil
}

// RemovedRegion is one region Remove deleted, and what became of the file it
// was in.
type RemovedRegion struct {
	Module string
	// Target is the repo-relative file the region was in.
	Target string
	// Present is false when there was no such region to remove — an already
	// half-removed repo, not an error.
	Present bool
	// HandEdited means the body no longer matched its marker: someone had
	// changed it since the render, and remove took it anyway.
	HandEdited bool
	// FileDeleted means the file held nothing but this region, so it is gone.
	FileDeleted bool
	// FileKept means a kind:file module's file stays, because the project has
	// written in it; only the region left.
	FileKept bool
}

// RemoveResult is what Remove did.
type RemoveResult struct {
	// Removed are the modules dropped from the manifest, in the order given.
	Removed []string
	// Regions is one entry per removed module, in the same order.
	Regions []RemovedRegion
}

// Remove drops modules from the manifest and deletes their regions.
//
// It is not a sync: only the named regions are touched, so a stale or
// hand-edited region of another module is left exactly as it was, and a
// hand-edited region of a module being removed goes anyway — removing is a
// decision. The one region it touches unasked is the generated index, whose
// rows are the manifest's and so cannot outlive it. A kind:file module's file
// is deleted when it held nothing but the region; a file the project has
// written in keeps every one of its own bytes and only loses the region, and
// the result says so for the caller to warn.
// Nothing above the first region of AGENTS.md is ever touched.
//
// The result describes a completed removal. The manifest is written last, so
// the one state an interrupted remove can leave is a manifest entry whose
// region is gone — which `check` calls missing and `sync` renders back.
func (r *Repo) Remove(names []string) (RemoveResult, error) {
	if len(names) == 0 {
		return RemoveResult{}, ErrNoModulesGiven
	}
	var res RemoveResult
	var unknown, disabled []string
	for _, name := range names {
		ref, err := r.Manifest.Ref(name)
		switch {
		case err != nil || !r.knows(ref.Name):
			unknown = append(unknown, name)
		case !slices.Contains(r.Manifest.Modules, ref):
			disabled = append(disabled, name)
		case slices.Contains(res.Removed, name):
			// Named twice; once is enough.
		default:
			res.Removed = append(res.Removed, name)
		}
	}
	if len(unknown) > 0 {
		return RemoveResult{}, wrapNames(ErrUnknownModule, unknown)
	}
	if len(disabled) > 0 {
		return RemoveResult{}, wrapNames(ErrNotEnabled, disabled)
	}
	kept := slices.DeleteFunc(slices.Clone(r.Manifest.Modules), func(ref manifest.ModuleRef) bool {
		return slices.Contains(res.Removed, ref.Name)
	})
	if len(kept) == 0 {
		return RemoveResult{}, wrapNames(ErrNoModulesLeft, res.Removed)
	}
	next, err := r.with(kept)
	if err != nil {
		return RemoveResult{}, err
	}

	for _, name := range res.Removed {
		mod, _ := r.Set.Get(name)
		removed, err := r.removeRegion(mod)
		res.Regions = append(res.Regions, removed)
		if err != nil {
			return res, err
		}
	}
	if err := next.updateIndex(); err != nil {
		return res, err
	}
	if err := manifest.Write(filepath.Join(r.Dir, manifest.FileName), next.Manifest); err != nil {
		return res, err
	}
	r.adopt(next)
	return res, nil
}

// updateIndex brings AGENTS.md's generated index into line with the modules
// this repo now has. Remove touches no region it was not asked to, and the
// index is the exception it cannot avoid: its body is the manifest's, so
// dropping a situational module leaves a row that names a file the removal
// just deleted. Add needs no such thing — it renders through Sync.
func (r *Repo) updateIndex() error {
	path := filepath.Join(r.Dir, AgentsFile)
	current, exists, err := readIfPresent(path)
	if err != nil || !exists {
		return err
	}
	var next string
	if idx, ok := render.Index(r.mods); ok {
		next, err = render.SetRegion(current, idx)
	} else {
		next, _, err = render.RemoveRegions(current, render.IndexName)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", AgentsFile, err)
	}
	if next == current {
		return nil
	}
	return writeFile(path, next)
}

// removeRegion deletes one module's region from the file that holds it, and
// deletes that file when the region was all it held.
func (r *Repo) removeRegion(m module.Module) (RemovedRegion, error) {
	out := RemovedRegion{Module: m.Name, Target: AgentsFile}
	if m.Kind == module.KindFile {
		out.Target = filepath.ToSlash(filepath.Clean(m.Path))
	}
	path := filepath.Join(r.Dir, filepath.FromSlash(out.Target))
	current, exists, err := readIfPresent(path)
	if err != nil {
		return out, err
	}
	if !exists {
		return out, nil
	}
	rest, removed, err := render.RemoveRegions(current, m.Name)
	if err != nil {
		return out, fmt.Errorf("%s: %w", out.Target, err)
	}
	if len(removed) == 0 {
		return out, nil
	}
	out.Present = true
	out.HandEdited = module.Hash(removed[0].Body) != removed[0].MarkerHash

	// Only a generated file can go: AGENTS.md is the repo's, whatever is left
	// of it. What is left of a generated file is the project's the moment it
	// is more than blank space.
	if m.Kind == module.KindFile && strings.TrimSpace(rest) == "" {
		if err := os.Remove(path); err != nil {
			return out, fmt.Errorf("removing %s: %w", out.Target, err)
		}
		out.FileDeleted = true
		return out, nil
	}
	if err := writeFile(path, rest); err != nil {
		return out, err
	}
	out.FileKept = m.Kind == module.KindFile
	return out, nil
}

// knows reports whether the module set has a module of this name.
func (r *Repo) knows(name string) bool {
	_, ok := r.Set.Get(name)
	return ok
}

// with is this repo with a different module list, resolved but not yet
// written: the shape add and remove need to render or check a change before
// committing it to .agents.yaml.
func (r *Repo) with(modules []manifest.ModuleRef) (*Repo, error) {
	m := manifest.Manifest{Sources: r.Manifest.Sources, Modules: modules}
	mods, err := manifest.Resolve(m, r.Set)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.Dir, err)
	}
	return &Repo{Dir: r.Dir, Set: r.Set, Manifest: m, Templates: r.Templates, mods: mods}, nil
}

// adopt takes on the manifest a change has just written, so a caller that goes
// on using the repo sees what is on disk.
func (r *Repo) adopt(next *Repo) {
	r.Manifest, r.mods = next.Manifest, next.mods
}

func wrapNames(err error, names []string) error {
	return fmt.Errorf("%w: %s", err, strings.Join(names, ", "))
}
