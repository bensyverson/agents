package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A directory that exists is a path source; a spec naming a host is HTTPS;
// anything else reaches git verbatim. The name is the last segment, without any
// .git.
func TestParseSource(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "house-rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		spec string
		want Source
	}{
		{dir, Source{Name: "house-rules", Path: dir}},
		{"https://example.com/team/house.git", Source{Name: "house", Git: "https://example.com/team/house.git"}},
		{"git@example.com:team/house-rules.git", Source{Name: "house-rules", Git: "git@example.com:team/house-rules.git"}},
		{"https://example.com/team/house", Source{Name: "house", Git: "https://example.com/team/house"}},
		{"ssh://git@example.com/team/house.git", Source{Name: "house", Git: "ssh://git@example.com/team/house.git"}},
		{"file:///srv/git/house.git", Source{Name: "house", Git: "file:///srv/git/house.git"}},

		// A scheme-less host/path is HTTPS.
		{"github.com/bensyverson/agents-md", Source{Name: "agents-md", Git: "https://github.com/bensyverson/agents-md"}},
		{"github.com/bensyverson/agents-md.git", Source{Name: "agents-md", Git: "https://github.com/bensyverson/agents-md.git"}},
		{"gitlab.example.org/a/b/c/", Source{Name: "c", Git: "https://gitlab.example.org/a/b/c"}},

		// Not a host: no dot in the first segment, or it starts with one, or
		// the spec is scp-style, so the spec goes to git as typed.
		{"localhost/team/house", Source{Name: "house", Git: "localhost/team/house"}},
		{"./team/house", Source{Name: "house", Git: "./team/house"}},
		{"../team/house", Source{Name: "house", Git: "../team/house"}},
		{".hidden.example/team/house", Source{Name: "house", Git: ".hidden.example/team/house"}},
		{"/srv/git/house.git", Source{Name: "house", Git: "/srv/git/house.git"}},
		{"git@example.com:team/house", Source{Name: "house", Git: "git@example.com:team/house"}},
		{"example.com:team/house", Source{Name: "house", Git: "example.com:team/house"}},
		{"example.com", Source{Name: "example.com", Git: "example.com"}},
	} {
		got, err := ParseSource(tc.spec)
		if err != nil {
			t.Errorf("ParseSource(%q): %v", tc.spec, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseSource(%q) = %+v, want %+v", tc.spec, got, tc.want)
		}
	}
}

// A directory that exists beats the HTTPS reading, even when it looks like a host.
func TestParseSourceExistingDirectoryWinsOverAHost(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("example.com/team/rules", 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ParseSource("example.com/team/rules")
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if want := (Source{Name: "rules", Path: "example.com/team/rules"}); got != want {
		t.Errorf("ParseSource = %+v, want %+v", got, want)
	}
}

func TestParseSourceEmptySpec(t *testing.T) {
	for _, spec := range []string{"", "/", "///"} {
		if _, err := ParseSource(spec); !errors.Is(err, ErrNoSourceGiven) {
			t.Errorf("ParseSource(%q) error = %v, want ErrNoSourceGiven", spec, err)
		}
	}
}
