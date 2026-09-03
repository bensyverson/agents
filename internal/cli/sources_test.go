package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/source"
)

// emptyRepo is a working directory with nothing in it, with the registry and
// the source cache pointed at temp files.
func emptyRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(registryEnvVar, filepath.Join(t.TempDir(), "repos"))
	t.Setenv(source.CacheEnv, t.TempDir())
	t.Chdir(dir)
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// With neither --source nor --with, init writes no sources: block, enables the
// two modules the binary carries, and says where other sources go.
func TestInitWithNoSourceUsesTheEmbeddedModules(t *testing.T) {
	dir := emptyRepo(t)

	stdout, stderr, err := run(t, "init")
	if err != nil {
		t.Fatalf("init: %v (stderr %s)", err, stderr)
	}

	written := readFile(t, filepath.Join(dir, manifest.FileName))
	if want := "modules:\n  - agents\n  - module-authoring\n"; written != want {
		t.Errorf("%s:\n got %q\nwant %q", manifest.FileName, written, want)
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "AGENTS.md")), "agents:begin agents@") {
		t.Errorf("AGENTS.md holds no agents region:\n%s", readFile(t, filepath.Join(dir, "AGENTS.md")))
	}
	// One line saying which source was used and where to name others.
	if !strings.Contains(stdout, "built into the binary") || !strings.Contains(stdout, manifest.FileName) {
		t.Errorf("init did not say it used the embedded source:\n%s", stdout)
	}
}

// The defaults name the embedded example modules, so they are meaningless for
// a source of someone else's: --source without --with is one line of refusal,
// and nothing is written.
func TestInitWithASourceRequiresWith(t *testing.T) {
	dir := emptyRepo(t)
	sourceDir := newSource(t, sourceName, map[string]string{"modules/core.md": coreBody})

	stdout, _, err := run(t, "init", "--source", sourceDir)

	if err == nil {
		t.Fatal("init --source with no --with returned a nil error")
	}
	const want = "--with is required with --source: name the modules to enable (its modules/*.md)"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if stdout != "" {
		t.Errorf("init printed %q, want the failure on stderr alone", stdout)
	}
	if _, statErr := os.Stat(filepath.Join(dir, manifest.FileName)); statErr == nil {
		t.Errorf("init wrote %s despite refusing", manifest.FileName)
	}
}

// The help text must name the defaults, since they are what init uses when the
// flag is absent.
func TestInitHelpNamesTheDefaultModules(t *testing.T) {
	stdout, _, err := run(t, "init", "--help")
	if err != nil {
		t.Fatalf("init --help: %v", err)
	}
	if !strings.Contains(stdout, "agents,module-authoring") {
		t.Errorf("init --help does not name the default modules:\n%s", stdout)
	}
}

// A directory that exists is a path source, named after its last segment.
func TestInitWithADirectorySourceWritesAPathEntry(t *testing.T) {
	dir := emptyRepo(t)
	sourceDir := newSource(t, sourceName, map[string]string{"modules/core.md": coreBody})

	if _, stderr, err := run(t, "init", "--source", sourceDir, "--with", "core"); err != nil {
		t.Fatalf("init: %v (stderr %s)", err, stderr)
	}

	written := readFile(t, filepath.Join(dir, manifest.FileName))
	for _, want := range []string{"sources:", "name: " + sourceName, "path: " + sourceDir} {
		if !strings.Contains(written, want) {
			t.Errorf("%s missing %q:\n%s", manifest.FileName, want, written)
		}
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "AGENTS.md")), coreBody) {
		t.Error("AGENTS.md does not hold the source's module")
	}
}

