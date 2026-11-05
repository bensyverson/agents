package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertLinked fails unless CLAUDE.md is the relative link to AGENTS.md.
func assertLinked(t *testing.T, dir string) {
	t.Helper()
	target, err := os.Readlink(filepath.Join(dir, ClaudeFile))
	if err != nil {
		t.Fatalf("readlink %s: %v", ClaudeFile, err)
	}
	if target != AgentsFile {
		t.Errorf("%s -> %q, want %q", ClaudeFile, target, AgentsFile)
	}
}

func TestMigrateMovesALoneClaudeFile(t *testing.T) {
	const content = "# My rules\n\nUse tabs.\n"
	dir := newRepo(t, map[string]string{ClaudeFile: content})

	res, err := Migrate(dir)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res != MigrateMoved {
		t.Errorf("result = %v, want MigrateMoved", res)
	}
	if got := readFixture(t, dir, AgentsFile); got != content {
		t.Errorf("%s = %q, want %q", AgentsFile, got, content)
	}
	assertLinked(t, dir)
}

func TestMigrateAppendsToAnAgentsFileWithoutRegions(t *testing.T) {
	dir := newRepo(t, map[string]string{
		AgentsFile: "# Agents rules\n\nRun the tests.\n",
		ClaudeFile: "# Claude rules\n\nUse tabs.\n",
	})

	res, err := Migrate(dir)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res != MigrateAppended {
		t.Errorf("result = %v, want MigrateAppended", res)
	}
	want := "# Agents rules\n\nRun the tests.\n\n" + MovedMarker + "\n\n# Claude rules\n\nUse tabs.\n"
	if got := readFixture(t, dir, AgentsFile); got != want {
		t.Errorf("%s:\n got %q\nwant %q", AgentsFile, got, want)
	}
	assertLinked(t, dir)
}

// In a managed repo the moved text belongs to the project-owned head, which
// ends where the first region begins; after the regions it would read as part
// of the generated rules.
func TestMigrateInsertsAboveTheFirstRegion(t *testing.T) {
	regions := fresh("core", coreBody)
	dir := newRepo(t, map[string]string{
		AgentsFile: "# Head\n\n" + regions,
		ClaudeFile: "Use tabs.\n",
	})

	if _, err := Migrate(dir); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	want := "# Head\n\n" + MovedMarker + "\n\nUse tabs.\n\n" + regions
	if got := readFixture(t, dir, AgentsFile); got != want {
		t.Errorf("%s:\n got %q\nwant %q", AgentsFile, got, want)
	}
}

func TestMigrateIntoAnAgentsFileThatIsOnlyRegions(t *testing.T) {
	regions := fresh("core", coreBody)
	dir := newRepo(t, map[string]string{
		AgentsFile: regions,
		ClaudeFile: "Use tabs.\n",
	})

	if _, err := Migrate(dir); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	want := MovedMarker + "\n\nUse tabs.\n\n" + regions
	if got := readFixture(t, dir, AgentsFile); got != want {
		t.Errorf("%s:\n got %q\nwant %q", AgentsFile, got, want)
	}
}

// Some projects link AGENTS.md to CLAUDE.md, the other way round. Appending
// through that link would write CLAUDE.md into itself.
func TestMigrateReversesAnAgentsLinkToClaude(t *testing.T) {
	const content = "# My rules\n"
	dir := newRepo(t, map[string]string{ClaudeFile: content})
	if err := os.Symlink(ClaudeFile, filepath.Join(dir, AgentsFile)); err != nil {
		t.Fatal(err)
	}

	res, err := Migrate(dir)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res != MigrateMoved {
		t.Errorf("result = %v, want MigrateMoved", res)
	}
	info, err := os.Lstat(filepath.Join(dir, AgentsFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("%s is still a symlink", AgentsFile)
	}
	if got := readFixture(t, dir, AgentsFile); got != content {
		t.Errorf("%s = %q, want %q", AgentsFile, got, content)
	}
	assertLinked(t, dir)
}

func TestMigrateHasNothingToDo(t *testing.T) {
	t.Run("no CLAUDE.md", func(t *testing.T) {
		dir := newRepo(t, map[string]string{AgentsFile: "# Head\n"})
		res, err := Migrate(dir)
		if err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		if res != MigrateNothing {
			t.Errorf("result = %v, want MigrateNothing", res)
		}
		if _, err := os.Lstat(filepath.Join(dir, ClaudeFile)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s was created; linking is init's job", ClaudeFile)
		}
	})
	t.Run("already linked", func(t *testing.T) {
		dir := newRepo(t, map[string]string{AgentsFile: "# Head\n"})
		if err := os.Symlink(AgentsFile, filepath.Join(dir, ClaudeFile)); err != nil {
			t.Fatal(err)
		}
		res, err := Migrate(dir)
		if err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		if res != MigrateNothing {
			t.Errorf("result = %v, want MigrateNothing", res)
		}
		if got := readFixture(t, dir, AgentsFile); got != "# Head\n" {
			t.Errorf("%s = %q, want it untouched", AgentsFile, got)
		}
	})
}

func TestMigrateRefusesALinkElsewhere(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "rules.md")
	if err := os.WriteFile(shared, []byte("# Shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := newRepo(t, map[string]string{AgentsFile: "# Head\n"})
	if err := os.Symlink(shared, filepath.Join(dir, ClaudeFile)); err != nil {
		t.Fatal(err)
	}

	_, err := Migrate(dir)
	var cerr *ClaudeFileError
	if !errors.As(err, &cerr) || cerr.Kind != ClaudeLinkedElsewhere {
		t.Fatalf("err = %v, want a ClaudeFileError of kind ClaudeLinkedElsewhere", err)
	}
	if !strings.Contains(err.Error(), shared) {
		t.Errorf("error %q does not name the link's target", err)
	}
	if got := readFixture(t, dir, AgentsFile); got != "# Head\n" {
		t.Errorf("%s = %q, want it untouched", AgentsFile, got)
	}
	if target, _ := os.Readlink(filepath.Join(dir, ClaudeFile)); target != shared {
		t.Errorf("%s -> %q, want the link untouched", ClaudeFile, target)
	}
}

// A CLAUDE.md copied from a managed repo carries region markers; moved into a
// head above the same regions, they would make AGENTS.md unparseable.
func TestMigrateRefusesTextThatBreaksTheRegions(t *testing.T) {
	regions := fresh("core", coreBody)
	claude := "# Copied\n\n" + regions
	dir := newRepo(t, map[string]string{
		AgentsFile: "# Head\n\n" + regions,
		ClaudeFile: claude,
	})

	if _, err := Migrate(dir); err == nil {
		t.Fatal("Migrate returned nil error")
	}
	if got := readFixture(t, dir, AgentsFile); got != "# Head\n\n"+regions {
		t.Errorf("%s changed: %q", AgentsFile, got)
	}
	if got := readFixture(t, dir, ClaudeFile); got != claude {
		t.Errorf("%s changed: %q", ClaudeFile, got)
	}
}
