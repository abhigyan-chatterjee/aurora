package cmd

import (
	"strings"
	"testing"

	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
)

func TestAutoremove_Help(t *testing.T) {
	out := captureStdout(func() {
		_ = autoremoveCmd.Help()
	})
	if !strings.Contains(out, "orphan") {
		t.Errorf("expected orphan in autoremove help, got: %s", out)
	}
}

func TestOrphansReturnsNonEmptyNames(t *testing.T) {
	orphans, err := pacman.Orphans()
	if err != nil {
		t.Fatalf("unexpected error running pacman.Orphans: %v", err)
	}
	// Verify that none of the returned orphan names are empty strings
	for _, o := range orphans {
		if strings.TrimSpace(o) == "" {
			t.Errorf("expected non-empty orphan name, got %q", o)
		}
	}
}
