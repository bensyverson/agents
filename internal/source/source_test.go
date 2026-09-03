package source_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/source"
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

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture is a bare repo holding two commits: the first tagged v1 with
// "core v1", the second on main with "core v2".
type fixture struct {
	bare string
	v1   string
	v2   string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, work, "init", "-b", "main")
	write(t, filepath.Join(work, "modules", "core.md"), "core v1\n")
	write(t, filepath.Join(work, "templates", "head.md"), "seed\n")
	git(t, work, "add", "-A")
	git(t, work, "commit", "-m", "one")
	git(t, work, "tag", "v1")
	v1 := git(t, work, "rev-parse", "HEAD")

	write(t, filepath.Join(work, "modules", "core.md"), "core v2\n")
	git(t, work, "add", "-A")
	git(t, work, "commit", "-m", "two")
	v2 := git(t, work, "rev-parse", "HEAD")

	bare := filepath.Join(root, "bare.git")
	git(t, root, "clone", "--bare", work, bare)
	return fixture{bare: bare, v1: v1, v2: v2}
}

func read(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	body, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(body)
}

func TestFetchAndLoadAtTag(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())

	sha, err := loader.Fetch(fx.bare, "v1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if sha != fx.v1 {
		t.Errorf("Fetch sha = %s, want %s", sha, fx.v1)
	}

	src, err := loader.Git("house", fx.bare, "v1")
	if err != nil {
		t.Fatalf("Git: %v", err)
	}
	if src.Name != "house" {
		t.Errorf("Name = %q, want %q", src.Name, "house")
	}
	if src.Commit != fx.v1 {
		t.Errorf("Commit = %s, want %s", src.Commit, fx.v1)
	}
	if got := read(t, src.Modules, "core.md"); got != "core v1\n" {
		t.Errorf("modules/core.md = %q, want %q", got, "core v1\n")
	}
	if got := read(t, src.Templates, "head.md"); got != "seed\n" {
		t.Errorf("templates/head.md = %q, want %q", got, "seed\n")
	}
}

func TestFetchAndLoadAtBranch(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())

	sha, err := loader.Fetch(fx.bare, "main")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if sha != fx.v2 {
		t.Errorf("Fetch sha = %s, want %s", sha, fx.v2)
	}
	src, err := loader.Git("house", fx.bare, "main")
	if err != nil {
		t.Fatalf("Git: %v", err)
	}
	if got := read(t, src.Modules, "core.md"); got != "core v2\n" {
		t.Errorf("modules/core.md = %q, want %q", got, "core v2\n")
	}
}

func TestFetchAndLoadAtSha(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())

	if _, err := loader.Fetch(fx.bare, fx.v1); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	src, err := loader.Git("house", fx.bare, fx.v1)
	if err != nil {
		t.Fatalf("Git: %v", err)
	}
	if src.Commit != fx.v1 {
		t.Errorf("Commit = %s, want %s", src.Commit, fx.v1)
	}
	if got := read(t, src.Modules, "core.md"); got != "core v1\n" {
		t.Errorf("modules/core.md = %q, want %q", got, "core v1\n")
	}
}

// A ref fetched under one name is loadable by its sha, which is what `agents
// update` writes back into the manifest.
func TestLoadByShaAfterFetchingBranch(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())

	if _, err := loader.Fetch(fx.bare, "main"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	src, err := loader.Git("house", fx.bare, fx.v2)
	if err != nil {
		t.Fatalf("Git by sha: %v", err)
	}
	if got := read(t, src.Modules, "core.md"); got != "core v2\n" {
		t.Errorf("modules/core.md = %q, want %q", got, "core v2\n")
	}
}

func TestCacheLayout(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	cache := t.TempDir()
	loader := source.New(cache)
	if _, err := loader.Fetch(fx.bare, "v1"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	sum := sha256.Sum256([]byte(fx.bare))
	want := filepath.Join(cache, "sources", hex.EncodeToString(sum[:]))
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("expected a source directory at %s: %v", want, err)
	}
}

