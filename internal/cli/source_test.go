package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

func TestSourceAddAppendsAPathSourceAndEnablesNothing(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	other := newSource(t, "other", map[string]string{"modules/web.md": "web rules\n"})
	before := readAgents(t, repoDir)

	stdout, stderr, err := run(t, "source", "add", other)
	if err != nil {
		t.Fatalf("source add: %v (stderr %s)", err, stderr)
	}

	if !strings.Contains(stdout, "added source other") {
		t.Errorf("output does not say what was added:\n%s", stdout)
	}
	written := readFile(t, filepath.Join(repoDir, manifest.FileName))
	for _, want := range []string{"name: " + sourceName, "name: other", "path: " + other} {
		if !strings.Contains(written, want) {
			t.Errorf("%s missing %q:\n%s", manifest.FileName, want, written)
		}
	}
	if strings.Contains(written, "web") {
		t.Errorf("source add enabled a module:\n%s", written)
	}
	if got := readAgents(t, repoDir); got != before {
		t.Errorf("AGENTS.md changed:\n%s", got)
	}
}

// --name and --ref shape the entry; the ref is resolved to a sha.
func TestSourceAddPinsAGitSourceAndHonorsName(t *testing.T) {
	url, sha := gitFleet(t)
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)

	stdout, stderr, err := run(t, "source", "add", "file://"+url, "--name", "shared", "--ref", "main")
	if err != nil {
		t.Fatalf("source add: %v (stderr %s)", err, stderr)
	}

	written := readFile(t, manifest.FileName)
	for _, want := range []string{"name: shared", "git: file://" + url, "ref: " + sha} {
		if !strings.Contains(written, want) {
			t.Errorf("%s missing %q:\n%s", manifest.FileName, want, written)
		}
	}
	if !strings.Contains(stdout, "added source shared") {
		t.Errorf("output does not say what was added:\n%s", stdout)
	}
}

func TestSourceAddRefusesANameInUse(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	before := readFile(t, manifest.FileName)
	same := newSource(t, sourceName, map[string]string{"modules/web.md": "web\n"})

	_, _, err := run(t, "source", "add", same)

	if err == nil || !strings.Contains(err.Error(), sourceName) {
		t.Fatalf("source add error = %v, want it to name %q", err, sourceName)
	}
	if got := readFile(t, manifest.FileName); got != before {
		t.Errorf("manifest changed:\n%s", got)
	}
}

func TestSourceAddRefOnAPathSourceIsRefused(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	other := newSource(t, "other", map[string]string{"modules/web.md": "web\n"})

	if _, _, err := run(t, "source", "add", other, "--ref", "main"); err == nil {
		t.Fatal("source add --ref on a directory returned a nil error")
	}
}

// Decision 2: the first source replaces the example modules and the command
// says so in one line.
func TestSourceAddOnAnEmbeddedRepoRemovesTheExampleModulesAndAddWorks(t *testing.T) {
	dir := emptyRepo(t)
	if _, stderr, err := run(t, "init"); err != nil {
		t.Fatalf("init: %v (stderr %s)", err, stderr)
	}
	other := newSource(t, "other", map[string]string{"modules/web.md": "web rules\n"})

	stdout, stderr, err := run(t, "source", "add", other)
	if err != nil {
		t.Fatalf("source add: %v (stderr %s)", err, stderr)
	}

	removed := ""
	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.Contains(line, "agents") && strings.Contains(line, "module-authoring") && strings.Contains(line, "removed") {
			removed = line
		}
	}
	if removed == "" {
		t.Fatalf("output has no line naming the removed example modules:\n%s", stdout)
	}
	if !strings.Contains(removed, "built into the binary") {
		t.Errorf("line %q does not say why", removed)
	}
	if doc := readAgents(t, dir); strings.Contains(doc, "agents:begin agents@") {
		t.Errorf("AGENTS.md still holds the example region:\n%s", doc)
	}

	if _, stderr, err := run(t, "add", "web"); err != nil {
		t.Fatalf("add web: %v (stderr %s)", err, stderr)
	}
	if doc := readAgents(t, dir); !strings.Contains(doc, "web rules") {
		t.Errorf("add web did not render:\n%s", doc)
	}
}

func TestSourceRemoveRefusesWhileAModuleIsEnabledAndNamesIt(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	before := readFile(t, manifest.FileName)

	_, _, err := run(t, "source", "remove", sourceName)

	if err == nil {
		t.Fatal("source remove returned a nil error while its modules are enabled")
	}
	for _, want := range []string{"core", "principles"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if got := readFile(t, manifest.FileName); got != before {
		t.Errorf("manifest changed:\n%s", got)
	}
}

func TestSourceRemoveDropsAnUnusedSource(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	other := newSource(t, "other", map[string]string{"modules/web.md": "web\n"})
	if _, stderr, err := run(t, "source", "add", other); err != nil {
		t.Fatalf("source add: %v (stderr %s)", err, stderr)
	}

	stdout, stderr, err := run(t, "source", "remove", "other")
	if err != nil {
		t.Fatalf("source remove: %v (stderr %s)", err, stderr)
	}

	if !strings.Contains(stdout, "removed source other") {
		t.Errorf("output does not say what was removed:\n%s", stdout)
	}
	if written, _ := os.ReadFile(manifest.FileName); strings.Contains(string(written), "other") {
		t.Errorf("manifest still names other:\n%s", written)
	}
}

func TestSourceRemoveOfAnUnknownName(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)

	if _, _, err := run(t, "source", "remove", "nope"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("source remove error = %v, want it to name nope", err)
	}
}
