package musicplugin

import "testing"

func TestMessageMatches(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		result bool
	}{
		{"has prefix", "!play music", true},
		{"doesn't have prefix", "play music", false},
		{"valid prefix but not command", "!do it", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if hasValidCmd(c.line, "!") != c.result {
				t.Fatalf("expcted '%s' to be '%+v'", c.name, c.result)
			}
		})
	}
}
