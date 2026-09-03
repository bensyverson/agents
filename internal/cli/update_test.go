package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/source"
)

// The git fixture's module, before and after the commit `update` moves to.
const (
	gitCoreBody = "core rules from the source\n"
	movedCore   = "core rules from the source, revised\n"
	// gitSourceName is the name every manifest here gives the git fixture.
	gitSourceName = "house"
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

func gitIn(t *testing.T, dir string, args ...string) string {
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

// newGitFixture is a bare repository holding one source on branch main.
func newGitFixture(t *testing.T) (url, sha string) {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, work, "init", "-b", "main")
	writeSourceFile(t, work, "modules/core.md", gitCoreBody)
	gitIn(t, work, "add", "-A")
	gitIn(t, work, "commit", "-m", "one")
	sha = gitIn(t, work, "rev-parse", "HEAD")
	url = filepath.Join(root, "bare.git")
	gitIn(t, root, "clone", "--bare", work, url)
	return url, sha
}

// pushModule commits a new body for the source's core module and returns the
// commit the source's default branch now names.
func pushModule(t *testing.T, url, body string) string {
	t.Helper()
	root := t.TempDir()
	gitIn(t, root, "clone", url, "work")
	work := filepath.Join(root, "work")
	writeSourceFile(t, work, "modules/core.md", body)
	gitIn(t, work, "add", "-A")
	gitIn(t, work, "commit", "-m", "two")
	gitIn(t, work, "push", "origin", "HEAD:main")
	return gitIn(t, work, "rev-parse", "HEAD")
}

// managedRepo makes a repo pinned to the git source at sha, renders it and
// registers it, and leaves the working directory there. init is what registers
// it; the manifest is written first because a local bare repository is a
// directory, which `--source` would read as a path source.
func managedRepo(t *testing.T, url, sha string) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	writeSourceFile(t, dir, manifest.FileName,
		"sources:\n  - name: "+gitSourceName+"\n    git: "+url+"\n    ref: "+sha+"\nmodules:\n  - core\n")
	if _, stderr, err := run(t, "init", "--with", "core"); err != nil {
		t.Fatalf("init: %v (stderr %s)", err, stderr)
	}
	return dir
}

// gitFleet points the registry and the cache at temp directories and returns a
// git source with no repo attached yet.
func gitFleet(t *testing.T) (url, sha string) {
	t.Helper()
	hermeticGit(t)
	t.Setenv(registryEnvVar, filepath.Join(t.TempDir(), "repos"))
	t.Setenv(source.CacheEnv, t.TempDir())
	return newGitFixture(t)
}

// update prints one header line per source that moved and the diff of every
// enabled module whose body changed, then the file it re-rendered.
func TestUpdateMovesThePinAndPrintsTheDiff(t *testing.T) {
	url, sha := gitFleet(t)
	dir := managedRepo(t, url, sha)
	newSha := pushModule(t, url, movedCore)

	stdout, stderr, err := run(t, "update")
	if err != nil {
		t.Fatalf("update: %v (stderr %s)", err, stderr)
	}

	header := gitSourceName + ": " + sha[:7] + " -> " + newSha[:7]
	if !strings.Contains(stdout, header) {
		t.Errorf("output has no %q header:\n%s", header, stdout)
	}
	for _, want := range []string{
		"-" + strings.TrimSuffix(gitCoreBody, "\n"),
		"+" + strings.TrimSuffix(movedCore, "\n"),
		"rendered AGENTS.md",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
	if written := readFile(t, filepath.Join(dir, manifest.FileName)); !strings.Contains(written, "ref: "+newSha) {
		t.Errorf("%s is not pinned to the new sha:\n%s", manifest.FileName, written)
	}
	if got := readFile(t, filepath.Join(dir, "AGENTS.md")); !strings.Contains(got, movedCore) {
		t.Errorf("AGENTS.md does not hold the new body:\n%s", got)
	}
}

// A source already at its pinned commit is silent and exits 0.
func TestUpdateOnACurrentSourcePrintsNothing(t *testing.T) {
	url, sha := gitFleet(t)
	managedRepo(t, url, sha)

	stdout, stderr, err := run(t, "update")
	if err != nil {
		t.Fatalf("update: %v (stderr %s)", err, stderr)
	}
	if stdout != "" {
		t.Errorf("update printed %q, want silence", stdout)
	}
}

// --all reports under each repo's path and exits non-zero when any repo
// refused to overwrite a hand-edited region; the other repo is still updated.
func TestUpdateAllReportsPerRepoAndFailsOnARefusal(t *testing.T) {
	url, sha := gitFleet(t)
	edited := managedRepo(t, url, sha)
	handEditRegion(t, edited, gitCoreBody)
	clean := managedRepo(t, url, sha)
	newSha := pushModule(t, url, movedCore)

	stdout, _, err := run(t, "update", "--all")
	if err == nil {
		t.Fatal("update --all returned nil with a refusing repo; the process would exit 0")
	}
	for _, want := range []string{edited, clean, "hand-edited", sha[:7] + " -> " + newSha[:7]} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
	if written := readFile(t, filepath.Join(clean, manifest.FileName)); !strings.Contains(written, "ref: "+newSha) {
		t.Errorf("the clean repo was not updated:\n%s", written)
	}
	if got := readFile(t, filepath.Join(clean, "AGENTS.md")); !strings.Contains(got, movedCore) {
		t.Errorf("the clean repo did not re-render:\n%s", got)
	}
}

// handEditRegion changes a rendered region's body, so its marker hash no
// longer matches what it holds.
func handEditRegion(t *testing.T, repoDir, body string) {
	t.Helper()
	path := filepath.Join(repoDir, "AGENTS.md")
	current := readFile(t, path)
	edited := strings.Replace(current, body, body+"AND MY OWN LINE\n", 1)
	if edited == current {
		t.Fatalf("fixture did not apply; AGENTS.md is:\n%s", current)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
}
