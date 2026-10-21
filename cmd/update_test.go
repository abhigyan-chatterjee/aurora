package cmd

import (
	"strings"
	"testing"
)

func TestUpdate_Help(t *testing.T) {
	out := captureStdout(func() {
		_ = updateCmd.Help()
	})

	if !strings.Contains(out, "The system upgrade runs FIRST, then outdated AUR packages are rebuilt") {
		t.Errorf("expected update help to explain execution order, got: %s", out)
	}

	if !strings.Contains(out, "-Syu") {
		t.Errorf("expected -Syu in update help, got: %s", out)
	}

	if !strings.Contains(out, "--noconfirm") {
		t.Errorf("expected --noconfirm in update help, got: %s", out)
	}
}
