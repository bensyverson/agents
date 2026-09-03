package manifest_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// canonical is the form Marshal must produce and the form of the manifest
// checked into this repo's root.
const canonical = "modules:\n  - core\n  - principles\n  - stage-build\n  - go\n"

// exampleRefs are the module entries a manifest with no sources: block parses
// to: every bare name comes from the implicit example source.
func exampleRefs(names ...string) []manifest.ModuleRef {
	refs := make([]manifest.ModuleRef, len(names))
	for i, name := range names {
		refs[i] = manifest.ModuleRef{Source: manifest.ExampleSource, Name: name}
	}
	return refs
}

func TestFileName(t *testing.T) {
	if manifest.FileName != ".agents.yaml" {
		t.Errorf("FileName = %q, want %q", manifest.FileName, ".agents.yaml")
	}
}

func TestParse(t *testing.T) {
	m, err := manifest.Parse([]byte(canonical))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := exampleRefs("core", "principles", "stage-build", "go")
	if !reflect.DeepEqual(m.Modules, want) {
		t.Errorf("Modules = %v, want %v (manifest order preserved)", m.Modules, want)
	}
}

func TestParseAcceptsFlowAndInlineForms(t *testing.T) {
	m, err := manifest.Parse([]byte("modules: [core, web3, stage-build]\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := exampleRefs("core", "web3", "stage-build")
	if !reflect.DeepEqual(m.Modules, want) {
		t.Errorf("Modules = %v, want %v", m.Modules, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{"empty document", "", manifest.ErrNoModules},
		{"no modules key", "other: 1\n", nil},
		{"null modules", "modules:\n", manifest.ErrNoModules},
		{"empty list", "modules: []\n", manifest.ErrNoModules},
		{"duplicate names", "modules:\n  - core\n  - core\n", manifest.ErrDuplicateModule},
		{"uppercase name", "modules:\n  - Core\n", manifest.ErrInvalidModuleName},
		{"leading dash", "modules:\n  - -core\n", manifest.ErrInvalidModuleName},
		{"underscore", "modules:\n  - stage_build\n", manifest.ErrInvalidModuleName},
		{"dot", "modules:\n  - core.md\n", manifest.ErrInvalidModuleName},
		{"empty name", "modules:\n  - \"\"\n", manifest.ErrInvalidModuleName},
		{"space", "modules:\n  - stage build\n", manifest.ErrInvalidModuleName},
		{"unknown top-level key", "modules:\n  - core\nextra: true\n", nil},
		{"malformed yaml", "modules: [core\n", nil},
		{"wrong type", "modules: core\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.Parse([]byte(tt.in))
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want one", tt.in)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Parse(%q) error = %v, want %v", tt.in, err, tt.want)
			}
		})
	}
}

func TestParseErrorNamesTheOffender(t *testing.T) {
	_, err := manifest.Parse([]byte("modules:\n  - core\n  - Stage_Build\n"))
	if err == nil {
		t.Fatal("Parse = nil error, want one")
	}
	if !strings.Contains(err.Error(), "Stage_Build") {
		t.Errorf("error = %q, want it to name the offending module", err)
	}
}

func TestMarshalCanonicalForm(t *testing.T) {
	got, err := manifest.Marshal(manifest.Manifest{Modules: exampleRefs("core", "principles", "stage-build", "go")})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != canonical {
		t.Errorf("Marshal =\n%q\nwant\n%q", got, canonical)
	}
}

func TestMarshalRejectsInvalid(t *testing.T) {
	if _, err := manifest.Marshal(manifest.Manifest{}); !errors.Is(err, manifest.ErrNoModules) {
		t.Errorf("Marshal(empty) error = %v, want ErrNoModules", err)
	}
	if _, err := manifest.Marshal(manifest.Manifest{Modules: exampleRefs("core", "core")}); !errors.Is(err, manifest.ErrDuplicateModule) {
		t.Errorf("Marshal(dupes) error = %v, want ErrDuplicateModule", err)
	}
}

