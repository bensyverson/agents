package repo

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	agents "github.com/bensyverson/agents"
	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// A declared seed the repo does not have yet is created from its template.
func TestSyncSeedsAMissingFile(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n",
		AgentsFile:        fresh("core", coreBody),
	})

	res := mustSync(t, mustOpenSeeding(t, dir), false)

	if want := []string{GotchasFile}; !reflect.DeepEqual(res.Seeded, want) {
		t.Errorf("Seeded = %v, want %v", res.Seeded, want)
	}
	if got := readFixture(t, dir, GotchasFile); got != templateBody {
		t.Errorf("%s = %q, want the template %q", GotchasFile, got, templateBody)
	}
}

// Every seed of every module in the manifest, in manifest order.
func TestSyncSeedsEveryDeclaredFile(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n  - docs\n",
		AgentsFile:        fresh("core", coreBody) + "\n" + fresh("docs", docsBody),
	})

	res := mustSync(t, mustOpenSeeding(t, dir), false)

	if want := []string{GotchasFile, backlogFile}; !reflect.DeepEqual(res.Seeded, want) {
		t.Errorf("Seeded = %v, want %v", res.Seeded, want)
	}
	if got := readFixture(t, dir, backlogFile); got != backlogTemplateBody {
		t.Errorf("%s = %q, want the template %q", backlogFile, got, backlogTemplateBody)
	}
}

// A seeded file belongs to the repo the moment it exists: sync never rewrites it.
func TestSyncLeavesAnExistingSeedUntouched(t *testing.T) {
	const ours = "# Gotchas\n\nours, hand-written\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n",
		AgentsFile:        fresh("core", coreBody),
		GotchasFile:       ours,
	})
	before := snapshot(t, dir)

	res := mustSync(t, mustOpenSeeding(t, dir), false)

	if len(res.Seeded) != 0 {
		t.Errorf("Seeded = %v, want nothing over an existing file", res.Seeded)
	}
	if got := readFixture(t, dir, GotchasFile); got != ours {
		t.Errorf("%s = %q, want it byte-identical to %q", GotchasFile, got, ours)
	}
	if after := snapshot(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("sync changed bytes on disk:\nbefore %v\n after %v", before, after)
	}
}

// A manifest whose modules declare no seeds writes nothing extra.
func TestSyncWithoutDeclaredSeeds(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - principles\n",
		AgentsFile:        fresh("principles", principlesBody),
	})

	res := mustSync(t, mustOpenSeeding(t, dir), false)

	if len(res.Seeded) != 0 {
		t.Errorf("Seeded = %v, want nothing", res.Seeded)
	}
	if fileExists(t, dir, GotchasFile) {
		t.Errorf("%s was created by a module that does not declare it", GotchasFile)
	}
}

// A seed with no template body is a packaging mistake, not a silent no-op.
func TestSyncMissingTemplateIsAnError(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n",
		AgentsFile:        fresh("core", coreBody),
	})
	source := testSourceFS()
	delete(source, "templates/"+GotchasFile)
	r, err := Open(dir, Options{Embedded: source})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	r.Seeds = SeedsCreated

	_, err = r.Sync(false)
	if err == nil {
		t.Fatal("Sync = nil error, want one naming the missing template")
	}
	for _, want := range []string{"core", GotchasFile} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if fileExists(t, dir, GotchasFile) {
		t.Errorf("%s was created from a missing template", GotchasFile)
	}
}

// Seeding is opt-in: a Repo left at SeedsSkipped renders and nothing more,
// which is what the read-only verbs open.
func TestSyncWithSeedsSkippedSeedsNothing(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n",
		AgentsFile:        fresh("core", coreBody),
	})

	res := mustSync(t, mustOpen(t, dir), false)

	if len(res.Seeded) != 0 {
		t.Errorf("Seeded = %v, want nothing with SeedsSkipped", res.Seeded)
	}
	if fileExists(t, dir, GotchasFile) {
		t.Errorf("%s was created with SeedsSkipped", GotchasFile)
	}
}

// An existing repo picks up a seed a module gained after it was initialised.
func TestSyncSeedsOnASecondRun(t *testing.T) {
	dir := t.TempDir()
	opts := initOptions(t, "core")
	if _, err := Init(dir, opts); err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeFixture(t, dir, manifest.FileName, "modules:\n  - core\n  - docs\n")

	r := mustOpenSeeding(t, dir)
	res := mustSync(t, r, false)

	if want := []string{backlogFile}; !reflect.DeepEqual(res.Seeded, want) {
		t.Errorf("Seeded = %v, want %v (gotchas is already there)", res.Seeded, want)
	}
}

// Packaging invariant: every seed the shipped modules declare must have a
// template in the shipped templates, or the repos that enable those modules get
// an error instead of a file.
func TestEmbeddedSeedsHaveTemplates(t *testing.T) {
	set, err := module.Load(agents.Modules())
	if err != nil {
		t.Fatalf("module.Load(embedded): %v", err)
	}
	templates := agents.Templates()
	for _, name := range set.Names() {
		m, _ := set.Get(name)
		for _, rel := range m.Seeds {
			if _, err := fs.ReadFile(templates, rel); err != nil {
				t.Errorf("module %s seeds %s: no templates/%s (%v)", name, rel, rel, err)
			}
		}
	}
}

// Init seeds through the same path, and reports what it created.
func TestInitSeedsDeclaredFiles(t *testing.T) {
	dir := t.TempDir()

	res, err := Init(dir, initOptions(t, "core", "docs"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if want := []string{GotchasFile, backlogFile}; !reflect.DeepEqual(res.Sync.Seeded, want) {
		t.Errorf("Sync.Seeded = %v, want %v", res.Sync.Seeded, want)
	}
	if got := readFixture(t, dir, GotchasFile); got != templateBody {
		t.Errorf("%s = %q, want the template", GotchasFile, got)
	}
	if got := readFixture(t, dir, backlogFile); got != backlogTemplateBody {
		t.Errorf("%s = %q, want the template", backlogFile, got)
	}
}
