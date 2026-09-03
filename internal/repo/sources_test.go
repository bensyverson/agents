package repo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/source"
)

// The two modules the fixture sources hold. Each seeds a file of its own, so a
// seed created from the wrong source's templates is visible in the body.
const (
	gitModuleBody   = "rules from the git source\n"
	gitSeedFile     = "project/one.md"
	gitSeedBody     = "# One\n\nseeded from the git source\n"
	pathModuleBody  = "rules from the path source\n"
	pathSeedFile    = "project/two.md"
	pathSeedBody    = "# Two\n\nseeded from the path source\n"
	pathHeadBody    = "# Head\n\nfrom the path source\n"
	fixtureRegistry = "repos"
)

// hermeticGit keeps every git call in this package — the test's own and the
// loader's — away from the developer's identity and config.
func hermeticGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeTree lays out a source directory from source-relative path -> content.
func writeTree(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	for rel, body := range files {
		writeFixture(t, dir, rel, body)
	}
	return dir
}

// gitFixture is a bare repository holding one source, on branch main.
type gitFixture struct {
	url string
	sha string
}

func newGitSource(t *testing.T, files map[string]string) gitFixture {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, work, "init", "-b", "main")
	writeTree(t, work, files)
	git(t, work, "add", "-A")
	git(t, work, "commit", "-m", "one")
	sha := git(t, work, "rev-parse", "HEAD")
	bare := filepath.Join(root, "bare.git")
	git(t, root, "clone", "--bare", work, bare)
	return gitFixture{url: bare, sha: sha}
}

// gitSourceFiles is the git fixture's tree: one module and the template it seeds.
func gitSourceFiles() map[string]string {
	return map[string]string{
		"modules/one.md":            "---\nseeds: [" + gitSeedFile + "]\n---\n" + gitModuleBody,
		"templates/" + gitSeedFile:  gitSeedBody,
		"templates/" + HeadTemplate: "# Head\n\nfrom the git source\n",
	}
}

// newPathSource writes a source directory named "local" under a temp root, so
// the source's name is predictable.
func newPathSource(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "local")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return writeTree(t, dir, map[string]string{
		"modules/two.md":            "---\nseeds: [" + pathSeedFile + "]\n---\n" + pathModuleBody,
		"templates/" + pathSeedFile: pathSeedBody,
		"templates/" + HeadTemplate: pathHeadBody,
	})
}

// sourcesManifest is a manifest naming the git fixture as "house" (the default)
// and the path source as "local".
func sourcesManifest(fx gitFixture, pathDir, modules string) string {
	return "sources:\n" +
		"  - name: house\n    git: " + fx.url + "\n    ref: " + fx.sha + "\n" +
		"  - name: local\n    path: " + pathDir + "\n" +
		"modules:\n" + modules
}

func testOptionsWith(cache string, fetch FetchPolicy) Options {
	return Options{Loader: source.New(cache), Embedded: testSourceFS(), Fetch: fetch}
}

// A manifest may name a git source and a path source at once: both render, in
// manifest order, and each module's seed comes from its own source's templates.
func TestSyncRendersFromAGitAndAPathSource(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	pathDir := newPathSource(t)
	dir := newRepo(t, map[string]string{
		manifest.FileName: sourcesManifest(fx, pathDir, "  - one\n  - local/two\n"),
	})

	r, err := Open(dir, testOptionsWith(t.TempDir(), FetchMissing))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	r.Seeds = SeedsCreated
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got := readFixture(t, dir, AgentsFile)
	want := fresh("one", gitModuleBody) + "\n" + fresh("two", pathModuleBody)
	if got != want {
		t.Errorf("%s:\n got %q\nwant %q", AgentsFile, got, want)
	}
	if got := readFixture(t, dir, gitSeedFile); got != gitSeedBody {
		t.Errorf("%s = %q, want the git source's template %q", gitSeedFile, got, gitSeedBody)
	}
	if got := readFixture(t, dir, pathSeedFile); got != pathSeedBody {
		t.Errorf("%s = %q, want the path source's template %q", pathSeedFile, got, pathSeedBody)
	}
}

// A path source is relative to the repo that names it, so a checkout beside
// the repo can be pinned without an absolute path in a shared manifest.
func TestPathSourceIsRelativeToTheRepo(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, filepath.Join(dir, "rules"), map[string]string{
		"modules/two.md": pathModuleBody,
	})
	writeFixture(t, dir, manifest.FileName, "sources:\n  - name: local\n    path: rules\nmodules:\n  - two\n")

	r, err := Open(dir, testOptionsWith(t.TempDir(), FetchNever))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := r.Sync(false); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got, want := readFixture(t, dir, AgentsFile), fresh("two", pathModuleBody); got != want {
		t.Errorf("%s:\n got %q\nwant %q", AgentsFile, got, want)
	}
}

