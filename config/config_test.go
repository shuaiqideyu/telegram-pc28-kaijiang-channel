package config

import "testing"

func TestParseBoolEnv(t *testing.T) {
	cases := map[string]bool{
		"1":     true,
		"true":  true,
		"YES":   true,
		"on":    true,
		"":      false,
		"0":     false,
		"false": false,
		"debug": false,
	}
	for raw, want := range cases {
		if got := parseBoolEnv(raw); got != want {
			t.Fatalf("%q: got=%v want=%v", raw, got, want)
		}
	}
}
