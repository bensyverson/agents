package render

import "testing"

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
	}{
		{"identical", "one\ntwo\n", "one\ntwo\n", ""},
		{"both empty", "", "", ""},
		{
			"one line changed",
			"one\ntwo\nthree\n",
			"one\n2\nthree\n",
			" one\n-two\n+2\n three\n",
		},
		{
			"line appended",
			"one\n",
			"one\ntwo\n",
			" one\n+two\n",
		},
		{
			"line deleted",
			"one\ntwo\n",
			"two\n",
			"-one\n two\n",
		},
		{
			"from empty",
			"",
			"one\n",
			"+one\n",
		},
		{
			"unchanged runs beyond the context are elided",
			"l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n",
			"changed\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n",
			"-l1\n+changed\n l2\n l3\n l4\n...\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Diff(tt.a, tt.b); got != tt.want {
				t.Errorf("Diff:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}
