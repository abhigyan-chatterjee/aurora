package pacman

import (
	"os/exec"
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	if _, err := exec.LookPath("vercmp"); err != nil {
		t.Skip("vercmp not found on PATH; skipping version comparison test")
	}

	tests := []struct {
		local    string
		aur      string
		expected bool
	}{
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "", false},
		{"", "1.0.0", false},
		{"", "", false},
		{"1.0.0", "1.0.1", true},
		{"1.0.1", "1.0.0", false},
		{"2.0-1", "2.0-2", true},
		{"2.0-2", "2.0-1", false},
		{"1.0a", "1.0b", true},
	}

	for _, tc := range tests {
		got := IsNewerVersion(tc.local, tc.aur)
		if got != tc.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v, expected %v", tc.local, tc.aur, got, tc.expected)
		}
	}
}