// The cache is authoritative: once fetched, loading must not shell out to git.
// Both the fixture repo and git itself are taken away to prove it.
func TestLoadFromCacheWithoutGit(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())
	if _, err := loader.Fetch(fx.bare, "v1"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := os.RemoveAll(fx.bare); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())

	src, err := loader.Git("house", fx.bare, "v1")
	if err != nil {
		t.Fatalf("Git without git on PATH: %v", err)
	}
	if got := read(t, src.Modules, "core.md"); got != "core v1\n" {
		t.Errorf("modules/core.md = %q, want %q", got, "core v1\n")
	}
}

func TestLoadNotFetched(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())

	_, err := loader.Git("house", fx.bare, "v1")
	if !errors.Is(err, source.ErrNotFetched) {
		t.Fatalf("Git before fetch: err = %v, want ErrNotFetched", err)
	}
	if !strings.Contains(err.Error(), "house") {
		t.Errorf("error %q does not name the source", err)
	}
}

// A ref that was never fetched is not-fetched even when the repo is cached.
func TestLoadUnfetchedRefInFetchedRepo(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())
	if _, err := loader.Fetch(fx.bare, "v1"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := loader.Git("house", fx.bare, "main"); !errors.Is(err, source.ErrNotFetched) {
		t.Fatalf("Git at an unfetched ref: err = %v, want ErrNotFetched", err)
	}
}

// RemoteHeadRef asks the remote itself, so it follows a default branch that
// moved after any cache was populated — what `agents update` pins by default.
func TestRemoteHeadRef(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)

	if got, err := source.RemoteHeadRef(fx.bare); err != nil || got != "refs/heads/main" {
		t.Fatalf("RemoteHeadRef = %q, %v, want refs/heads/main", got, err)
	}

	// The source makes another branch its default.
	git(t, fx.bare, "branch", "next", "main")
	git(t, fx.bare, "symbolic-ref", "HEAD", "refs/heads/next")

	if got, err := source.RemoteHeadRef(fx.bare); err != nil || got != "refs/heads/next" {
		t.Fatalf("RemoteHeadRef after the default branch moved = %q, %v, want refs/heads/next", got, err)
	}
}

// A remote that answers nothing at all is an error naming it, not an empty ref
// that would quietly resolve to something else.
func TestRemoteHeadRefOfANonRepository(t *testing.T) {
	hermeticGit(t)

	if got, err := source.RemoteHeadRef(t.TempDir()); err == nil {
		t.Fatalf("RemoteHeadRef of a directory that is no repository = %q, want an error", got)
	}
}

func TestResolve(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())

	for _, tc := range []struct{ ref, want string }{
		{"main", fx.v2},
		{"v1", fx.v1},
		{fx.v1, fx.v1},
		{fx.v1[:8], fx.v1},
		{"", fx.v2},
	} {
		got, err := loader.Resolve(fx.bare, tc.ref)
		if err != nil {
			t.Errorf("Resolve(%q): %v", tc.ref, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Resolve(%q) = %s, want %s", tc.ref, got, tc.want)
		}
	}
}

func TestResolveUnknownRef(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	loader := source.New(t.TempDir())

	_, err := loader.Resolve(fx.bare, "no-such-ref")
	if !errors.Is(err, source.ErrUnknownRef) {
		t.Fatalf("Resolve of an unknown ref: err = %v, want ErrUnknownRef", err)
	}
	if !strings.Contains(err.Error(), "no-such-ref") {
		t.Errorf("error %q does not name the ref", err)
	}
}

