package manifest_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/manifest"
	"github.com/bensyverson/agents/internal/module"
)

// canonicalSources is the form Marshal must produce for a manifest that names
// its own sources: the sources block first, then the modules that pick from it.
const canonicalSources = `sources:
  - name: house
    git: https://example.invalid/house.git
    ref: 9c1f2e
  - name: local
    path: ../rules
modules:
  - core
  - local/go
  - principles
`

func TestExampleSourceName(t *testing.T) {
	if manifest.ExampleSource != "example" {
		t.Errorf("ExampleSource = %q, want %q", manifest.ExampleSource, "example")
	}
}

func TestDefaultSourceIsTheExampleSourceWhenNoneAreListed(t *testing.T) {
	m, err := manifest.Parse([]byte(canonical))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := m.DefaultSource(); got != manifest.ExampleSource {
		t.Errorf("DefaultSource = %q, want %q", got, manifest.ExampleSource)
	}
	if len(m.Sources) != 0 {
		t.Errorf("Sources = %v, want none: the embedded source is implicit", m.Sources)
	}
	for _, ref := range m.Modules {
		if ref.Source != manifest.ExampleSource {
			t.Errorf("%q comes from source %q, want %q", ref.Name, ref.Source, manifest.ExampleSource)
		}
	}
}

func TestDefaultSourceIsTheFirstListed(t *testing.T) {
	m, err := manifest.Parse([]byte(canonicalSources))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := m.DefaultSource(); got != "house" {
		t.Errorf("DefaultSource = %q, want %q (the first listed)", got, "house")
	}
}

func TestParseMixedBareAndQualifiedNames(t *testing.T) {
	m, err := manifest.Parse([]byte(canonicalSources))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []manifest.ModuleRef{
		{Source: "house", Name: "core"},
		{Source: "local", Name: "go"},
		{Source: "house", Name: "principles"},
	}
	if !reflect.DeepEqual(m.Modules, want) {
		t.Errorf("Modules = %v, want %v (bare entries take the default source)", m.Modules, want)
	}
	wantSources := []manifest.Source{
		{Name: "house", Git: "https://example.invalid/house.git", Ref: "9c1f2e"},
		{Name: "local", Path: "../rules"},
	}
	if !reflect.DeepEqual(m.Sources, wantSources) {
		t.Errorf("Sources = %+v, want %+v", m.Sources, wantSources)
	}
}

func TestRoundTripWithSources(t *testing.T) {
	m, err := manifest.Parse([]byte(canonicalSources))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != canonicalSources {
		t.Errorf("round trip =\n%q\nwant\n%q", got, canonicalSources)
	}
}

func TestMarshalQualifiesOnlyNonDefaultSources(t *testing.T) {
	m := manifest.Manifest{
		Sources: []manifest.Source{
			{Name: "house", Git: "https://example.invalid/house.git", Ref: "9c1f2e"},
			{Name: "local", Path: "../rules"},
		},
		Modules: []manifest.ModuleRef{
			{Source: "house", Name: "core"},
			{Source: "local", Name: "go"},
			{Name: "principles"},
		},
	}
	got, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != canonicalSources {
		t.Errorf("Marshal =\n%q\nwant\n%q", got, canonicalSources)
	}
}

// A file may qualify an entry that comes from the default source; the
// canonical form drops the qualifier.
func TestMarshalCanonicalizesAQualifiedDefaultEntry(t *testing.T) {
	in := "sources:\n  - name: house\n    path: ../rules\nmodules:\n  - house/core\n"
	m, err := manifest.Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := (manifest.ModuleRef{Source: "house", Name: "core"}); m.Modules[0] != want {
		t.Errorf("Modules[0] = %+v, want %+v", m.Modules[0], want)
	}
	got, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := "sources:\n  - name: house\n    path: ../rules\nmodules:\n  - core\n"
	if string(got) != want {
		t.Errorf("Marshal =\n%q\nwant\n%q", got, want)
	}
}

func TestMarshalWritesNoSourcesBlockForTheImplicitSource(t *testing.T) {
	m := manifest.Manifest{Modules: []manifest.ModuleRef{
		{Source: manifest.ExampleSource, Name: "core"},
		{Name: "go"},
	}}
	got, err := manifest.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := "modules:\n  - core\n  - go\n"; string(got) != want {
		t.Errorf("Marshal =\n%q\nwant\n%q", got, want)
	}
}

