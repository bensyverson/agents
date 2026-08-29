package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/registry"
)

func initOptions(t *testing.T, modules ...string) InitOptions {
	t.Helper()
	return InitOptions{
		Modules:      modules,
		Set:          testSet(t),
		Templates:    testTemplates(),
		RegistryPath: filepath.Join(t.TempDir(), "repos"),
	}
}

// defaultRegions is what the default module list renders to in AGENTS.md.
func defaultRegions() string {
	return fresh("core", coreBody) + "\n" + fresh("principles", principlesBody) + "\n" + fresh("stage-build", stageBody)
}

func TestDefaultModules(t *testing.T) {
	want := []string{"core", "principles", "stage-build"}
	if !reflect.DeepEqual(DefaultModules(), want) {
		t.Errorf("DefaultModules() = %v, want %v", DefaultModules(), want)
	}
}

func TestInitFromEmptyDir(t *testing.T) {
	dir := t.TempDir()
	opts := initOptions(t)

	res, err := Init(dir, opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got, want := readFixture(t, dir, manifest.FileName), "modules:\n  - core\n  - principles\n  - stage-build\n"; got != want {
		t.Errorf("%s:\n got %q\nwant %q", manifest.FileName, got, want)
	}
	if got := readFixture(t, dir, AgentsFile); got != defaultRegions() {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, defaultRegions())
	}
	if got := readFixture(t, dir, GotchasFile); got != templateBody {
		t.Errorf("%s = %q, want the template", GotchasFile, got)
	}
	target, err := os.Readlink(filepath.Join(dir, ClaudeFile))
	if err != nil {
		t.Fatalf("readlink %s: %v", ClaudeFile, err)
	}
	if target != AgentsFile {
		t.Errorf("%s -> %q, want %q", ClaudeFile, target, AgentsFile)
	}
	repos, err := registry.Read(opts.RegistryPath)
	if err != nil {
		t.Fatalf("registry.Read: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	if len(repos) != 1 || repos[0] != filepath.Clean(abs) {
		t.Errorf("registry = %v, want [%s]", repos, abs)
	}

	if !res.ManifestWritten {
		t.Error("ManifestWritten = false")
	}
	if want := []string{GotchasFile}; !reflect.DeepEqual(res.Sync.Seeded, want) {
		t.Errorf("Sync.Seeded = %v, want %v", res.Sync.Seeded, want)
	}
	if res.Link != LinkCreated {
		t.Errorf("Link = %v, want LinkCreated", res.Link)
	}
	if !res.Registered {
		t.Error("Registered = false")
	}
	if len(res.Sync.Written) != 1 || res.Sync.Written[0] != AgentsFile {
		t.Errorf("Sync.Written = %v, want [%s]", res.Sync.Written, AgentsFile)
	}
}

// The whole point of init on an existing repo: nothing the project wrote is lost.
func TestInitPreservesExistingAgentsFile(t *testing.T) {
	head := "# Project\n\nHand-written overview.\n"
	dir := newRepo(t, map[string]string{AgentsFile: head})

	if _, err := Init(dir, initOptions(t, "core")); err != nil {
		t.Fatalf("Init: %v", err)
	}

	want := head + "\n" + fresh("core", coreBody)
	if got := readFixture(t, dir, AgentsFile); got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	opts := initOptions(t)
	if _, err := Init(dir, opts); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	before := snapshot(t, dir)
	registryBefore := readFile(t, opts.RegistryPath)

	res, err := Init(dir, opts)
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}

	if after := snapshot(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("second Init changed bytes on disk:\nbefore %v\n after %v", before, after)
	}
	if got := readFile(t, opts.RegistryPath); got != registryBefore {
		t.Errorf("second Init changed the registry: %q -> %q", registryBefore, got)
	}
	if res.ManifestWritten || res.Registered || len(res.Sync.Seeded) != 0 {
		t.Errorf("second Init reports work done: %+v", res)
	}
	if res.Link != LinkAlready {
		t.Errorf("Link = %v, want LinkAlready", res.Link)
	}
	if len(res.Sync.Written) != 0 {
		t.Errorf("Sync.Written = %v, want nothing", res.Sync.Written)
	}
}

func TestInitUnknownModules(t *testing.T) {
	dir := t.TempDir()

	_, err := Init(dir, initOptions(t, "core", "nope", "alsonope"))
	if err == nil {
		t.Fatal("want an error naming the unknown modules, got nil")
	}
	for _, name := range []string{"nope", "alsonope"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %q", err, name)
		}
	}
	if fileExists(t, dir, manifest.FileName) {
		t.Error("a manifest was written despite the error")
	}
}

// init never rewrites a manifest a human has curated.
func TestInitManifestConflict(t *testing.T) {
	existing := "modules:\n  - core\n"
	dir := newRepo(t, map[string]string{manifest.FileName: existing})

	_, err := Init(dir, initOptions(t, "core", "principles"))
	if err == nil {
		t.Fatal("want an error on a differing manifest, got nil")
	}
	if !strings.Contains(err.Error(), manifest.FileName) || !strings.Contains(err.Error(), "sync") {
		t.Errorf("error %q should tell the user to edit %s and run sync", err, manifest.FileName)
	}
	if got := readFixture(t, dir, manifest.FileName); got != existing {
		t.Errorf("manifest = %q, want it untouched", got)
	}
	if fileExists(t, dir, AgentsFile) {
		t.Error("AGENTS.md was rendered despite the conflict")
	}
}

