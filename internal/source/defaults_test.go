package source_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/source"
)

const defaultsFile = "defaults: [core, principles, stage-build]\n"

var wantDefaults = []string{"core", "principles", "stage-build"}

func defaultsFS(sourceYAML string) fstest.MapFS {
	fsys := fstest.MapFS{"modules/core.md": &fstest.MapFile{Data: []byte("core\n")}}
	if sourceYAML != "" {
		fsys[source.SourceFile] = &fstest.MapFile{Data: []byte(sourceYAML)}
	}
	return fsys
}

// A source declares its default modules in source.yaml, in render order.
func TestFromFSReadsDefaults(t *testing.T) {
	src, err := source.FromFS("house", defaultsFS(defaultsFile))
	if err != nil {
		t.Fatalf("FromFS: %v", err)
	}
	if !reflect.DeepEqual(src.Defaults, wantDefaults) {
		t.Errorf("Defaults = %v, want %v", src.Defaults, wantDefaults)
	}
}

// No source.yaml is the ordinary case: the source simply declares no defaults.
func TestSourceWithoutSourceYAMLHasNoDefaults(t *testing.T) {
	src, err := source.FromFS("house", defaultsFS(""))
	if err != nil {
		t.Fatalf("FromFS: %v", err)
	}
	if len(src.Defaults) != 0 {
		t.Errorf("Defaults = %v, want none", src.Defaults)
	}
}

func TestPathSourceReadsDefaults(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "modules", "core.md"), "core\n")
	write(t, filepath.Join(dir, source.SourceFile), defaultsFile)

	src, err := source.New(t.TempDir()).Path("house", dir)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !reflect.DeepEqual(src.Defaults, wantDefaults) {
		t.Errorf("Defaults = %v, want %v", src.Defaults, wantDefaults)
	}
}

func TestGitSourceReadsDefaults(t *testing.T) {
	hermeticGit(t)
	root := t.TempDir()
	work := filepath.Join(root, "work")
	git(t, root, "init", "-b", "main", work)
	write(t, filepath.Join(work, "modules", "core.md"), "core\n")
	write(t, filepath.Join(work, source.SourceFile), defaultsFile)
	git(t, work, "add", "-A")
	git(t, work, "commit", "-m", "one")
	bare := filepath.Join(root, "bare.git")
	git(t, root, "clone", "--bare", work, bare)

	loader := source.New(t.TempDir())
	if _, err := loader.Fetch(bare, "main"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	src, err := loader.Git("house", bare, "main")
	if err != nil {
		t.Fatalf("Git: %v", err)
	}
	if !reflect.DeepEqual(src.Defaults, wantDefaults) {
		t.Errorf("Defaults = %v, want %v", src.Defaults, wantDefaults)
	}
}

// A typo in source.yaml is an error, not a source that quietly declares
// nothing.
func TestSourceYAMLErrors(t *testing.T) {
	for name, body := range map[string]string{
		"unknown field": "default: [core]\n",
		"not a list":    "defaults: core\n",
		"not yaml":      "defaults: [core\n",
	} {
		_, err := source.FromFS("house", defaultsFS(body))
		if err == nil {
			t.Errorf("%s: FromFS returned a nil error", name)
			continue
		}
		if !errors.Is(err, source.ErrSourceFile) || !strings.Contains(err.Error(), "house") {
			t.Errorf("%s: error = %v, want source.ErrSourceFile naming the source", name, err)
		}
	}
}
