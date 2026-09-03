package agents

import (
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

// whenSituation is the phrase the example file module declares; the index
// region renders it verbatim, so a change here is a change to what agents read.
const whenSituation = "Writing or editing an agents module"

// exampleModules loads the example source's modules the way the repo layer
// does: through module.Load over the source's "modules" subtree.
func exampleModules(t *testing.T) module.Set {
	t.Helper()
	sub, err := fs.Sub(Example(), "modules")
	if err != nil {
		t.Fatalf("modules subtree: %v", err)
	}
	set, err := module.Load(sub)
	if err != nil {
		t.Fatalf("loading example modules: %v", err)
	}
	return set
}

func TestExampleSourceHoldsExactlyTwoModules(t *testing.T) {
	entries, err := fs.ReadDir(Example(), "modules")
	if err != nil {
		t.Fatalf("reading modules: %v", err)
	}
	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	if want := []string{"agents.md", "module-authoring.md"}; !slices.Equal(files, want) {
		t.Errorf("modules/ holds %v, want %v", files, want)
	}

	if got, want := exampleModules(t).Names(), []string{"agents", "module-authoring"}; !slices.Equal(got, want) {
		t.Errorf("loaded modules = %v, want %v", got, want)
	}
}

func TestExampleSourceSeedsAHead(t *testing.T) {
	head, err := fs.ReadFile(Example(), "templates/head.md")
	if err != nil {
		t.Fatalf("templates/head.md: %v", err)
	}
	if strings.TrimSpace(string(head)) == "" {
		t.Error("templates/head.md is empty")
	}
}

func TestExampleAgentsModuleIsInline(t *testing.T) {
	m, ok := exampleModules(t).Get("agents")
	if !ok {
		t.Fatal("no agents module")
	}
	if m.Kind != module.KindInline {
		t.Errorf("kind = %v, want %v", m.Kind, module.KindInline)
	}
	if m.Path != "" {
		t.Errorf("path = %q, want empty", m.Path)
	}
	if m.When != "" {
		t.Errorf("when = %q, want empty", m.When)
	}
}

func TestExampleModuleAuthoringIsSituational(t *testing.T) {
	m, ok := exampleModules(t).Get("module-authoring")
	if !ok {
		t.Fatal("no module-authoring module")
	}
	if m.Kind != module.KindFile {
		t.Errorf("kind = %v, want %v", m.Kind, module.KindFile)
	}
	if m.When != whenSituation {
		t.Errorf("when = %q, want %q", m.When, whenSituation)
	}
	if want := "project/agents/module-authoring.md"; m.Path != want {
		t.Errorf("path = %q, want %q", m.Path, want)
	}
}

// The example source ships in the binary to every repo, so it must name no
// person, no repository and no stack.
func TestExampleModulesNameNobodyAndNothing(t *testing.T) {
	forbidden := []string{
		"Ben", "Syverson", "bensyverson", "agents-md",
		"Go", "Swift", "SwiftUI", "Python", "github.com",
	}
	set := exampleModules(t)
	for _, name := range set.Names() {
		m, _ := set.Get(name)
		for _, term := range forbidden {
			if strings.Contains(m.Body, term) {
				t.Errorf("%s: body contains %q", name, term)
			}
		}
	}
}
