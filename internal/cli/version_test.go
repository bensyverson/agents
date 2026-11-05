package cli

import (
	"strings"
	"testing"
)

func TestVersionFlagPrintsTheReleaseVersion(t *testing.T) {
	stdout, _, err := run(t, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if got, want := strings.TrimSpace(stdout), "agents "+version; got != want {
		t.Errorf("--version printed %q, want %q", got, want)
	}
}
