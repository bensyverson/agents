package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join("/xdg", "conf"))
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join("/xdg", "conf", "agents", "repos")
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(home, ".config", "agents", "repos")
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestReadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nowhere", "repos")
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read of missing file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Read of missing file = %v, want empty", got)
	}
}

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"empty file", "", nil},
		{"one entry", "/a/b\n", []string{"/a/b"}},
		{"no trailing newline", "/a/b", []string{"/a/b"}},
		{"skips blanks", "/a/b\n\n   \n/c/d\n", []string{"/a/b", "/c/d"}},
		{"skips comments", "# a comment\n/a/b\n   # indented comment\n", []string{"/a/b"}},
		{"trims whitespace", "  /a/b  \n\t/c/d\t\n", []string{"/a/b", "/c/d"}},
		{"preserves order", "/c\n/a\n/b\n", []string{"/c", "/a", "/b"}},
		{"cleans paths", "/a/b/\n/c//d/../e\n", []string{"/a/b", "/c/e"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repos")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := Read(path)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Read = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Read[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAddCreatesFileAndParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents", "repos")
	added, err := Add(path, "/home/dev/git/agents")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !added {
		t.Error("Add on a fresh file = false, want true")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "/home/dev/git/agents\n" {
		t.Errorf("file = %q, want %q", got, "/home/dev/git/agents\n")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("file perm = %v, want 0644", fi.Mode().Perm())
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o755 {
		t.Errorf("dir perm = %v, want 0755", di.Mode().Perm())
	}
}

func TestAddIsIdempotent(t *testing.T) {
	tests := []struct {
		name  string
		again string
	}{
		{"identical", "/home/dev/git/agents"},
		{"trailing slash", "/home/dev/git/agents/"},
		{"dot segment", "/home/dev/git/./agents"},
		{"parent segment", "/home/dev/git/other/../agents"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repos")
			if _, err := Add(path, "/home/dev/git/agents"); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			added, err := Add(path, tt.again)
			if err != nil {
				t.Fatalf("second Add: %v", err)
			}
			if added {
				t.Errorf("second Add(%q) = true, want false", tt.again)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Errorf("file changed: %q -> %q", before, after)
			}
		})
	}
}

func TestAddRejectsRelativePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos")
	added, err := Add(path, "relative/repo")
	if err == nil {
		t.Fatal("Add of a relative path: want error, got nil")
	}
	if added {
		t.Error("Add of a relative path = true, want false")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("Add of a relative path created the registry file")
	}
}

func TestAddPreservesCommentsAndBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos")
	initial := "# my repos\n\n/a/b\n\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(path, "/c/d"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := initial + "/c/d\n"
	if string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

func TestAddToFileWithoutTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos")
	if err := os.WriteFile(path, []byte("/a/b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Add(path, "/c/d"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "/a/b\n/c/d\n" {
		t.Errorf("file = %q, want %q", got, "/a/b\n/c/d\n")
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		repo        string
		wantRemoved bool
		wantFile    string
	}{
		{
			name:        "removes the line and keeps the rest",
			initial:     "# mine\n\n/a/b\n/c/d\n",
			repo:        "/a/b",
			wantRemoved: true,
			wantFile:    "# mine\n\n/c/d\n",
		},
		{
			name:        "matches after cleaning",
			initial:     "/a/b\n",
			repo:        "/a/b/",
			wantRemoved: true,
			wantFile:    "",
		},
		{
			name:        "absent repo leaves the file alone",
			initial:     "/a/b\n",
			repo:        "/x/y",
			wantRemoved: false,
			wantFile:    "/a/b\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repos")
			if err := os.WriteFile(path, []byte(tt.initial), 0o644); err != nil {
				t.Fatal(err)
			}
			removed, err := Remove(path, tt.repo)
			if err != nil {
				t.Fatalf("Remove: %v", err)
			}
			if removed != tt.wantRemoved {
				t.Errorf("Remove = %v, want %v", removed, tt.wantRemoved)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.wantFile {
				t.Errorf("file = %q, want %q", got, tt.wantFile)
			}
		})
	}
}

func TestRemoveMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nowhere", "repos")
	removed, err := Remove(path, "/a/b")
	if err != nil {
		t.Fatalf("Remove of missing file: %v", err)
	}
	if removed {
		t.Error("Remove of missing file = true, want false")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("Remove created the registry file")
	}
}
