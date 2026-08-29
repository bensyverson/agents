package gotchas

import (
	"strings"
	"testing"
)

// The shape of the shipped template: prose, then the rule that separates the
// preamble from the entries.
const testTemplate = `# Gotchas

Project-specific traps that cost real time.

- **Delete anything that becomes obvious.** Keep this list short.

Format: one dated H2 per entry, a bold headline, then one paragraph.

---
`

const testEntries = `
## 2026-08-16 — First headline

**Bold thing.** What happened, and what to do instead.

## 2026-08-10: Second headline

Second body.
`

func TestReseedIdenticalPreambleIsANoOp(t *testing.T) {
	content := testTemplate + testEntries
	got, changed := Reseed(content, testTemplate)
	if changed {
		t.Error("changed = true on a file whose preamble already matches the template")
	}
	if got != content {
		t.Errorf("content rewritten:\n%q", got)
	}
}

func TestReseedReplacesAnOldPreambleAndKeepsEntries(t *testing.T) {
	const old = `# Gotchas

Things that bit us.

---
`
	got, changed := Reseed(old+testEntries, testTemplate)
	if !changed {
		t.Fatal("changed = false over a stale preamble")
	}
	if want := testTemplate + testEntries; got != want {
		t.Errorf("Reseed =\n%q\nwant\n%q", got, want)
	}
	if !strings.Contains(got, "Format: one dated H2 per entry") {
		t.Error("the template's prune instruction was not installed")
	}
	if !strings.HasSuffix(got, testEntries) {
		t.Errorf("entries were not preserved byte-for-byte:\n%q", got)
	}
}

// Only the first rule is the preamble boundary: a "---" inside an entry's body
// must not be mistaken for it.
func TestReseedStopsAtTheFirstRule(t *testing.T) {
	const body = `
## 2026-08-16 — an entry with a rule in it

before

---

after
`
	got, changed := Reseed("# Old\n\n---\n"+body, testTemplate)
	if !changed {
		t.Fatal("changed = false over a stale preamble")
	}
	if want := testTemplate + body; got != want {
		t.Errorf("Reseed =\n%q\nwant\n%q", got, want)
	}
}

// Alpha's file predates the template and has no rule at all: everything in
// it is entries, so the preamble and a rule go above the lot.
func TestReseedFileWithNoRuleGetsPreambleAndRule(t *testing.T) {
	const old = `# Gotchas

Two-line header from before the template.

## 2026-07-27 — the oldest entry

body
`
	got, changed := Reseed(old, testTemplate)
	if !changed {
		t.Fatal("changed = false over a file with no rule")
	}
	if !strings.HasPrefix(got, testTemplate) {
		t.Errorf("template preamble and rule were not prepended:\n%q", got)
	}
	if !strings.Contains(got, "## 2026-07-27 — the oldest entry\n\nbody\n") {
		t.Errorf("the entry was not preserved:\n%q", got)
	}
	// Nothing of the old file is lost — it is all body now.
	if !strings.Contains(got, "Two-line header from before the template.") {
		t.Errorf("old content was dropped:\n%q", got)
	}
	if got != testTemplate+"\n"+old {
		t.Errorf("Reseed =\n%q\nwant\n%q", got, testTemplate+"\n"+old)
	}
	// Reseeding again is a no-op: the file now has a rule and a fresh preamble.
	again, changed := Reseed(got, testTemplate)
	if changed || again != got {
		t.Errorf("second Reseed changed the file: changed=%v\n%q", changed, again)
	}
}

func TestReseedEmptyFile(t *testing.T) {
	got, changed := Reseed("", testTemplate)
	if !changed {
		t.Fatal("changed = false over an empty file")
	}
	if got != testTemplate {
		t.Errorf("Reseed(\"\") = %q, want the template", got)
	}
}

// A template with no rule of its own is all preamble; the rule is still what
// separates it from the entries in the file.
func TestReseedTemplateWithoutARule(t *testing.T) {
	const template = "# Gotchas\n\nseeded\n"
	got, changed := Reseed("# Old\n\n---\n"+testEntries, template)
	if !changed {
		t.Fatal("changed = false over a stale preamble")
	}
	if want := template + "---\n" + testEntries; got != want {
		t.Errorf("Reseed =\n%q\nwant\n%q", got, want)
	}
}