func TestParseSourceLocationErrors(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		names []string
	}{
		{
			"both git and path",
			"sources:\n  - name: house\n    git: https://example.invalid/house.git\n    path: ../rules\nmodules:\n  - core\n",
			[]string{"house"},
		},
		{
			"neither git nor path",
			"sources:\n  - name: house\n    ref: 9c1f2e\nmodules:\n  - core\n",
			[]string{"house"},
		},
		{
			"the embedded example source listed explicitly",
			"sources:\n  - name: example\nmodules:\n  - core\n",
			[]string{"example"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.Parse([]byte(tt.in))
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want one", tt.in)
			}
			if !errors.Is(err, manifest.ErrSourceLocation) {
				t.Errorf("error = %v, want ErrSourceLocation", err)
			}
			for _, name := range tt.names {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("error = %q, want it to name the source %q", err, name)
				}
			}
		})
	}
}

func TestParseSourceNameErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{
			"uppercase source name",
			"sources:\n  - name: House\n    path: ../rules\nmodules:\n  - core\n",
			manifest.ErrInvalidSourceName,
		},
		{
			"empty source name",
			"sources:\n  - path: ../rules\nmodules:\n  - core\n",
			manifest.ErrInvalidSourceName,
		},
		{
			"duplicate source names",
			"sources:\n  - name: house\n    path: ../rules\n  - name: house\n    path: ../other\nmodules:\n  - core\n",
			manifest.ErrDuplicateSource,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.Parse([]byte(tt.in))
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want one", tt.in)
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestParseRejectsUnknownFieldInSource(t *testing.T) {
	in := "sources:\n  - name: house\n    path: ../rules\n    branch: main\nmodules:\n  - core\n"
	if _, err := manifest.Parse([]byte(in)); err == nil {
		t.Fatal("Parse = nil error, want one: `branch` is not a source key")
	}
}

func TestParseUnknownSource(t *testing.T) {
	in := "sources:\n  - name: house\n    path: ../rules\nmodules:\n  - core\n  - nowhere/go\n"
	_, err := manifest.Parse([]byte(in))
	if err == nil {
		t.Fatal("Parse = nil error, want one")
	}
	if !errors.Is(err, manifest.ErrUnknownSource) {
		t.Errorf("error = %v, want ErrUnknownSource", err)
	}
	if !strings.Contains(err.Error(), "nowhere") {
		t.Errorf("error = %q, want it to name the unknown source", err)
	}
}

func TestParseUnknownSourceUnderTheImplicitSource(t *testing.T) {
	_, err := manifest.Parse([]byte("modules:\n  - house/core\n"))
	if !errors.Is(err, manifest.ErrUnknownSource) {
		t.Errorf("error = %v, want ErrUnknownSource: nothing but the example source exists", err)
	}
}

func TestParseCollisionNamesBothQualifiedNames(t *testing.T) {
	in := "sources:\n  - name: house\n    path: ../rules\n  - name: local\n    path: ../other\nmodules:\n  - go\n  - local/go\n"
	_, err := manifest.Parse([]byte(in))
	if err == nil {
		t.Fatal("Parse = nil error, want one: two sources supply `go`")
	}
	if !errors.Is(err, manifest.ErrModuleCollision) {
		t.Errorf("error = %v, want ErrModuleCollision", err)
	}
	for _, qualified := range []string{"house/go", "local/go"} {
		if !strings.Contains(err.Error(), qualified) {
			t.Errorf("error = %q, want it to name %q", err, qualified)
		}
	}
}

func TestParseDuplicateOfTheSamePairStaysADuplicate(t *testing.T) {
	in := "sources:\n  - name: house\n    path: ../rules\nmodules:\n  - go\n  - house/go\n"
	_, err := manifest.Parse([]byte(in))
	if !errors.Is(err, manifest.ErrDuplicateModule) {
		t.Errorf("error = %v, want ErrDuplicateModule: the same (source, name) twice", err)
	}
}

func TestParseRejectsMalformedModuleEntries(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"mapping entry", "modules:\n  - name: core\n"},
		{"fractional entry", "modules:\n  - 3.5\n"},
		{"empty source half", "modules:\n  - /core\n"},
		{"three segments", "modules:\n  - a/b/c\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := manifest.Parse([]byte(tt.in)); err == nil {
				t.Fatalf("Parse(%q) = nil error, want one", tt.in)
			}
		})
	}
}