// The read-only verbs never touch the network: an uncached git source is one
// error naming the source and the verb that fixes it, with no git call at all.
func TestOpenWithoutTheCacheRefusesAndRunsNoGit(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir := newRepo(t, map[string]string{
		manifest.FileName: sourcesManifest(fx, newPathSource(t), "  - one\n"),
	})
	// An empty PATH makes any git call a different, recognisable failure.
	t.Setenv("PATH", "")

	_, err := Open(dir, testOptionsWith(t.TempDir(), FetchNever))
	if err == nil {
		t.Fatal("Open = nil error with no cache, want the not-fetched refusal")
	}
	if !errors.Is(err, source.ErrNotFetched) {
		t.Errorf("Open error = %v, want it to wrap source.ErrNotFetched", err)
	}
	for _, want := range []string{"house", "agents sync"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("error is more than one line: %q", err)
	}
}

// sync, add and init may populate the cache; a second open needs no git at all.
func TestOpenFetchesAMissingGitSourceThenReadsTheCache(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir := newRepo(t, map[string]string{
		manifest.FileName: sourcesManifest(fx, newPathSource(t), "  - one\n"),
	})
	cache := t.TempDir()

	if _, err := Open(dir, testOptionsWith(cache, FetchMissing)); err != nil {
		t.Fatalf("Open with FetchMissing: %v", err)
	}

	t.Setenv("PATH", "")
	r, err := Open(dir, testOptionsWith(cache, FetchNever))
	if err != nil {
		t.Fatalf("Open from the cache: %v", err)
	}
	if got := r.Sources["house"].Source.Commit; got != fx.sha {
		t.Errorf("house commit = %q, want %q", got, fx.sha)
	}
}

// Decision 7: init writes the sha it resolved, so the manifest is a pin even
// when the human asked for a branch.
func TestInitWritesTheResolvedShaForABranch(t *testing.T) {
	hermeticGit(t)
	fx := newGitSource(t, gitSourceFiles())
	dir := t.TempDir()

	res, err := Init(dir, InitOptions{
		Modules:      []string{"one"},
		Source:       manifest.Source{Name: "house", Git: fx.url, Ref: "main"},
		Open:         testOptionsWith(t.TempDir(), FetchMissing),
		RegistryPath: filepath.Join(t.TempDir(), fixtureRegistry),
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.Source.Ref != fx.sha {
		t.Errorf("InitResult.Source.Ref = %q, want the resolved sha %q", res.Source.Ref, fx.sha)
	}
	written := readFixture(t, dir, manifest.FileName)
	if !strings.Contains(written, "ref: "+fx.sha) {
		t.Errorf("%s does not pin the sha:\n%s", manifest.FileName, written)
	}
	if strings.Contains(written, "ref: main") {
		t.Errorf("%s still names the branch:\n%s", manifest.FileName, written)
	}
	if got, want := readFixture(t, dir, AgentsFile), fresh("one", gitModuleBody); !strings.Contains(got, want) {
		t.Errorf("%s does not hold the module's region:\n%s", AgentsFile, got)
	}
	if got := readFixture(t, dir, gitSeedFile); got != gitSeedBody {
		t.Errorf("%s = %q, want the git source's template", gitSeedFile, got)
	}
}

// With no source named, init writes no sources: block at all and renders the
// modules the binary carries.
func TestInitWithNoSourceWritesNoSourcesBlock(t *testing.T) {
	dir := t.TempDir()

	res, err := Init(dir, initOptions(t, "core"))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if res.Source != (manifest.Source{}) {
		t.Errorf("InitResult.Source = %+v, want the zero value", res.Source)
	}
	written := readFixture(t, dir, manifest.FileName)
	if strings.Contains(written, "sources:") {
		t.Errorf("%s names a source:\n%s", manifest.FileName, written)
	}
	if got, want := readFixture(t, dir, AgentsFile), fresh("core", coreBody); !strings.Contains(got, want) {
		t.Errorf("%s does not hold the embedded module's region:\n%s", AgentsFile, got)
	}
}

// A directory that exists is a path source; anything else is a git URL. The
// name is the last segment, without any .git.
func TestParseSource(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "house-rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		spec string
		want manifest.Source
	}{
		{dir, manifest.Source{Name: "house-rules", Path: dir}},
		{"https://example.com/team/house.git", manifest.Source{Name: "house", Git: "https://example.com/team/house.git"}},
		{"git@example.com:team/house-rules.git", manifest.Source{Name: "house-rules", Git: "git@example.com:team/house-rules.git"}},
		{"https://example.com/team/house", manifest.Source{Name: "house", Git: "https://example.com/team/house"}},
	} {
		got, err := ParseSource(tc.spec)
		if err != nil {
			t.Errorf("ParseSource(%q): %v", tc.spec, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseSource(%q) = %+v, want %+v", tc.spec, got, tc.want)
		}
	}
}
