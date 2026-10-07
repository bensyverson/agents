package repo

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

// openWithSources opens dir against the fixture sources, fetching what the
// cache lacks the way `agents source add` does.
func openWithSources(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := Open(dir, testOptionsWith(t.TempDir(), FetchMissing))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return r
}

func pathOnlyManifest(pathDir string) string {
	return "sources:\n  - name: local\n    path: " + pathDir + "\nmodules:\n  - two\n"
}

// A git source is appended with its ref resolved on the remote to a sha, and no
// module is enabled.
func TestAddSourcePinsTheRemoteDefaultBranch(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	pathDir := newPathSource(t)
	dir := newRepo(t, map[string]string{manifest.FileName: pathOnlyManifest(pathDir)})
	r := openWithSources(t, dir)
	r.Seeds = SeedsCreated
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	before := snapshot(t, dir)

	res, err := r.AddSource(manifest.Source{Name: "house", Git: fx.url}, testOptionsWith(t.TempDir(), FetchMissing))
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	if res.Source.Ref != fx.sha {
		t.Errorf("pinned %q, want %q", res.Source.Ref, fx.sha)
	}
	if len(res.Removed) != 0 {
		t.Errorf("Removed = %v, want none: a manifest with sources has no embedded modules", res.Removed)
	}
	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if got, ok := m.SourceByName("house"); !ok || got.Git != fx.url || got.Ref != fx.sha {
		t.Errorf("house = %+v (found %v), want git %s at %s", got, ok, fx.url, fx.sha)
	}
	if got := m.Entries(); !slices.Equal(got, []string{"two"}) {
		t.Errorf("modules = %v, want them unchanged", got)
	}
	after := snapshot(t, dir)
	delete(after, manifest.FileName)
	delete(before, manifest.FileName)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("source add touched files other than the manifest:\nbefore %v\n after %v", before, after)
	}
	if len(r.Manifest.Sources) != 2 || r.Manifest.Sources[1].Name != "house" {
		t.Errorf("the open repo does not see the new source: %v", r.Manifest.Sources)
	}
}

// A ref the human gave is resolved too: the manifest records the sha.
func TestAddSourcePinsAGivenBranchToItsSha(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir := newRepo(t, map[string]string{manifest.FileName: pathOnlyManifest(newPathSource(t))})
	r := openWithSources(t, dir)

	res, err := r.AddSource(manifest.Source{Name: "house", Git: fx.url, Ref: "main"}, testOptionsWith(t.TempDir(), FetchMissing))
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if res.Source.Ref != fx.sha {
		t.Errorf("pinned %q, want %q", res.Source.Ref, fx.sha)
	}
	if got := readFixture(t, dir, manifest.FileName); strings.Contains(got, "ref: main") {
		t.Errorf("manifest still names the branch:\n%s", got)
	}
}

func TestAddSourceRefusesANameInUse(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	manifestBody := pathOnlyManifest(newPathSource(t))
	dir := newRepo(t, map[string]string{manifest.FileName: manifestBody})
	r := openWithSources(t, dir)

	_, err := r.AddSource(manifest.Source{Name: "local", Git: fx.url}, testOptionsWith(t.TempDir(), FetchMissing))

	if !errors.Is(err, manifest.ErrDuplicateSource) {
		t.Fatalf("AddSource error = %v, want ErrDuplicateSource", err)
	}
	if got := readFixture(t, dir, manifest.FileName); got != manifestBody {
		t.Errorf("manifest changed:\n%s", got)
	}
}

// A remote that cannot be reached leaves the manifest as it was.
func TestAddSourceWithAnUnreachableRemoteChangesNothing(t *testing.T) {
	hermeticGit(t)
	manifestBody := pathOnlyManifest(newPathSource(t))
	dir := newRepo(t, map[string]string{manifest.FileName: manifestBody})
	r := openWithSources(t, dir)

	_, err := r.AddSource(manifest.Source{Name: "house", Git: filepath.Join(t.TempDir(), "nowhere.git")}, testOptionsWith(t.TempDir(), FetchMissing))

	if err == nil {
		t.Fatal("AddSource returned a nil error for a remote that does not exist")
	}
	if got := readFixture(t, dir, manifest.FileName); got != manifestBody {
		t.Errorf("manifest changed:\n%s", got)
	}
}

func TestAddSourceRefusesAnInvalidName(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: pathOnlyManifest(newPathSource(t))})
	r := openWithSources(t, dir)

	_, err := r.AddSource(manifest.Source{Name: "Bad Name", Path: newPathSource(t)}, testOptionsWith(t.TempDir(), FetchMissing))

	if !errors.Is(err, manifest.ErrInvalidSourceName) {
		t.Fatalf("AddSource error = %v, want ErrInvalidSourceName", err)
	}
}