func TestInitKeepsIdenticalManifestBytes(t *testing.T) {
	// Same modules, different formatting: init must not reformat it.
	existing := "modules:\n- core\n- principles\n"
	dir := newRepo(t, map[string]string{manifest.FileName: existing})

	res, err := Init(dir, initOptions(t, "core", "principles"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.ManifestWritten {
		t.Error("ManifestWritten = true for an identical module list")
	}
	if got := readFixture(t, dir, manifest.FileName); got != existing {
		t.Errorf("manifest = %q, want it byte-identical", got)
	}
}

func TestInitLeavesForeignClaudeFile(t *testing.T) {
	const content = "# My own CLAUDE.md\n"
	dir := newRepo(t, map[string]string{ClaudeFile: content})

	res, err := Init(dir, initOptions(t, "core"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.Link != LinkForeign {
		t.Errorf("Link = %v, want LinkForeign", res.Link)
	}
	if got := readFixture(t, dir, ClaudeFile); got != content {
		t.Errorf("%s = %q, want it untouched", ClaudeFile, got)
	}
}

func TestInitLeavesExistingGotchas(t *testing.T) {
	const content = "# Gotchas\n\nours\n"
	dir := newRepo(t, map[string]string{GotchasFile: content})

	res, err := Init(dir, initOptions(t, "core"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if len(res.Sync.Seeded) != 0 {
		t.Errorf("Sync.Seeded = %v over an existing file", res.Sync.Seeded)
	}
	if got := readFixture(t, dir, GotchasFile); got != content {
		t.Errorf("%s = %q, want it untouched", GotchasFile, got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func initOptionsWithHead(t *testing.T, modules ...string) InitOptions {
	t.Helper()
	opts := initOptions(t, modules...)
	opts.Templates = testTemplatesWithHead()
	return opts
}

// A repo with no AGENTS.md at all gets one that starts with the head template:
// the project's own rules have a place to live above the generated regions.
func TestInitSeedsHeadFromTemplate(t *testing.T) {
	dir := t.TempDir()

	res, err := Init(dir, initOptionsWithHead(t))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !res.HeadSeeded {
		t.Error("HeadSeeded = false on an empty directory")
	}
	want := headTemplateBody + "\n" + defaultRegions()
	if got := readFixture(t, dir, AgentsFile); got != want {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
	}
	if len(res.Sync.Written) != 1 || res.Sync.Written[0] != AgentsFile {
		t.Errorf("Sync.Written = %v, want [%s]", res.Sync.Written, AgentsFile)
	}
}

// An existing AGENTS.md is already the head, whatever it says — even nothing.
func TestInitDoesNotSeedHeadOverExistingAgentsFile(t *testing.T) {
	cases := map[string]string{
		"with content": "# Ours\n\nHand-written overview.\n",
		"empty":        "",
	}
	for name, existing := range cases {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t, map[string]string{AgentsFile: existing})

			res, err := Init(dir, initOptionsWithHead(t, "core"))
			if err != nil {
				t.Fatalf("Init: %v", err)
			}

			if res.HeadSeeded {
				t.Error("HeadSeeded = true over an existing AGENTS.md")
			}
			want := fresh("core", coreBody)
			if existing != "" {
				want = existing + "\n" + want
			}
			if got := readFixture(t, dir, AgentsFile); got != want {
				t.Errorf("AGENTS.md:\n got %q\nwant %q", got, want)
			}
			if strings.Contains(readFixture(t, dir, AgentsFile), headTemplateBody) {
				t.Error("the head template was seeded over an existing AGENTS.md")
			}
		})
	}
}

// Without a head template the tool behaves exactly as it did before.
func TestInitWithoutHeadTemplate(t *testing.T) {
	dir := t.TempDir()

	res, err := Init(dir, initOptions(t))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.HeadSeeded {
		t.Error("HeadSeeded = true with no head.md in the template FS")
	}
	if got := readFixture(t, dir, AgentsFile); got != defaultRegions() {
		t.Errorf("AGENTS.md:\n got %q\nwant %q", got, defaultRegions())
	}
}

// Seeding happens once: the second init finds an AGENTS.md and leaves it be.
func TestInitSeedsHeadOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	opts := initOptionsWithHead(t)
	if _, err := Init(dir, opts); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	before := snapshot(t, dir)

	res, err := Init(dir, opts)
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if res.HeadSeeded {
		t.Error("HeadSeeded = true on the second init")
	}
	if len(res.Sync.Written) != 0 {
		t.Errorf("Sync.Written = %v, want nothing", res.Sync.Written)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("second Init changed bytes on disk:\nbefore %v\n after %v", before, after)
	}
}
