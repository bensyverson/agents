package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
)

const (
	docsBody = "docs rules\n"
	// delegationModule is a kind:file module, so add has a file to install and
	// remove has one to delete.
	delegationPath   = "project/agents/delegation.md"
	delegationBody   = "# Delegating\n\nrules\n"
	delegationModule = "---\nkind: file\n---\n" + delegationBody
)

// writeModule adds a module to the source directory a fixture built.
func writeModule(t *testing.T, sourceDir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(sourceDir, "modules", name+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// head is everything above the first generated region — the project's own.
func head(t *testing.T, repoDir string) string {
	t.Helper()
	doc := readAgents(t, repoDir)
	before, _, ok := strings.Cut(doc, "<!-- agents:begin ")
	if !ok {
		t.Fatalf("AGENTS.md has no region:\n%s", doc)
	}
	return before
}

// writeSourcesManifest writes a manifest naming the fixture source and these
// module entries, one per line.
func writeSourcesManifest(t *testing.T, repoDir, sourceDir string, entries ...string) {
	t.Helper()
	body := "sources:\n  - name: " + sourceName + "\n    path: " + sourceDir + "\nmodules:"
	if len(entries) == 0 {
		body += " []\n"
	}
	for _, e := range entries {
		body += "\n  - " + e
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".agents.yaml"), []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddOnAnEmptyManifest(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	writeSourcesManifest(t, repoDir, sourceDir)

	stdout, _, err := run(t, "add", "principles")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(stdout, "enabled principles") {
		t.Errorf("add output:\n%s", stdout)
	}
	if got := readAgents(t, repoDir); !strings.Contains(got, principlesBody) {
		t.Errorf("AGENTS.md does not hold the principles region:\n%s", got)
	}
	if _, _, err := run(t, "check"); err != nil {
		t.Errorf("check after add: %v", err)
	}
}

func TestEmbeddedModulesUnderSourcesAreExplainedAndRemovable(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	writeSourcesManifest(t, repoDir, sourceDir, "core", "agents", "module-authoring")

	_, _, err := run(t, "check")
	if err == nil {
		t.Fatal("check succeeded on a manifest listing embedded modules under sources:")
	}
	for _, want := range []string{
		"agents and module-authoring are example modules built into the binary",
		"run agents remove agents module-authoring",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}

	stdout, _, err := run(t, "remove", "agents", "module-authoring")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(stdout, "disabled agents") || !strings.Contains(stdout, "disabled module-authoring") {
		t.Errorf("remove output:\n%s", stdout)
	}
	m, err := manifest.Read(filepath.Join(repoDir, manifest.FileName))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	if got := strings.Join(m.Entries(), ","); got != "core" {
		t.Errorf("manifest lists %q, want core", got)
	}
	if _, _, err := run(t, "sync"); err != nil {
		t.Errorf("sync after remove: %v", err)
	}
}

func TestAddEnablesRendersAndInstallsTheFile(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	writeModule(t, sourceDir, "docs", docsBody)
	writeModule(t, sourceDir, "delegation", delegationModule)
	initRepo(t, sourceDir)
	before := head(t, repoDir)

	stdout, _, err := run(t, "add", "docs", "delegation")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, want := range []string{"enabled docs", "enabled delegation", "rendered AGENTS.md", "rendered " + delegationPath} {
		if !strings.Contains(stdout, want) {
			t.Errorf("add output missing %q:\n%s", want, stdout)
		}
	}
	if b, err := os.ReadFile(filepath.Join(repoDir, ".agents.yaml")); err != nil ||
		!strings.Contains(string(b), "docs") || !strings.Contains(string(b), "delegation") {
		t.Errorf("manifest %q does not list the added modules (err %v)", b, err)
	}
	if got := readAgents(t, repoDir); !strings.Contains(got, docsBody) {
		t.Errorf("AGENTS.md does not hold the docs region:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(repoDir, delegationPath)); err != nil {
		t.Errorf("%s: %v", delegationPath, err)
	}
	if now := head(t, repoDir); now != before {
		t.Errorf("add rewrote the project-owned head:\n got %q\nwant %q", now, before)
	}
	// Whatever add wrote, a following sync must have nothing to do.
	if out, _, err := run(t, "sync"); err != nil || out != "" {
		t.Errorf("sync after add printed %q (err %v), want silence", out, err)
	}
}

func TestAddSaysWhenAModuleIsAlreadyEnabled(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)

	stdout, _, err := run(t, "add", "core")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(stdout, "core is already enabled") {
		t.Errorf("add output = %q, want it to say core is already enabled", stdout)
	}
}

func TestAddRejectsAnUnknownModule(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)

	_, _, err := run(t, "add", "nosuch")
	if err == nil {
		t.Fatal("add of an unknown module returned nil error")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error = %q, want it to name the module", err)
	}
}

func TestRemoveDropsTheRegionAndDeletesTheFile(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	writeModule(t, sourceDir, "delegation", delegationModule)
	initRepo(t, sourceDir)
	if _, _, err := run(t, "add", "delegation"); err != nil {
		t.Fatalf("add: %v", err)
	}
	before := head(t, repoDir)

	stdout, _, err := run(t, "remove", "delegation")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	for _, want := range []string{"disabled delegation", "deleted " + delegationPath} {
		if !strings.Contains(stdout, want) {
			t.Errorf("remove output missing %q:\n%s", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(repoDir, delegationPath)); err == nil {
		t.Errorf("%s still exists", delegationPath)
	}
	if now := head(t, repoDir); now != before {
		t.Errorf("remove rewrote the project-owned head:\n got %q\nwant %q", now, before)
	}
	if out, _, err := run(t, "sync"); err != nil || out != "" {
		t.Errorf("sync after remove printed %q (err %v), want silence", out, err)
	}
}

func TestRemoveKeepsAndNamesAHandExtendedFile(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	writeModule(t, sourceDir, "delegation", delegationModule)
	initRepo(t, sourceDir)
	if _, _, err := run(t, "add", "delegation"); err != nil {
		t.Fatalf("add: %v", err)
	}
	path := filepath.Join(repoDir, delegationPath)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", delegationPath, err)
	}
	const ours = "\n## Ours\n\nProject-written.\n"
	if err := os.WriteFile(path, append(b, []byte(ours)...), 0o644); err != nil {
		t.Fatalf("extend %s: %v", delegationPath, err)
	}

	stdout, _, err := run(t, "remove", "delegation")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(stdout, delegationPath) || !strings.Contains(stdout, "kept") {
		t.Errorf("remove output does not warn about the kept file:\n%s", stdout)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s was deleted: %v", delegationPath, err)
	}
	if !strings.Contains(string(got), "Project-written.") {
		t.Errorf("%s lost the project's text:\n%s", delegationPath, got)
	}
	if strings.Contains(string(got), "agents:begin") {
		t.Errorf("%s still holds the generated region:\n%s", delegationPath, got)
	}
}

func TestRemoveRejectsAModuleThatIsNotEnabled(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	writeModule(t, sourceDir, "docs", docsBody)
	initRepo(t, sourceDir)

	_, _, err := run(t, "remove", "docs")
	if err == nil {
		t.Fatal("remove of a module that is not enabled returned nil error")
	}
	if !strings.Contains(err.Error(), "docs") {
		t.Errorf("error = %q, want it to name the module", err)
	}
}

func TestListMarksTheEnabledModules(t *testing.T) {
	_, sourceDir, _ := fixture(t)
	writeModule(t, sourceDir, "docs", docsBody)
	writeModule(t, sourceDir, "delegation", delegationModule)
	initRepo(t, sourceDir)

	stdout, _, err := run(t, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	marks := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimRight(stdout, "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(line, "*") {
			marks[fields[1]] = true
			continue
		}
		marks[fields[0]] = false
	}
	for _, name := range []string{"core", "delegation", "docs", "principles"} {
		if _, listed := marks[name]; !listed {
			t.Errorf("list does not mention %q:\n%s", name, stdout)
		}
	}
	if !marks["core"] || !marks["principles"] {
		t.Errorf("list does not mark the enabled modules:\n%s", stdout)
	}
	if marks["docs"] || marks["delegation"] {
		t.Errorf("list marks a module the manifest does not enable:\n%s", stdout)
	}
	if !strings.Contains(stdout, delegationPath) {
		t.Errorf("list does not show the kind:file path:\n%s", stdout)
	}
}

// `agents list` answers "what could I enable?", so it must work where there is
// no manifest at all — listing the binary's own modules and marking nothing.
func TestListWorksOutsideAManagedRepo(t *testing.T) {
	emptyRepo(t)

	stdout, _, err := run(t, "list")
	if err != nil {
		t.Fatalf("list outside a managed repo: %v", err)
	}
	if !strings.Contains(stdout, "agents") {
		t.Errorf("list printed no modules:\n%s", stdout)
	}
	if strings.Contains(stdout, "*") {
		t.Errorf("list marked a module with no manifest:\n%s", stdout)
	}
}
