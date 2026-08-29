package render

import (
	"fmt"
	"strings"

	"github.com/bensyverson/agents/internal/module"
)

// Render rewrites doc so that its regions hold the current modules.
//
// Project-owned text survives byte-for-byte. The region slots already in the
// document are filled, in document order, with the modules of mods that have a
// region there — in mods order, so reordering the manifest reorders the file.
// Surplus slots (modules dropped from the manifest) are removed, and modules
// with no slot are appended after the last region. Render always renders the
// current module body, so a hand-edited region is overwritten; deciding whether
// that is allowed is the caller's job (see Inspect).
func Render(doc string, mods []module.Module) (string, error) {
	d, err := Parse(doc)
	if err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}
	if err := validate(mods); err != nil {
		return "", err
	}

	rendered := map[string]bool{}
	lastRegion := -1
	for i, s := range d.Segments {
		if s.Kind == SegmentRegion {
			rendered[s.Region.Name] = true
			lastRegion = i
		}
	}
	var fill, added []module.Module
	for _, m := range mods {
		if rendered[m.Name] {
			fill = append(fill, m)
		} else {
			added = append(added, m)
		}
	}

	var out strings.Builder
	slot := 0
	// dropBlank swallows the blank line that separated a removed region from
	// what follows, so removal does not leave a doubled gap.
	dropBlank := false
	for i, s := range d.Segments {
		switch s.Kind {
		case SegmentText:
			text := s.Text
			if dropBlank {
				text = strings.TrimPrefix(text, "\n")
				dropBlank = false
			}
			out.WriteString(text)
		case SegmentRegion:
			if slot < len(fill) {
				out.WriteString(regionFor(fill[slot]))
				slot++
				dropBlank = false
			} else {
				dropBlank = true
			}
		}
		if i == lastRegion {
			appendRegions(&out, added)
		}
	}
	if lastRegion < 0 {
		appendRegions(&out, added)
	}
	return out.String(), nil
}

// RenderFile renders a whole generated file, where the file is a single region
// (plus whatever head the project has written above it).
func RenderFile(existing string, m module.Module) (string, error) {
	return Render(existing, []module.Module{m})
}

func appendRegions(out *strings.Builder, mods []module.Module) {
	for _, m := range mods {
		out.WriteString(separator(out.String()))
		out.WriteString(regionFor(m))
	}
}

// separator is what must precede a newly appended region so that exactly one
// blank line sits between it and whatever came before.
func separator(sofar string) string {
	switch {
	case sofar == "", strings.HasSuffix(sofar, "\n\n"):
		return ""
	case strings.HasSuffix(sofar, "\n"):
		return "\n"
	default:
		return "\n\n"
	}
}

func regionFor(m module.Module) string {
	body := m.Body
	// A body without a trailing newline would put the end marker mid-line and
	// corrupt the document; the loader guarantees one, but never trust it.
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return Region{Name: m.Name, MarkerHash: m.Hash, Body: body}.String()
}

// validate rejects modules that cannot be rendered into a re-parseable document.
func validate(mods []module.Module) error {
	seen := map[string]bool{}
	for _, m := range mods {
		if !nameRE.MatchString(m.Name) {
			return fmt.Errorf("module %q: name must match %s", m.Name, nameRE)
		}
		if !hashRE.MatchString(m.Hash) {
			return fmt.Errorf("module %q: hash %q must be %d lowercase hex digits", m.Name, m.Hash, module.HashLength)
		}
		if seen[m.Name] {
			return fmt.Errorf("module %q appears twice", m.Name)
		}
		seen[m.Name] = true
	}
	return nil
}