func TestModuleRefString(t *testing.T) {
	tests := []struct {
		ref  manifest.ModuleRef
		want string
	}{
		{manifest.ModuleRef{Source: "house", Name: "go"}, "house/go"},
		{manifest.ModuleRef{Name: "go"}, "go"},
	}
	for _, tt := range tests {
		if got := tt.ref.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		in      string
		want    manifest.ModuleRef
		wantErr bool
	}{
		{in: "go", want: manifest.ModuleRef{Name: "go"}},
		{in: "house/go", want: manifest.ModuleRef{Source: "house", Name: "go"}},
		{in: "House/go", wantErr: true},
		{in: "house/Go", wantErr: true},
		{in: "", wantErr: true},
		{in: "a/b/c", wantErr: true},
	}
	for _, tt := range tests {
		got, err := manifest.ParseRef(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseRef(%q) = %+v, want an error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRef(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseRef(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

// Ref is what `agents add <module>` needs: a bare argument means the default
// source, so it matches the entry the manifest already holds.
func TestRefResolvesAgainstTheDefaultSource(t *testing.T) {
	m, err := manifest.Parse([]byte(canonicalSources))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	tests := []struct {
		in      string
		want    manifest.ModuleRef
		wantErr bool
	}{
		{in: "principles", want: manifest.ModuleRef{Source: "house", Name: "principles"}},
		{in: "house/principles", want: manifest.ModuleRef{Source: "house", Name: "principles"}},
		{in: "local/go", want: manifest.ModuleRef{Source: "local", Name: "go"}},
		{in: "nowhere/go", wantErr: true},
		{in: "Go", wantErr: true},
	}
	for _, tt := range tests {
		got, err := m.Ref(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Ref(%q) = %+v, want an error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Ref(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Ref(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestRefUnderTheImplicitSource(t *testing.T) {
	m, err := manifest.Parse([]byte(canonical))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := m.Ref("go")
	if err != nil {
		t.Fatalf("Ref: %v", err)
	}
	if want := (manifest.ModuleRef{Source: manifest.ExampleSource, Name: "go"}); got != want {
		t.Errorf("Ref(go) = %+v, want %+v", got, want)
	}
}

func TestEntriesAreTheFileForm(t *testing.T) {
	m, err := manifest.Parse([]byte(canonicalSources))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"core", "local/go", "principles"}
	if got := m.Entries(); !reflect.DeepEqual(got, want) {
		t.Errorf("Entries = %v, want %v", got, want)
	}
}

func TestSourceByName(t *testing.T) {
	m, err := manifest.Parse([]byte(canonicalSources))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, ok := m.SourceByName("local")
	if !ok {
		t.Fatal("SourceByName(local) = not found, want the source")
	}
	if got.Path != "../rules" {
		t.Errorf("SourceByName(local).Path = %q, want %q", got.Path, "../rules")
	}
	if _, ok := m.SourceByName("nowhere"); ok {
		t.Error("SourceByName(nowhere) = found, want not found")
	}
}

func TestResolveWithLooksModulesUpPerSource(t *testing.T) {
	m, err := manifest.Parse([]byte("sources:\n  - name: house\n    path: ../rules\n  - name: local\n    path: ../other\nmodules:\n  - core\n  - local/go\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	set := testSet(t)
	var asked []manifest.ModuleRef
	got, err := manifest.ResolveWith(m, func(ref manifest.ModuleRef) (module.Module, bool) {
		asked = append(asked, ref)
		if ref.Source != "local" {
			return module.Module{}, false
		}
		return set.Get(ref.Name)
	})
	if err == nil {
		t.Fatalf("ResolveWith = %v, want an error: house has no modules", got)
	}
	want := []manifest.ModuleRef{{Source: "house", Name: "core"}, {Source: "local", Name: "go"}}
	if !reflect.DeepEqual(asked, want) {
		t.Errorf("looked up %v, want %v", asked, want)
	}
	var unknown *manifest.UnknownModulesError
	if !errors.As(err, &unknown) {
		t.Fatalf("error = %v, want an *UnknownModulesError", err)
	}
	if len(unknown.Refs) != 1 || unknown.Refs[0].String() != "house/core" {
		t.Errorf("UnknownModulesError.Refs = %v, want [house/core]", unknown.Refs)
	}
	if !strings.Contains(err.Error(), "house/core") {
		t.Errorf("error = %q, want the qualified name", err)
	}
}
