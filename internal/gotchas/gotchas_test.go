package gotchas

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return d
}

// The seeded template is preamble only: it must parse to zero entries.
func TestParseTemplateHasNoEntries(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "project", "gotchas.md"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	if got := Parse(string(content)).Entries; len(got) != 0 {
		t.Errorf("Parse(template) = %d entries, want 0: %+v", len(got), got)
	}
}

const docTwoEntries = `# Gotchas

Preamble prose that is not an entry.

---

## 2026-08-16 — First headline

**Bold thing.** What happened, and what to do instead.

## 2026-08-10: Second headline

Second body.
`

func TestParseTwoEntriesWithHeadingHeadlines(t *testing.T) {
	got := Parse(docTwoEntries).Entries
	if len(got) != 2 {
		t.Fatalf("Parse = %d entries, want 2: %+v", len(got), got)
	}
	want := []Entry{
		{
			Date:     date(t, "2026-08-16"),
			Headline: "First headline",
			Body:     "**Bold thing.** What happened, and what to do instead.",
			Rule:     false,
			Line:     7,
		},
		{
			Date:     date(t, "2026-08-10"),
			Headline: "Second headline",
			Body:     "Second body.",
			Rule:     false,
			Line:     11,
		},
	}
	for i := range want {
		if !got[i].Date.Equal(want[i].Date) {
			t.Errorf("entry %d Date = %v, want %v", i, got[i].Date, want[i].Date)
		}
		if got[i].Headline != want[i].Headline {
			t.Errorf("entry %d Headline = %q, want %q", i, got[i].Headline, want[i].Headline)
		}
		if got[i].Body != want[i].Body {
			t.Errorf("entry %d Body = %q, want %q", i, got[i].Body, want[i].Body)
		}
		if got[i].Rule != want[i].Rule {
			t.Errorf("entry %d Rule = %v, want %v", i, got[i].Rule, want[i].Rule)
		}
		if got[i].Line != want[i].Line {
			t.Errorf("entry %d Line = %d, want %d", i, got[i].Line, want[i].Line)
		}
	}
}

func TestParseHeadlineFallsBackToBoldSpan(t *testing.T) {
	const doc = `## 2026-08-16

**The bold headline.** Then the story of what happened.

More detail, with a **second bold span** that must not win.
`
	got := Parse(doc).Entries
	if len(got) != 1 {
		t.Fatalf("Parse = %d entries, want 1", len(got))
	}
	if got[0].Headline != "The bold headline." {
		t.Errorf("Headline = %q, want %q", got[0].Headline, "The bold headline.")
	}
}

func TestParseHeadlineEmptyWhenNothingToUse(t *testing.T) {
	const doc = `## 2026-08-16

Just prose, no bold span anywhere.
`
	got := Parse(doc).Entries
	if len(got) != 1 {
		t.Fatalf("Parse = %d entries, want 1", len(got))
	}
	if got[0].Headline != "" {
		t.Errorf("Headline = %q, want empty", got[0].Headline)
	}
}

func TestParseRule(t *testing.T) {
	tests := []struct {
		name         string
		doc          string
		wantRule     bool
		wantHeadline string
	}{
		{
			name:         "rule in the H2 remainder",
			doc:          "## 2026-08-16 — rule: TDD wording misled me\n\n**Not a rule marker.** Body.\n",
			wantRule:     true,
			wantHeadline: "rule: TDD wording misled me",
		},
		{
			name:         "rule in the bold span",
			doc:          "## 2026-08-16\n\n**rule: the go module note is wrong.** Body.\n",
			wantRule:     true,
			wantHeadline: "rule: the go module note is wrong.",
		},
		{
			name:         "rule in the bold span while the H2 has a headline",
			doc:          "## 2026-08-16 — some headline\n\n**rule: still a rule.** Body.\n",
			wantRule:     true,
			wantHeadline: "some headline",
		},
		{
			name:         "case-insensitive and no space after the colon",
			doc:          "## 2026-08-16 RULE:no space here\n\nBody.\n",
			wantRule:     true,
			wantHeadline: "RULE:no space here",
		},
		{
			name:         "plain entry is not a rule",
			doc:          "## 2026-08-16 — ruler of all things\n\n**rules are made to be broken.** Body.\n",
			wantRule:     false,
			wantHeadline: "ruler of all things",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.doc).Entries
			if len(got) != 1 {
				t.Fatalf("Parse = %d entries, want 1", len(got))
			}
			if got[0].Rule != tt.wantRule {
				t.Errorf("Rule = %v, want %v", got[0].Rule, tt.wantRule)
			}
			if got[0].Headline != tt.wantHeadline {
				t.Errorf("Headline = %q, want %q", got[0].Headline, tt.wantHeadline)
			}
		})
	}
}

