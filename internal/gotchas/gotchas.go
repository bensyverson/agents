// Package gotchas parses project/gotchas.md, the checked-in convention where
// agents record project traps and feedback about the shared rules.
//
// The format is deliberately loose — humans and agents append to it by hand —
// so parsing is liberal, but the shape it recognises is exact:
//
//   - An entry starts at an H2 line ("## …") whose text begins with a
//     YYYY-MM-DD date. It runs to the next such H2, or to end of file.
//   - Everything before the first dated H2 (the preamble and its "---" rule) is
//     ignored, as is any H2 without a leading date and any heading at another
//     level; such lines simply belong to the body of the entry they sit in.
//   - An H2 that looks dated but is not (## 2026-13-45 …) is skipped, silently:
//     a typo must not turn the file into an error.
package gotchas

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	// dateLayout is the date every entry heading starts with.
	dateLayout = "2006-01-02"
	// entryPrefix opens an entry heading; deeper or shallower headings are body.
	entryPrefix = "## "
	// rulePrefix marks an entry as feedback about AGENTS.md rather than a trap.
	rulePrefix = "rule:"
	// headlineSeparators are the leaders humans put between the date and the
	// headline ("2026-08-16 — the thing", "2026-08-16: the thing").
	headlineSeparators = " \t—–-:·"
)

// boldSpan matches the first **bold** run on a line — the conventional headline
// when the H2 carries only a date.
var boldSpan = regexp.MustCompile(`\*\*([^\n]+?)\*\*`)

// Entry is one dated gotcha.
type Entry struct {
	// Date is the date in the H2, parsed in UTC.
	Date time.Time
	// Headline is the H2 text after the date, or else the first bold span in
	// the body, or else empty. A "rule:" prefix is left in place.
	Headline string
	// Body is everything below the H2 line, with surrounding blank lines trimmed.
	Body string
	// Rule reports feedback about AGENTS.md itself: the headline — or, when the
	// H2 has none, the first bold span — starts with "rule:".
	Rule bool
	// Line is the 1-based line number of the H2.
	Line int
}

// Entries is a parsed gotchas file, with the summary helpers `agents status` needs.
type Entries []Entry

// Oldest is the earliest entry date, and false when there are no entries.
func (e Entries) Oldest() (time.Time, bool) {
	var oldest time.Time
	for i, entry := range e {
		if i == 0 || entry.Date.Before(oldest) {
			oldest = entry.Date
		}
	}
	return oldest, len(e) > 0
}

// Rules are the entries flagged as feedback about the shared rules.
func (e Entries) Rules() Entries {
	var rules Entries
	for _, entry := range e {
		if entry.Rule {
			rules = append(rules, entry)
		}
	}
	return rules
}

// Read parses the gotchas file at path. A missing file yields a zero File and
// exists false: callers distinguish "no gotchas file" from "a file with no
// entries" by exists.
func Read(path string) (file File, exists bool, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return File{}, false, nil
		}
		return File{}, false, fmt.Errorf("reading gotchas %s: %w", path, err)
	}
	return Parse(string(content)), true, nil
}

// Parse returns the entries in a gotchas file, in file order, with the size of
// the file they came from — the two numbers the budget is measured against.
func Parse(content string) File {
	return File{Entries: parseEntries(content), Lines: countLines(content)}
}

// parseEntries walks the file's lines, collecting each dated H2 and its body.
func parseEntries(content string) Entries {
	lines := strings.Split(content, "\n")
	var entries Entries
	var current *Entry
	var body []string

	flush := func() {
		if current != nil {
			current.Body = strings.Trim(strings.Join(body, "\n"), "\n")
			current.finishHeadline()
			entries = append(entries, *current)
		}
		current, body = nil, nil
	}

	for i, line := range lines {
		date, rest, ok := parseHeading(line)
		if !ok {
			if current != nil {
				body = append(body, line)
			}
			continue
		}
		flush()
		current = &Entry{Date: date, Headline: rest, Rule: isRule(rest), Line: i + 1}
	}
	flush()
	return entries
}

// parseHeading reports whether line opens an entry, returning its date and the
// heading text after it (trimmed of the usual separators).
func parseHeading(line string) (time.Time, string, bool) {
	text, ok := strings.CutPrefix(strings.TrimRight(line, " \t"), entryPrefix)
	if !ok {
		return time.Time{}, "", false
	}
	text = strings.TrimSpace(text)
	if len(text) < len(dateLayout) {
		return time.Time{}, "", false
	}
	date, err := time.Parse(dateLayout, text[:len(dateLayout)])
	if err != nil {
		return time.Time{}, "", false
	}
	rest := strings.TrimSpace(strings.TrimLeft(text[len(dateLayout):], headlineSeparators))
	return date, rest, true
}

// finishHeadline falls back to the body's first bold span when the H2 carried
// only a date, and flags a rule found in either place.
func (e *Entry) finishHeadline() {
	bold := ""
	if match := boldSpan.FindStringSubmatch(e.Body); match != nil {
		bold = strings.TrimSpace(match[1])
	}
	if e.Headline == "" {
		e.Headline = bold
	}
	e.Rule = e.Rule || isRule(bold)
}

func isRule(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), rulePrefix)
}
