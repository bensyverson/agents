package gotchas

import "strings"

// ruleLine is the horizontal rule that separates a gotchas file's preamble —
// the instructions, which the template owns — from its entries, which the
// project owns.
const ruleLine = "---"

// Reseed replaces content's preamble — everything above its first rule line —
// with template's, and reports whether that changed anything. Entries are
// never touched: the rule line and everything below it is copied byte for
// byte.
//
// A file with no rule line at all is the pre-template shape, from before the
// preamble had a boundary. Every byte of it is treated as content the project
// wrote, so the template's preamble and a rule line go above the lot rather
// than over any of it.
func Reseed(content, template string) (string, bool) {
	preamble := preambleOf(template)
	_, entries, found := cutAtRule(content)
	if found {
		out := preamble + entries
		return out, out != content
	}
	out := preamble + ruleLine + "\n"
	if body := strings.TrimLeft(content, "\n"); body != "" {
		out += "\n" + body
	}
	return out, out != content
}

// preambleOf is everything above s's first rule line, newline-terminated. A
// template with no rule line of its own is all preamble.
func preambleOf(s string) string {
	head, _, found := cutAtRule(s)
	if !found {
		head = s
	}
	if head != "" && !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return head
}

// cutAtRule splits s at the first line that is a bare rule; rest begins with
// that line. When there is none, head is all of s and found is false.
func cutAtRule(s string) (head, rest string, found bool) {
	for offset := 0; offset < len(s); {
		end := len(s)
		if i := strings.IndexByte(s[offset:], '\n'); i >= 0 {
			end = offset + i + 1
		}
		if strings.TrimSpace(s[offset:end]) == ruleLine {
			return s[:offset], s[offset:], true
		}
		offset = end
	}
	return s, "", false
}
