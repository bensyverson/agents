package render

import (
	"fmt"
	"strings"

	"github.com/bensyverson/agents/internal/module"
)

// The generated situational index. It is the one region whose body comes from
// the manifest rather than from a module file: a table of the situations the
// enabled modules declare and the file to read in each.
const (
	// IndexName is the region's name, reserved so no module can collide with it.
	IndexName     = module.ReservedName
	indexHeading  = "## Situational instructions"
	indexPreamble = "These files carry instructions for specific situations. " +
		"When one applies, read the file before acting and follow it."
	indexHeader = "| Situation | File |\n|---|---|\n"
)

// Index synthesises the index of mods as a module: a name, a body and a hash,
// so every part of the tool that already handles regions — rendering,
// staleness, hand edits, diffs, removal — handles the index without knowing it
// is generated. Rows are the modules carrying a when: phrase, in the order
// given, which is the manifest's.
//
// ok is false when no module is situational; the document must then hold no
// index region, which the caller gets for free by not passing one to Render.
func Index(mods []module.Module) (module.Module, bool) {
	var rows strings.Builder
	for _, m := range mods {
		if m.When == "" {
			continue
		}
		rows.WriteString("| " + cell(m.When) + " | `" + m.Path + "` |\n")
	}
	if rows.Len() == 0 {
		return module.Module{}, false
	}
	body := indexHeading + "\n\n" + indexPreamble + "\n\n" + indexHeader + rows.String()
	return module.Module{
		Name: IndexName,
		Kind: module.KindInline,
		Body: body,
		Hash: module.Hash(body),
	}, true
}

// cell escapes what would otherwise end the table cell early.
func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// SetRegion renders m into doc's region of that name and leaves every other
// byte alone: the other regions, stale or hand-edited, and all project-owned
// text. A document with no such region gets one after its last region.
//
// It is what Render is not — a change to one region rather than to the whole
// document — which is what `remove` needs to keep the index current without
// syncing the regions it was told not to touch.
func SetRegion(doc string, m module.Module) (string, error) {
	d, err := Parse(doc)
	if err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}
	if err := validate([]module.Module{m}); err != nil {
		return "", err
	}

	last := -1
	for i, s := range d.Segments {
		if s.Kind == SegmentRegion {
			last = i
		}
	}

	var out strings.Builder
	found := false
	for i, s := range d.Segments {
		switch s.Kind {
		case SegmentText:
			out.WriteString(s.Text)
		case SegmentRegion:
			if s.Region.Name == m.Name {
				out.WriteString(regionFor(m))
				found = true
			} else {
				out.WriteString(s.Region.String())
			}
		}
		// Parse rejects a duplicate region name, so by the last region the
		// document has said whether it holds this one.
		if i == last && !found {
			appendRegions(&out, []module.Module{m})
		}
	}
	if last < 0 {
		appendRegions(&out, []module.Module{m})
	}
	return out.String(), nil
}