// Decision 2: the first real source replaces the embedded example modules,
// whose regions and files go exactly as `agents remove` would take them.
func TestAddSourceToAnEmbeddedRepoRemovesTheExampleModules(t *testing.T) {
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n  - delegation\n  - alpha\n",
	})
	r := mustOpenSeeding(t, dir)
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !fileExists(t, dir, delegationPath) || !fileExists(t, dir, alphaPath) {
		t.Fatal("fixture did not render the situational files")
	}
	pathDir := newPathSource(t)
	opts := testOptionsWith(t.TempDir(), FetchMissing)
	r, err := Open(dir, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	res, err := r.AddSource(manifest.Source{Name: "local", Path: pathDir}, opts)
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	if want := []string{"core", "delegation", "alpha"}; !slices.Equal(res.Removed, want) {
		t.Errorf("Removed = %v, want %v", res.Removed, want)
	}
	for _, rel := range []string{delegationPath, alphaPath} {
		if fileExists(t, dir, rel) {
			t.Errorf("%s survived the swap", rel)
		}
	}
	agents := readFixture(t, dir, AgentsFile)
	for _, gone := range []string{"agents:begin core", "agents:begin index", alphaWhen} {
		if strings.Contains(agents, gone) {
			t.Errorf("%s still holds %q:\n%s", AgentsFile, gone, agents)
		}
	}
	if !fileExists(t, dir, GotchasFile) {
		t.Errorf("%s was deleted; seeds belong to the repo once created", GotchasFile)
	}
	written := readFixture(t, dir, manifest.FileName)
	if !strings.Contains(written, "name: local") || strings.Contains(written, "core") {
		t.Errorf("manifest should name local and no module:\n%s", written)
	}
}

// The project-owned head above the first region is never part of the swap.
func TestAddSourceToAnEmbeddedRepoLeavesTheHeadAlone(t *testing.T) {
	head := "# Mine\n\nhand-written\n\n"
	dir := newRepo(t, map[string]string{
		manifest.FileName: "modules:\n  - core\n",
		AgentsFile:        head,
	})
	r := mustOpenSeeding(t, dir)
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	opts := testOptionsWith(t.TempDir(), FetchMissing)
	r, err := Open(dir, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if _, err := r.AddSource(manifest.Source{Name: "local", Path: newPathSource(t)}, opts); err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	if got := readFixture(t, dir, AgentsFile); !strings.HasPrefix(got, strings.TrimRight(head, "\n")) {
		t.Errorf("head changed:\n%s", got)
	}
}

// Decision 1: removing a source is refused while an enabled module comes from
// it, and the refusal names those modules.
func TestRemoveSourceRefusesWhileAModuleIsEnabled(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	manifestBody := sourcesManifest(fx, newPathSource(t), "  - one\n  - local/two\n")
	dir := newRepo(t, map[string]string{manifest.FileName: manifestBody})
	r := openWithSources(t, dir)

	_, err := r.RemoveSource("local")

	if !errors.Is(err, ErrSourceInUse) {
		t.Fatalf("RemoveSource error = %v, want ErrSourceInUse", err)
	}
	if !strings.Contains(err.Error(), "local/two") || strings.Contains(err.Error(), "one") {
		t.Errorf("error %q should name local/two and only the modules from local", err)
	}
	if got := readFixture(t, dir, manifest.FileName); got != manifestBody {
		t.Errorf("manifest changed:\n%s", got)
	}
}

// The refusal names modules as the manifest does: bare when they come from the
// default source, so the `agents remove` it suggests can be pasted as is.
func TestRemoveSourceNamesDefaultSourceModulesBare(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir := newRepo(t, map[string]string{manifest.FileName: sourcesManifest(fx, newPathSource(t), "  - one\n  - local/two\n")})

	_, err := openWithSources(t, dir).RemoveSource("house")

	if !errors.Is(err, ErrSourceInUse) {
		t.Fatalf("RemoveSource error = %v, want ErrSourceInUse", err)
	}
	if !strings.Contains(err.Error(), "supplies one;") || strings.Contains(err.Error(), "house/one") {
		t.Errorf("error %q should name one, bare", err)
	}
}

func TestRemoveSourceDropsAnUnusedSource(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir := newRepo(t, map[string]string{
		manifest.FileName: sourcesManifest(fx, newPathSource(t), "  - one\n"),
	})
	r := openWithSources(t, dir)
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	before := readFixture(t, dir, AgentsFile)

	res, err := r.RemoveSource("local")
	if err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}

	if res.Source.Name != "local" {
		t.Errorf("Source = %+v, want local", res.Source)
	}
	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if _, ok := m.SourceByName("local"); ok {
		t.Error("local is still in the manifest")
	}
	if _, ok := m.SourceByName("house"); !ok {
		t.Error("house left the manifest")
	}
	if got := readFixture(t, dir, AgentsFile); got != before {
		t.Errorf("%s changed:\n%s", AgentsFile, got)
	}
}

func TestRemoveSourceOfAnUnknownName(t *testing.T) {
	dir := newRepo(t, map[string]string{manifest.FileName: pathOnlyManifest(newPathSource(t))})
	r := openWithSources(t, dir)

	if _, err := r.RemoveSource("nope"); !errors.Is(err, manifest.ErrUnknownSource) {
		t.Fatalf("RemoveSource error = %v, want ErrUnknownSource", err)
	}
}

// Dropping the default source makes the next one the default: the entries that
// came from it are written bare, as the manifest's own rule has it.
func TestRemoveSourceTheDefaultMakesTheNextOneTheDefault(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir := newRepo(t, map[string]string{
		manifest.FileName: sourcesManifest(fx, newPathSource(t), "  - local/two\n"),
	})
	r := openWithSources(t, dir)

	if _, err := r.RemoveSource("house"); err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}

	m, err := manifest.Read(filepath.Join(dir, manifest.FileName))
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if m.DefaultSource() != "local" || !slices.Equal(m.Entries(), []string{"two"}) {
		t.Errorf("default %q entries %v, want local and [two]", m.DefaultSource(), m.Entries())
	}
}
