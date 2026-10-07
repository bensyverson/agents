package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

var orphanHash = module.Hash("x\n")

var (
	orphanMarkerBegin = "<!-- agents:begin gone@" + orphanHash + " -->\n"
	orphanMarkerEnd   = "<!-- agents:end gone -->\n"
)

func writeOrphan(t *testing.T, repoDir, rel, content string) {
	t.Helper()
	path := filepath.Join(repoDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusShowsOrphanedGeneratedFilesAndUnrendered(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	writeOrphan(t, repoDir, "project/agents/gone.md", orphanMarkerBegin+"x\n"+orphanMarkerEnd)

	stdout, _, err := run(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, want := range []string{"orphans=1", "unrendered=0"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status output missing %q:\n%s", want, stdout)
		}
	}
}

func TestCheckAndSyncHandleOrphanedGeneratedFiles(t *testing.T) {
	repoDir, sourceDir, _ := fixture(t)
	initRepo(t, sourceDir)
	writeOrphan(t, repoDir, "project/agents/gone.md", orphanMarkerBegin+"x\n"+orphanMarkerEnd)
	writeOrphan(t, repoDir, "project/agents/mixed.md", "mine\n\n"+orphanMarkerBegin+"x\n"+orphanMarkerEnd)

	stdout, _, err := run(t, "check")
	if err == nil {
		t.Fatal("check passed with orphaned generated files")
	}
	if !strings.Contains(stdout, "project/agents/gone.md: gone orphaned") {
		t.Errorf("check output:\n%s", stdout)
	}

	stdout, _, err = run(t, "sync")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	for _, want := range []string{"removed project/agents/gone.md", "kept project/agents/mixed.md"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("sync output missing %q:\n%s", want, stdout)
		}
	}
	if _, _, err := run(t, "check"); err != nil {
		t.Errorf("check after sync: %v", err)
	}
}