func TestPathSource(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "modules", "core.md"), "local\n")
	write(t, filepath.Join(dir, "templates", "head.md"), "seed\n")

	cache := filepath.Join(t.TempDir(), "cache")
	loader := source.New(cache)

	src, err := loader.Path("local", dir)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if src.Commit != "" {
		t.Errorf("Commit = %q, want empty for a path source", src.Commit)
	}
	if got := read(t, src.Modules, "core.md"); got != "local\n" {
		t.Errorf("modules/core.md = %q, want %q", got, "local\n")
	}
	if got := read(t, src.Templates, "head.md"); got != "seed\n" {
		t.Errorf("templates/head.md = %q, want %q", got, "seed\n")
	}
	if _, err := os.Stat(cache); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a path source touched the cache at %s: %v", cache, err)
	}
}

func TestPathSourceWithoutModules(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "templates", "head.md"), "seed\n")

	loader := source.New(t.TempDir())
	_, err := loader.Path("local", dir)
	if !errors.Is(err, source.ErrNoModules) {
		t.Fatalf("Path without modules/: err = %v, want ErrNoModules", err)
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("error %q does not name the source", err)
	}
}

func TestFromFS(t *testing.T) {
	root := fstest.MapFS{
		"modules/core.md":    {Data: []byte("embedded\n")},
		"templates/head.md":  {Data: []byte("seed\n")},
		"modules/other.md":   {Data: []byte("other\n")},
		"README.md":          {Data: []byte("ignored\n")},
		"modules/sub/why.md": {Data: []byte("nested\n")},
	}
	src, err := source.FromFS("example", root)
	if err != nil {
		t.Fatalf("FromFS: %v", err)
	}
	if src.Name != "example" {
		t.Errorf("Name = %q, want %q", src.Name, "example")
	}
	if src.Commit != "" {
		t.Errorf("Commit = %q, want empty for an embedded source", src.Commit)
	}
	if got := read(t, src.Modules, "core.md"); got != "embedded\n" {
		t.Errorf("modules/core.md = %q, want %q", got, "embedded\n")
	}
	if got := read(t, src.Templates, "head.md"); got != "seed\n" {
		t.Errorf("templates/head.md = %q, want %q", got, "seed\n")
	}
}

func TestFromFSWithoutModules(t *testing.T) {
	_, err := source.FromFS("example", fstest.MapFS{"templates/head.md": {Data: []byte("seed\n")}})
	if !errors.Is(err, source.ErrNoModules) {
		t.Fatalf("FromFS without modules/: err = %v, want ErrNoModules", err)
	}
}

// templates/ is optional: git cannot track an empty directory, so a source
// with no seeds has none.
func TestSourceWithoutTemplates(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "modules", "core.md"), "local\n")

	src, err := source.New(t.TempDir()).Path("local", dir)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if _, err := fs.ReadFile(src.Templates, "head.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("reading a seed from a source with no templates/: err = %v, want fs.ErrNotExist", err)
	}
}

func TestDefaultCacheDir(t *testing.T) {
	t.Run("AGENTS_CACHE wins", func(t *testing.T) {
		t.Setenv("AGENTS_CACHE", "/tmp/explicit")
		t.Setenv("XDG_CACHE_HOME", "/tmp/xdg")
		got, err := source.DefaultCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		if got != "/tmp/explicit" {
			t.Errorf("DefaultCacheDir = %q, want %q", got, "/tmp/explicit")
		}
	})

	t.Run("XDG_CACHE_HOME next", func(t *testing.T) {
		t.Setenv("AGENTS_CACHE", "")
		t.Setenv("XDG_CACHE_HOME", "/tmp/xdg")
		got, err := source.DefaultCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join("/tmp/xdg", "agents"); got != want {
			t.Errorf("DefaultCacheDir = %q, want %q", got, want)
		}
	})

	t.Run("user cache dir last", func(t *testing.T) {
		t.Setenv("AGENTS_CACHE", "")
		t.Setenv("XDG_CACHE_HOME", "")
		base, err := os.UserCacheDir()
		if err != nil {
			t.Skipf("no user cache dir: %v", err)
		}
		got, err := source.DefaultCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(base, "agents"); got != want {
			t.Errorf("DefaultCacheDir = %q, want %q", got, want)
		}
	})
}
