package render

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/bensyverson/agents/internal/module"
)

// testHash re-implements the module content hash independently of the package
// under test, so the tests cannot agree with the implementation by construction.
func testHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:6]
}

func mod(name, body string) module.Module {
	return module.Module{Name: name, Kind: module.KindInline, Body: body, Hash: testHash(body)}
}

// fileMod is a kind:file module, optionally situational. The path is spelled
// out rather than taken from the module package, so the tests pin the
// convention the index prints.
func fileMod(name, when, body string) module.Module {
	return module.Module{
		Name: name,
		Kind: module.KindFile,
		Path: "project/agents/" + name + ".md",
		When: when,
		Body: body,
		Hash: testHash(body),
	}
}

// wantRegion is the exact text a rendered region for m must have.
func wantRegion(m module.Module) string {
	return markedRegion(m.Name, m.Hash, m.Body)
}

// markedRegion builds a region whose begin marker carries an arbitrary hash,
// for stale and hand-edited fixtures.
func markedRegion(name, markerHash, body string) string {
	return "<!-- agents:begin " + name + "@" + markerHash + " -->\n" + body + "<!-- agents:end " + name + " -->\n"
}

func mustRender(t *testing.T, doc string, mods []module.Module) string {
	t.Helper()
	got, err := Render(doc, mods)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return got
}

func mustInspect(t *testing.T, doc string, mods []module.Module) Report {
	t.Helper()
	rep, err := Inspect(doc, mods)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	return rep
}

func textSeg(s string) Segment { return Segment{Kind: SegmentText, Text: s} }

func regionSeg(name, hash, body string, begin, end int) Segment {
	return Segment{Kind: SegmentRegion, Region: Region{
		Name: name, MarkerHash: hash, Body: body, BeginLine: begin, EndLine: end,
	}}
}

// stripFrontmatter removes a leading ---\n…\n---\n block, the way the module
// loader is specified to.
func stripFrontmatter(s string) string {
	const fence = "---\n"
	if !strings.HasPrefix(s, fence) {
		return s
	}
	rest := s[len(fence):]
	if i := strings.Index(rest, "\n"+fence); i >= 0 {
		return rest[i+1+len(fence):]
	}
	return s
}