// The read-only verbs run offline: a source the cache lacks is one line naming
// the source and the verb that fixes it, and no git runs at all.
func TestCheckWithoutTheCacheSaysRunSyncAndRunsNoGit(t *testing.T) {
	dir := emptyRepo(t)
	writeSourceFile(t, dir, manifest.FileName,
		"sources:\n  - name: house\n    git: https://example.invalid/house.git\n    ref: main\nmodules:\n  - core\n")
	// An empty PATH turns any git call into a different, recognisable failure.
	t.Setenv("PATH", "")

	stdout, _, err := run(t, "check")
	if err == nil {
		t.Fatal("check returned nil with an unfetched source; the process would exit 0")
	}
	for _, want := range []string{"house", "agents sync"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("error is more than one line: %q", err)
	}
	if stdout != "" {
		t.Errorf("check printed %q, want the failure on stderr alone", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
		t.Error("check created AGENTS.md")
	}
}

// twoSourceRepo is a repo whose manifest names the fixture source and a second
// one holding a module of its own.
func twoSourceRepo(t *testing.T) (repoDir, sourceDir, otherDir string) {
	t.Helper()
	repoDir, sourceDir, _ = fixture(t)
	initRepo(t, sourceDir)
	otherDir = newSource(t, "other", map[string]string{"modules/extra.md": docsBody})
	writeSourceFile(t, repoDir, manifest.FileName,
		"sources:\n  - name: "+sourceName+"\n    path: "+sourceDir+"\n  - name: other\n    path: "+otherDir+"\n"+
			"modules:\n  - core\n  - principles\n")
	return repoDir, sourceDir, otherDir
}

// add takes a bare name against the default source and a qualified one against
// any other; the manifest keeps each entry in the form that names its source.
func TestAddResolvesBareAndQualifiedNames(t *testing.T) {
	repoDir, sourceDir, _ := twoSourceRepo(t)
	writeSourceFile(t, sourceDir, "modules/docs.md", docsBody)

	stdout, _, err := run(t, "add", "docs", "other/extra")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, want := range []string{"enabled docs", "enabled other/extra"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("add output missing %q:\n%s", want, stdout)
		}
	}
	written := readFile(t, filepath.Join(repoDir, manifest.FileName))
	for _, want := range []string{"\n  - docs\n", "\n  - other/extra\n"} {
		if !strings.Contains(written, want) {
			t.Errorf("%s missing %q:\n%s", manifest.FileName, want, written)
		}
	}
	// A bare name that only the other source has is not the default source's.
	if _, _, err := run(t, "add", "extra"); err == nil {
		t.Error("add of a bare name from a non-default source returned nil error")
	}
}

// list prints the qualified name of a module from any source but the default.
func TestListQualifiesModulesFromOtherSources(t *testing.T) {
	twoSourceRepo(t)

	stdout, _, err := run(t, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(stdout, "other/extra") {
		t.Errorf("list does not qualify the other source's module:\n%s", stdout)
	}
	if !strings.Contains(stdout, "* core") {
		t.Errorf("list does not mark the enabled default-source module:\n%s", stdout)
	}
	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.Contains(line, sourceName+"/") {
			t.Errorf("list qualified a module of the default source: %q", line)
		}
	}
}

// --modules is retired: a path source replaces it, and no verb still offers it.
func TestNoVerbHasAModulesFlag(t *testing.T) {
	emptyRepo(t)
	for _, verb := range []string{"", "init", "sync", "check", "diff", "status", "add", "remove", "list"} {
		args := []string{"--help"}
		if verb != "" {
			args = []string{verb, "--help"}
		}
		stdout, _, err := run(t, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if strings.Contains(stdout, "--modules") {
			t.Errorf("`agents %s --help` still offers --modules:\n%s", verb, stdout)
		}
	}
}

// --all must not mistake an unfetched source for a stale registry entry: a
// pre-commit `check --all` on a fresh machine has to fail, not warn and pass.
func TestCheckAllWithoutTheCacheFails(t *testing.T) {
	dir := emptyRepo(t)
	writeSourceFile(t, dir, manifest.FileName,
		"sources:\n  - name: house\n    git: https://example.invalid/house.git\n    ref: main\nmodules:\n  - core\n")
	registry := filepath.Join(t.TempDir(), "repos")
	writeSourceFile(t, filepath.Dir(registry), "repos", dir+"\n")
	t.Setenv(registryEnvVar, registry)
	t.Setenv("PATH", "")

	_, _, err := run(t, "check", "--all")
	if err == nil {
		t.Fatal("check --all returned nil with an unfetched source; the process would exit 0")
	}
	for _, want := range []string{"house", "agents sync"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