func TestParseIgnoresUndatedAndBadlyDatedHeadings(t *testing.T) {
	const doc = `## Notes

This heading has no date, so it is not an entry.

## 2026-08-16 — Real entry

Body of the real entry.

## 2026-13-45 — Impossible date

This is part of the real entry's body, not an entry of its own.

### 2026-08-01 — An H3 is not an entry either

Still body.
`
	got := Parse(doc).Entries
	if len(got) != 1 {
		t.Fatalf("Parse = %d entries, want 1: %+v", len(got), got)
	}
	if got[0].Headline != "Real entry" {
		t.Errorf("Headline = %q, want %q", got[0].Headline, "Real entry")
	}
	for _, want := range []string{"Impossible date", "An H3 is not an entry either"} {
		if !strings.Contains(got[0].Body, want) {
			t.Errorf("Body %q does not contain %q", got[0].Body, want)
		}
	}
}

const docSummary = `## 2026-08-16 — Newest

Body.

## 2026-01-02 — Oldest by date, not by position

Body.

## 2026-05-05 — rule: middle

Body.
`

func TestSummaryHelpers(t *testing.T) {
	entries := Parse(docSummary).Entries
	if got := len(entries); got != 3 {
		t.Errorf("Count = %d, want 3", got)
	}
	oldest, ok := entries.Oldest()
	if !ok {
		t.Fatal("Oldest = not ok, want ok")
	}
	if want := date(t, "2026-01-02"); !oldest.Equal(want) {
		t.Errorf("Oldest = %v, want %v", oldest, want)
	}
	rules := entries.Rules()
	if len(rules) != 1 {
		t.Fatalf("Rules = %d entries, want 1: %+v", len(rules), rules)
	}
	if rules[0].Headline != "rule: middle" {
		t.Errorf("Rules[0].Headline = %q, want %q", rules[0].Headline, "rule: middle")
	}
}

func TestSummaryHelpersOnEmpty(t *testing.T) {
	var entries Entries
	if got := len(entries); got != 0 {
		t.Errorf("Count = %d, want 0", got)
	}
	if _, ok := entries.Oldest(); ok {
		t.Error("Oldest on empty = ok, want not ok")
	}
	if got := entries.Rules(); len(got) != 0 {
		t.Errorf("Rules on empty = %v, want empty", got)
	}
}

func TestReadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project", "gotchas.md")
	file, exists, err := Read(path)
	if err != nil {
		t.Fatalf("Read of missing file: %v", err)
	}
	if exists {
		t.Error("Read of missing file: exists = true, want false")
	}
	if file.Entries != nil || file.Lines != 0 {
		t.Errorf("Read of missing file = %+v, want the zero File", file)
	}
}

func TestReadExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gotchas.md")
	if err := os.WriteFile(path, []byte(docTwoEntries), 0o644); err != nil {
		t.Fatal(err)
	}
	file, exists, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !exists {
		t.Error("exists = false, want true")
	}
	if len(file.Entries) != 2 {
		t.Errorf("Read = %d entries, want 2", len(file.Entries))
	}
}

func TestReadEmptyFileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gotchas.md")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	file, exists, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !exists {
		t.Error("exists = false, want true for an empty but present file")
	}
	if len(file.Entries) != 0 {
		t.Errorf("Read = %d entries, want 0", len(file.Entries))
	}
}
