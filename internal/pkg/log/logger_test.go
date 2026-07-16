package log

import "testing"

func TestParseLevel(t *testing.T) {
	tests := map[string]int{
		"debug": levelDebug,
		"info":  levelInfo,
		"warn":  levelWarn,
		"error": levelError,
	}
	for input, want := range tests {
		if got := parseLevel(input); got != want {
			t.Fatalf("parseLevel(%q) = %d, want %d", input, got, want)
		}
	}
}
