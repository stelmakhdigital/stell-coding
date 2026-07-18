package tui

import "testing"

func TestParseUserBashInput(t *testing.T) {
	tests := []struct {
		in      string
		cmd     string
		exclude bool
		ok      bool
	}{
		{"!ls", "ls", false, true},
		{"!!ls", "ls", true, true},
		{"! ls -la", "ls -la", false, true},
		{"!!  pwd", "pwd", true, true},
		{"!", "", false, false},
		{"!!", "", true, false},
		{"hello", "", false, false},
	}
	for _, tc := range tests {
		cmd, exclude, ok := parseUserBashInput(tc.in)
		if cmd != tc.cmd || exclude != tc.exclude || ok != tc.ok {
			t.Fatalf("parseUserBashInput(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tc.in, cmd, exclude, ok, tc.cmd, tc.exclude, tc.ok)
		}
	}
}
