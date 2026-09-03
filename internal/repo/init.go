package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
	"github.com/bensyverson/agents/internal/registry"
)

// DefaultModules is what a repo gets when init is given no module list: the
// universal rules, the principles, and the pre-launch stage.
func DefaultModules() []string { return []string{"core", "principles", "stage-build"} }

// InitOptions is everything init needs from its caller; nothing is read from
// the environment here, so tests never touch a real home directory.
type InitOptions struct {
	// Modules is the manifest to write; empty means DefaultModules.
	Modules []string
	Set     module.Set
	// Templates supplies HeadTemplate and the body of every seed the modules
	// declare, keyed by the seed's repo-relative path. A nil FS skips the
	// seeding steps.
	Templates fs.FS
	// RegistryPath is the repo list to append to; empty skips registration.
	RegistryPath string
}

// LinkResult says what init did about CLAUDE.md.
type LinkResult int

const (
	// LinkCreated means init made the CLAUDE.md -> AGENTS.md symlink.
	LinkCreated LinkResult = iota
	// LinkAlready means it was already that symlink.
	LinkAlready
	// LinkForeign means CLAUDE.md exists and is something else; init left it
	// alone rather than clobber a real file.
	LinkForeign
)

func (l LinkResult) String() string {
	switch l {
	case LinkCreated:
		return "created"
	case LinkAlready:
		return "already linked"
	case LinkForeign:
		return "left alone"
	default:
		return fmt.Sprintf("LinkResult(%d)", int(l))
	}
}

// InitResult reports which steps init performed and which it found already done.
type InitResult struct {
	Dir     string
	Modules []string
	// ManifestWritten is false when .agents.yaml already listed these modules.
	ManifestWritten bool
	// Sync is the render step: which files were written, which were fresh, and
	// which of the modules' declared seeds it created.
	Sync SyncResult
	// HeadSeeded is true when init wrote the head template as a brand-new
	// AGENTS.md's project-owned head.
	HeadSeeded bool
	Link       LinkResult
	// Registered is false when the repo was already in the registry.
	Registered   bool
	RegistryPath string
}

// ManifestConflictError says the repo already has a manifest listing different
// modules. Init never edits a manifest a human curated.
type ManifestConflictError struct {
	Dir       string
	Existing  []string
	Requested []string
}

func (e *ManifestConflictError) Error() string {
	return fmt.Sprintf("%s already lists %s, not %s; edit %s yourself and run `agents sync`",
		filepath.Join(e.Dir, manifest.FileName),
		strings.Join(e.Existing, ", "), strings.Join(e.Requested, ", "), manifest.FileName)
}

// Init makes dir a managed repo, and is safe to re-run: every step either acts
// or reports that it was already done. An existing AGENTS.md keeps all of its
// content — it becomes the project-owned head and the regions are appended —
// so pruning stays a human step.
func Init(dir string, opts InitOptions) (InitResult, error) {
	modules := opts.Modules
	if len(modules) == 0 {
		modules = DefaultModules()
	}
	res := InitResult{Dir: dir, Modules: modules, RegistryPath: opts.RegistryPath}

	refs := make([]manifest.ModuleRef, 0, len(modules))
	for _, name := range modules {
		ref, err := manifest.ParseRef(name)
		if err != nil {
			return res, fmt.Errorf("%s: %w", dir, err)
		}
		refs = append(refs, ref)
	}
	wanted := manifest.Manifest{Modules: refs}
	if _, err := manifest.Marshal(wanted); err != nil {
		return res, err
	}
	if _, err := manifest.Resolve(wanted, opts.Set); err != nil {
		return res, fmt.Errorf("%s: %w", dir, err)
	}

	written, err := writeManifest(dir, wanted)
	if err != nil {
		return res, err
	}
	res.ManifestWritten = written

	// Seeding precedes the render so the template becomes the head of the
	// document the regions are appended to, rather than a second file.
	if res.HeadSeeded, err = seedHead(dir, opts.Templates); err != nil {
		return res, err
	}

	r, err := Open(dir, opts.Set)
	if err != nil {
		return res, err
	}
	// The render and the seeds are one step: Sync creates whatever the
	// manifest's modules declare and this repo does not have yet.
	r.Templates = opts.Templates
	if res.Sync, err = r.Sync(false); err != nil {
		return res, err
	}

	if res.Link, err = linkClaude(dir); err != nil {
		return res, err
	}
	if res.Registered, err = register(dir, opts.RegistryPath); err != nil {
		return res, err
	}
	return res, nil
}

// writeManifest creates .agents.yaml, and reports whether it had to. An
// existing manifest with the same modules is left byte-identical, formatting
// and all.
func writeManifest(dir string, wanted manifest.Manifest) (bool, error) {
	path := filepath.Join(dir, manifest.FileName)
	existing, err := manifest.Read(path)
	switch {
	case err == nil:
		if !slices.Equal(existing.Entries(), wanted.Entries()) {
			return false, &ManifestConflictError{Dir: dir, Existing: existing.Entries(), Requested: wanted.Entries()}
		}
		return false, nil
	case !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return false, fmt.Errorf("creating %s: %w", dir, err)
	}
	if err := manifest.Write(path, wanted); err != nil {
		return false, err
	}
	return true, nil
}

// seedHead gives a repo with no AGENTS.md at all the head template as its
// project-owned head, so the first render is head + regions and the repo has a
// place for its own rules. An AGENTS.md that already exists — even an empty one
// — is the project's head already, and is never prefixed.
func seedHead(dir string, templates fs.FS) (bool, error) {
	if templates == nil {
		return false, nil
	}
	path := filepath.Join(dir, AgentsFile)
	// Lstat, not Stat: a dangling AGENTS.md symlink is still the project's.
	switch _, err := os.Lstat(path); {
	case err == nil:
		return false, nil
	case !errors.Is(err, fs.ErrNotExist):
		return false, fmt.Errorf("inspecting %s: %w", path, err)
	}
	head, err := fs.ReadFile(templates, HeadTemplate)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// No head template is a supported configuration: render from nothing.
		return false, nil
	case err != nil:
		return false, fmt.Errorf("reading template %s: %w", HeadTemplate, err)
	}
	if err := writeFile(path, string(head)); err != nil {
		return false, err
	}
	return true, nil
}

// linkClaude points CLAUDE.md at AGENTS.md with a relative symlink so the two
// files can never drift, and refuses to touch anything else living there.
func linkClaude(dir string) (LinkResult, error) {
	path := filepath.Join(dir, ClaudeFile)
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.Symlink(AgentsFile, path); err != nil {
			return LinkForeign, fmt.Errorf("linking %s: %w", path, err)
		}
		return LinkCreated, nil
	case err != nil:
		return LinkForeign, fmt.Errorf("inspecting %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return LinkForeign, fmt.Errorf("reading link %s: %w", path, err)
		}
		if filepath.Clean(target) == AgentsFile {
			return LinkAlready, nil
		}
	}
	return LinkForeign, nil
}

// register records the repo under its absolute path, so `--all` works from
// anywhere.
func register(dir, registryPath string) (bool, error) {
	if registryPath == "" {
		return false, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false, fmt.Errorf("resolving %s: %w", dir, err)
	}
	return registry.Add(registryPath, abs)
}
