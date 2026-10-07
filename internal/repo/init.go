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
	"github.com/bensyverson/agents/internal/registry"
	"github.com/bensyverson/agents/internal/source"
)

// ErrModulesRequired says init was given no module list and the default
// source declares none. The tool names no module of its own, so which of a
// source's modules to enable is the source's to say (source.yaml defaults) or
// the caller's (--with); guessing would render a manifest nobody asked for.
var ErrModulesRequired = errors.New("no modules to enable")

// InitOptions is everything init needs from its caller; nothing is read from
// the environment here, so tests never touch a real home directory.
type InitOptions struct {
	// Modules is the manifest to write. Empty means the default source's
	// declared defaults — or, in a repo that already has a manifest, whatever
	// that manifest lists — and is ErrModulesRequired when there are none.
	Modules []string
	// Source is the one source the new manifest names. The zero value writes
	// no sources: block at all, so the repo renders from the source the binary
	// embeds — the offline default, and the only one a fresh install has.
	Source manifest.Source
	// Open is how the repo is opened once the manifest is written. Init may
	// fetch a source the cache lacks, so its Fetch is FetchMissing.
	Open Options
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
	// Source is the source the manifest names, with a git ref resolved to the
	// sha it was pinned at. The zero value means the embedded source.
	Source manifest.Source
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

// SourceConflictError says the repo's manifest already names other sources.
// Init never repoints a manifest at a different source: that is a decision
// with a diff behind it, so a human edits the file and runs `agents sync`.
type SourceConflictError struct {
	Dir                 string
	Existing, Requested []manifest.Source
}

func (e *SourceConflictError) Error() string {
	return fmt.Sprintf("%s already names %s, not %s; edit %s yourself and run `agents sync`",
		filepath.Join(e.Dir, manifest.FileName),
		sourceNames(e.Existing), sourceNames(e.Requested), manifest.FileName)
}

// sourceNames is "name (location)" per source, for one line of an error.
func sourceNames(sources []manifest.Source) string {
	if len(sources) == 0 {
		return "no sources"
	}
	out := make([]string, len(sources))
	for i, s := range sources {
		location := s.Git
		if location == "" {
			location = s.Path
		}
		out[i] = fmt.Sprintf("%s (%s)", s.Name, location)
	}
	return strings.Join(out, ", ")
}

// Init makes dir a managed repo, and is safe to re-run: every step either acts
// or reports that it was already done. An existing AGENTS.md keeps all of its
// content — it becomes the project-owned head and the regions are appended —
// so pruning stays a human step.
func Init(dir string, opts InitOptions) (InitResult, error) {
	res := InitResult{Dir: dir, RegistryPath: opts.RegistryPath}

	// A manifest the repo already has wins: init never edits one a human
	// curated, and says so rather than rewriting it.
	existing, hasManifest, err := readExisting(dir)
	if err != nil {
		return res, err
	}
	wanted := manifest.Manifest{}
	if opts.Source.Name != "" {
		wanted.Sources = []manifest.Source{opts.Source}
	}
	if hasManifest {
		if err := agreesOnSources(dir, existing, wanted); err != nil {
			return res, err
		}
		wanted.Sources = existing.Sources
	}

	// The manifest records the sha the ref resolved to, so the repo is pinned
	// even when the human asked for a branch. An existing manifest keeps its
	// own pin; moving one is `agents update`'s job.
	if !hasManifest && len(wanted.Sources) == 1 {
		if err := pinSource(opts.Open.Loader, &wanted.Sources[0]); err != nil {
			return res, fmt.Errorf("%s: %w", dir, err)
		}
	}
	sources, err := LoadSources(dir, wanted, opts.Open)
	if err != nil {
		return res, fmt.Errorf("%s: %w", dir, err)
	}
	if len(wanted.Sources) == 1 {
		res.Source = wanted.Sources[0]
	}

	modules, err := initModules(opts.Modules, existing, hasManifest, sources, wanted.DefaultSource())
	if err != nil {
		return res, fmt.Errorf("%s: %w", dir, err)
	}
	res.Modules = modules
	refs := make([]manifest.ModuleRef, 0, len(modules))
	for _, name := range modules {
		ref, err := manifest.ParseRef(name)
		if err != nil {
			return res, fmt.Errorf("%s: %w", dir, err)
		}
		refs = append(refs, ref)
	}
	wanted.Modules = refs
	if hasManifest {
		if err := agreesOnModules(dir, existing, wanted); err != nil {
			return res, err
		}
		wanted = existing
	}

	if _, err := manifest.Marshal(wanted); err != nil {
		return res, err
	}
	mods, origin, err := sources.resolve(wanted)
	if err != nil {
		return res, fmt.Errorf("%s: %w", dir, err)
	}

	if !hasManifest {
		if err := manifest.Write(filepath.Join(dir, manifest.FileName), wanted); err != nil {
			return res, err
		}
		res.ManifestWritten = true
	}

	r := &Repo{Dir: dir, Manifest: wanted, Sources: sources, Seeds: SeedsCreated, mods: mods, origin: origin}
	// Seeding precedes the render so the template becomes the head of the
	// document the regions are appended to, rather than a second file. The
	// head comes from the default source: the one a bare module entry names.
	if res.HeadSeeded, err = seedHead(dir, sources.templatesOf(wanted.DefaultSource())); err != nil {
		return res, err
	}
	// The render and the seeds are one step: Sync creates whatever the
	// manifest's modules declare and this repo does not have yet.
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

// initModules is the module list init works from: the caller's, else the one
// the repo's manifest already holds, else the default source's declared
// defaults. The tool names none of its own.
func initModules(given []string, existing manifest.Manifest, hasManifest bool, sources Sources, defaultSource string) ([]string, error) {
	switch {
	case len(given) > 0:
		return given, nil
	case hasManifest:
		return existing.Entries(), nil
	}
	if defaults := sources[defaultSource].Source.Defaults; len(defaults) > 0 {
		return defaults, nil
	}
	return nil, fmt.Errorf("%w: source %q declares no default modules (defaults: in its %s); name them with --with",
		ErrModulesRequired, defaultSource, source.SourceFile)
}

// templatesOf is the templates of one loaded source, or nil when nothing goes
// by that name.
func (s Sources) templatesOf(name string) fs.FS {
	src, ok := s[name]
	if !ok {
		return nil
	}
	return src.Source.Templates
}

// readExisting reads the manifest dir already has, reporting false when there
// is none.
func readExisting(dir string) (manifest.Manifest, bool, error) {
	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	switch {
	case err == nil:
		return m, true, nil
	case errors.Is(err, fs.ErrNotExist):
		return manifest.Manifest{}, false, nil
	}
	return manifest.Manifest{}, false, err
}

// agreesOnSources reports whether an existing manifest names the source init
// was asked for: by name and location, but not by ref — the repo's own pin
// stands, and moving it is `agents update`'s job. No requested source agrees
// with anything.
func agreesOnSources(dir string, existing, wanted manifest.Manifest) error {
	for _, want := range wanted.Sources {
		got, ok := existing.SourceByName(want.Name)
		if !ok || got.Git != want.Git || got.Path != want.Path {
			return &SourceConflictError{Dir: dir, Existing: existing.Sources, Requested: wanted.Sources}
		}
	}
	return nil
}

// agreesOnModules reports whether an existing manifest lists exactly the
// modules init was asked for.
func agreesOnModules(dir string, existing, wanted manifest.Manifest) error {
	if !slices.Equal(existing.Entries(), wanted.Entries()) {
		return &ManifestConflictError{Dir: dir, Existing: existing.Entries(), Requested: wanted.Entries()}
	}
	return nil
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
