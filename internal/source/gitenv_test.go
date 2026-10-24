package source_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/source"
)

func TestGitEnvDropsTheVariablesThatPointGitAtARepository(t *testing.T) {
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_PREFIX"} {
		t.Setenv(name, "/somewhere/else")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")

	env := source.GitEnv()

	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_PREFIX":
			t.Errorf("GitEnv keeps %s", entry)
		}
	}
	for _, want := range []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0"} {
		if !slices.Contains(env, want) {
			t.Errorf("GitEnv is missing %s", want)
		}
	}
}

// A git hook exports GIT_DIR and GIT_INDEX_FILE for its own repository, and
// `git commit -- <paths>` points GIT_INDEX_FILE at a temporary index. Run from
// inside one, the loader's git calls must still act on its cache, not on the
// hook's repository.
func TestFetchInsideAGitHookLeavesTheHooksRepositoryAlone(t *testing.T) {
	hermeticGit(t)
	fx := newFixture(t)
	outer := t.TempDir()
	git(t, outer, "init", "-b", "main")
	index := filepath.Join(outer, ".git", "next-index.lock")
	t.Setenv("GIT_DIR", filepath.Join(outer, ".git"))
	t.Setenv("GIT_WORK_TREE", outer)
	t.Setenv("GIT_INDEX_FILE", index)

	sha, err := source.New(t.TempDir()).Fetch(fx.bare, "main")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if sha != fx.v2 {
		t.Errorf("Fetch sha = %s, want %s", sha, fx.v2)
	}
	if _, err := os.Stat(index); !os.IsNotExist(err) {
		t.Errorf("Fetch wrote the hook's index %s (stat: %v)", index, err)
	}
}
