package render

import (
	"reflect"
	"strings"
	"testing"
)

const (
	begCore = "<!-- agents:begin core@aaaaaa -->\n"
	endCore = "<!-- agents:end core -->\n"
	begGo   = "<!-- agents:begin go@bbbbbb -->\n"
	endGo   = "<!-- agents:end go -->\n"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []Segment
	}{
		{"empty document", "", nil},
		{"text only", "# Head\n\nbody\n", []Segment{textSeg("# Head\n\nbody\n")}},
		{"no trailing newline", "tail", []Segment{textSeg("tail")}},
		{
			"region only",
			begCore + "alpha\n" + endCore,
			[]Segment{regionSeg("core", "aaaaaa", "alpha\n", 1, 3)},
		},
		{
			"head region tail",
			"# H\n\n" + begCore + "alpha\n" + endCore + "\ntail\n",
			[]Segment{
				textSeg("# H\n\n"),
				regionSeg("core", "aaaaaa", "alpha\n", 3, 5),
				textSeg("\ntail\n"),
			},
		},
		{
			"two regions separated by a blank line",
			begCore + "a\n" + endCore + "\n" + begGo + "b\n" + endGo,
			[]Segment{
				regionSeg("core", "aaaaaa", "a\n", 1, 3),
				textSeg("\n"),
				regionSeg("go", "bbbbbb", "b\n", 5, 7),
			},
		},
		{
			"empty region body",
			begCore + endCore,
			[]Segment{regionSeg("core", "aaaaaa", "", 1, 2)},
		},
		{
			"multi-line body with a blank line",
			begCore + "a\n\nb\n" + endCore,
			[]Segment{regionSeg("core", "aaaaaa", "a\n\nb\n", 1, 5)},
		},
		{
			"marker text inside prose is not a marker",
			"The regions between `<!-- agents:begin core@aaaaaa -->` are generated.\n",
			[]Segment{textSeg("The regions between `<!-- agents:begin core@aaaaaa -->` are generated.\n")},
		},
		{
			"indented marker is not a marker",
			"  " + begCore,
			[]Segment{textSeg("  " + begCore)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Parse(tt.doc)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(d.Segments, tt.want) {
				t.Errorf("segments:\n got %#v\nwant %#v", d.Segments, tt.want)
			}
			if got := d.String(); got != tt.doc {
				t.Errorf("round trip: got %q want %q", got, tt.doc)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string // substrings the error must contain
	}{
		{"begin with no end", begCore + "a\n", []string{"core", "line 1"}},
		{"end with no begin", "x\n" + endCore, []string{"core", "line 2"}},
		{"end name does not match", begCore + "a\n" + endGo, []string{"go", "core", "line 3"}},
		{"nested regions", begCore + begGo + endGo + endCore, []string{"go", "core", "line 2"}},
		{"duplicate region names", begCore + endCore + begCore + endCore, []string{"core", "line 3"}},
		{"malformed begin: uppercase hash", "<!-- agents:begin core@AAAAAA -->\n", []string{"line 1"}},
		{"malformed begin: no hash", "<!-- agents:begin core -->\n", []string{"line 1"}},
		{"malformed begin: uppercase name", "<!-- agents:begin Core@aaaaaa -->\n", []string{"line 1"}},
		{"malformed end: trailing space", begCore + "<!-- agents:end core --> \n", []string{"line 2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.doc)
			if err == nil {
				t.Fatalf("Parse(%q): want error, got nil", tt.doc)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestDocumentRegions(t *testing.T) {
	d, err := Parse("# H\n\n" + begCore + "a\n" + endCore + "\n" + begGo + "b\n" + endGo + "\ntail\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := d.Regions()
	if len(got) != 2 || got[0].Name != "core" || got[1].Name != "go" {
		t.Fatalf("Regions() = %#v", got)
	}
}