func TestRoundTrip(t *testing.T) {
	m, err := manifest.Parse([]byte(canonical))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != canonical {
		t.Errorf("round trip =\n%q\nwant\n%q", got, canonical)
	}
}

// The manifest checked into this repo's root is the ground truth for the
// canonical form: reading and re-marshalling it must reproduce it byte for byte.
func TestRoundTripRepoManifest(t *testing.T) {
	path := filepath.Join("..", "..", manifest.FileName)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m, err := manifest.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("re-marshalled %s =\n%q\nwant\n%q", path, got, original)
	}
}

func TestWriteThenRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), manifest.FileName)
	want := manifest.Manifest{Modules: exampleRefs("core", "go")}
	if err := manifest.Write(path, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got, wantBytes := string(data), "modules:\n  - core\n  - go\n"; got != wantBytes {
		t.Errorf("file =\n%q\nwant\n%q", got, wantBytes)
	}
	got, err := manifest.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Read = %+v, want %+v", got, want)
	}
}

func TestWriteRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), manifest.FileName)
	if err := manifest.Write(path, manifest.Manifest{}); err == nil {
		t.Fatal("Write(empty) = nil error, want one")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Write(empty) created %s; an invalid manifest must not reach disk", path)
	}
}

func TestReadMissing(t *testing.T) {
	if _, err := manifest.Read(filepath.Join(t.TempDir(), manifest.FileName)); err == nil {
		t.Fatal("Read(missing) = nil error, want one")
	}
}

func TestReadInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), manifest.FileName)
	if err := os.WriteFile(path, []byte("modules: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := manifest.Read(path)
	if !errors.Is(err, manifest.ErrNoModules) {
		t.Errorf("Read error = %v, want ErrNoModules", err)
	}
	if err != nil && !strings.Contains(err.Error(), manifest.FileName) {
		t.Errorf("error = %q, want it to name the file", err)
	}
}

func testSet(t *testing.T) module.Set {
	t.Helper()
	set, err := module.Load(fstest.MapFS{
		"core.md":        &fstest.MapFile{Data: []byte("## Working rules\n")},
		"principles.md":  &fstest.MapFile{Data: []byte("## Principles\n")},
		"stage-build.md": &fstest.MapFile{Data: []byte("## Stage: BUILD\n")},
		"go.md":          &fstest.MapFile{Data: []byte("## Go\n")},
	})
	if err != nil {
		t.Fatalf("module.Load: %v", err)
	}
	return set
}

func TestResolveKeepsManifestOrder(t *testing.T) {
	m := manifest.Manifest{Modules: exampleRefs("go", "core", "principles")}
	got, err := manifest.Resolve(m, testSet(t))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	var names []string
	for _, mod := range got {
		names = append(names, mod.Name)
	}
	if want := m.Entries(); !reflect.DeepEqual(names, want) {
		t.Errorf("Resolve = %v, want %v", names, want)
	}
	if got[0].Body != "## Go\n" {
		t.Errorf("first module body = %q, want the loaded module", got[0].Body)
	}
}

func TestResolveListsEveryUnknownName(t *testing.T) {
	m := manifest.Manifest{Modules: exampleRefs("core", "nope", "go", "alsonope")}
	_, err := manifest.Resolve(m, testSet(t))
	if err == nil {
		t.Fatal("Resolve = nil error, want one")
	}
	for _, missing := range []string{"nope", "alsonope"} {
		if !strings.Contains(err.Error(), missing) {
			t.Errorf("error = %q, want it to list %q", err, missing)
		}
	}
	var unknown *manifest.UnknownModulesError
	if !errors.As(err, &unknown) {
		t.Fatalf("error = %v, want an *UnknownModulesError", err)
	}
	if want := exampleRefs("nope", "alsonope"); !reflect.DeepEqual(unknown.Refs, want) {
		t.Errorf("UnknownModulesError.Refs = %v, want %v (manifest order)", unknown.Refs, want)
	}
}
