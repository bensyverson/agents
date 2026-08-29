package render

import (
	"fmt"
	"slices"
	"strings"
)

// RemoveRegions deletes the named regions from doc and returns the document
// that is left along with the regions it removed, in document order.
//
// Every other byte survives: the project-owned head above the first region, a
// hand-written section between two, the tail below the last. The one thing it
// does drop is the single blank line that separated a removed region from what
// follows, so removal leaves no doubled gap — exactly what Render does when a
// module leaves the manifest, which is what keeps the result a document a
// later sync has nothing to say about.
//
// A name with no region in doc is not an error; the caller sees it missing
// from the returned regions. Neither is a region whose body was hand-edited:
// removing a module is a decision, not a sync, so the body comes back for the
// caller to report and the region goes.
func RemoveRegions(doc string, names ...string) (string, []Region, error) {
	d, err := Parse(doc)
	if err != nil {
		return "", nil, fmt.Errorf("parse document: %w", err)
	}

	var (
		out     strings.Builder
		removed []Region
		// dropBlank swallows the blank line a removed region left behind.
		dropBlank bool
	)
	for _, s := range d.Segments {
		switch s.Kind {
		case SegmentText:
			text := s.Text
			if dropBlank {
				text = strings.TrimPrefix(text, "\n")
				dropBlank = false
			}
			out.WriteString(text)
		case SegmentRegion:
			if slices.Contains(names, s.Region.Name) {
				removed = append(removed, s.Region)
				dropBlank = true
				continue
			}
			out.WriteString(s.Region.String())
			dropBlank = false
		}
	}
	return out.String(), removed, nil
}
