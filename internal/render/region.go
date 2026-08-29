// Package render turns a document (an AGENTS.md, or a whole generated file)
// into marked regions plus project-owned text, and renders modules back into
// it. It works on strings only; walking the file system is the caller's job.
package render

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bensyverson/agents/internal/module"
)

// Marker syntax. A marker must occupy a whole line; anything else in the
// document belongs to the project and is preserved verbatim.
const (
	beginPrefix  = "<!-- agents:begin "
	endPrefix    = "<!-- agents:end "
	markerSuffix = " -->"
)

const (
	namePattern = `[a-z0-9][a-z0-9-]*`
	hashDigit   = `[0-9a-f]`
)

var (
	// hashClass is the marker hash pattern, sized from the module hash length
	// so the two can never drift apart.
	hashClass = fmt.Sprintf("%s{%d}", hashDigit, module.HashLength)

	beginRE = regexp.MustCompile(`^` + beginPrefix + `(` + namePattern + `)@(` + hashClass + `)` + markerSuffix + `$`)
	endRE   = regexp.MustCompile(`^` + endPrefix + `(` + namePattern + `)` + markerSuffix + `$`)
	nameRE  = regexp.MustCompile(`^` + namePattern + `$`)
	hashRE  = regexp.MustCompile(`^` + hashClass + `$`)
)

// SegmentKind distinguishes project-owned text from a generated region.
type SegmentKind int

const (
	// SegmentText is project-owned text, opaque to the tool.
	SegmentText SegmentKind = iota
	// SegmentRegion is a begin/end marker pair and its body.
	SegmentRegion
)

// Segment is one piece of a parsed document.
type Segment struct {
	Kind SegmentKind
	// Text holds the verbatim bytes of a SegmentText, including newlines.
	Text string
	// Region holds the parsed region of a SegmentRegion.
	Region Region
}

// Region is one rendered module inside a document.
type Region struct {
	Name string
	// MarkerHash is the module hash recorded in the begin marker at render time.
	MarkerHash string
	// Body is everything strictly between the markers; it ends in "\n" unless empty.
	Body string
	// BeginLine and EndLine are 1-based line numbers of the marker lines.
	BeginLine int
	EndLine   int
}

// String renders the region in canonical marker form.
func (r Region) String() string {
	return beginPrefix + r.Name + "@" + r.MarkerHash + markerSuffix + "\n" +
		r.Body + endPrefix + r.Name + markerSuffix + "\n"
}

// Document is a document parsed into ordered segments.
type Document struct {
	Segments []Segment
}

// String reassembles the document; parsing and reassembling round-trips exactly.
func (d *Document) String() string {
	var b strings.Builder
	for _, s := range d.Segments {
		if s.Kind == SegmentText {
			b.WriteString(s.Text)
		} else {
			b.WriteString(s.Region.String())
		}
	}
	return b.String()
}

// Regions returns the document's regions in document order.
func (d *Document) Regions() []Region {
	var out []Region
	for _, s := range d.Segments {
		if s.Kind == SegmentRegion {
			out = append(out, s.Region)
		}
	}
	return out
}

// Parse splits doc into text and region segments. A line that looks like a
// marker but is not in canonical form is an error rather than text: silently
// treating it as prose would let a typo destroy the region on the next sync.
func Parse(doc string) (*Document, error) {
	var (
		d    Document
		text strings.Builder
		body strings.Builder
		open *Region
	)
	openedAt := map[string]int{}
	flushText := func() {
		if text.Len() > 0 {
			d.Segments = append(d.Segments, Segment{Kind: SegmentText, Text: text.String()})
			text.Reset()
		}
	}

	for i, raw := range splitLines(doc) {
		line := i + 1
		content := strings.TrimSuffix(raw, "\n")
		switch {
		case strings.HasPrefix(content, beginPrefix):
			m := beginRE.FindStringSubmatch(content)
			if m == nil {
				return nil, fmt.Errorf("malformed begin marker at line %d: %q", line, content)
			}
			if open != nil {
				return nil, fmt.Errorf("region %q at line %d opens inside region %q (opened at line %d)", m[1], line, open.Name, open.BeginLine)
			}
			if prev, dup := openedAt[m[1]]; dup {
				return nil, fmt.Errorf("duplicate region %q at line %d (already rendered at line %d)", m[1], line, prev)
			}
			openedAt[m[1]] = line
			flushText()
			open = &Region{Name: m[1], MarkerHash: m[2], BeginLine: line}
		case strings.HasPrefix(content, endPrefix):
			m := endRE.FindStringSubmatch(content)
			if m == nil {
				return nil, fmt.Errorf("malformed end marker at line %d: %q", line, content)
			}
			if open == nil {
				return nil, fmt.Errorf("end marker for region %q at line %d has no begin marker", m[1], line)
			}
			if m[1] != open.Name {
				return nil, fmt.Errorf("end marker for region %q at line %d does not close region %q (opened at line %d)", m[1], line, open.Name, open.BeginLine)
			}
			open.Body = body.String()
			open.EndLine = line
			d.Segments = append(d.Segments, Segment{Kind: SegmentRegion, Region: *open})
			body.Reset()
			open = nil
		default:
			if open != nil {
				body.WriteString(raw)
			} else {
				text.WriteString(raw)
			}
		}
	}
	if open != nil {
		return nil, fmt.Errorf("region %q opened at line %d has no end marker", open.Name, open.BeginLine)
	}
	flushText()
	return &d, nil
}

// splitLines splits s into lines, each keeping its trailing "\n" (the last one
// has none if s does not end in a newline), so text survives byte-for-byte.
func splitLines(s string) []string {
	var out []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}
