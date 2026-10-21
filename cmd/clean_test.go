package cmd

import (
	"strings"
	"testing"
)

func TestClean_Help(t *testing.T) {
	out := captureStdout(func() {
		_ = cleanCmd.Help()
	})
	if !strings.Contains(out, "clean") {
		t.Errorf("expected clean in help, got: %s", out)
	}
}

func TestClean_DryRun(t *testing.T) {
	cleanDryRunFlag = true
	cleanCacheFlag = true
	cleanPacmanFlag = false
	cleanAllFlag = false

	out := captureStdout(func() {
		clean(cleanCmd, []string{})
	})

	if !strings.Contains(out, "Dry Run") {
		t.Errorf("expected Dry Run in output, got: %s", out)
	}
	if !strings.Contains(out, "AUR Build Cache") {
		t.Errorf("expected AUR Build Cache in dry run output, got: %s", out)
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, tc := range tests {
		got := humanSize(tc.bytes)
		if got != tc.expected {
			t.Errorf("humanSize(%d) = %q, expected %q", tc.bytes, got, tc.expected)
		}
	}
}

func TestCleanScope(t *testing.T) {
	tests := []struct {
		name       string
		cacheFlag  bool
		pacmanFlag bool
		allFlag    bool
		wantAUR    bool
		wantPacman bool
	}{
		{
			name:       "default (no flags) cleans AUR cache only",
			cacheFlag:  false,
			pacmanFlag: false,
			allFlag:    false,
			wantAUR:    true,
			wantPacman: false,
		},
		{
			name:       "--pacman cleans pacman cache only",
			cacheFlag:  false,
			pacmanFlag: true,
			allFlag:    false,
			wantAUR:    false,
			wantPacman: true,
		},
		{
			name:       "--cache cleans AUR cache only",
			cacheFlag:  true,
			pacmanFlag: false,
			allFlag:    false,
			wantAUR:    true,
			wantPacman: false,
		},
		{
			name:       "--all cleans both caches",
			cacheFlag:  false,
			pacmanFlag: false,
			allFlag:    true,
			wantAUR:    true,
			wantPacman: true,
		},
		{
			name:       "--cache and --pacman together cleans both",
			cacheFlag:  true,
			pacmanFlag: true,
			allFlag:    false,
			wantAUR:    true,
			wantPacman: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAUR, gotPacman := cleanScope(tc.cacheFlag, tc.pacmanFlag, tc.allFlag)
			if gotAUR != tc.wantAUR || gotPacman != tc.wantPacman {
				t.Errorf("cleanScope(%v, %v, %v) = (%v, %v), want (%v, %v)",
					tc.cacheFlag, tc.pacmanFlag, tc.allFlag, gotAUR, gotPacman, tc.wantAUR, tc.wantPacman)
			}
		})
	}
}
