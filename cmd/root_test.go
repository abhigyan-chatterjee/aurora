package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestRootCmd_Flags(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup("yes") == nil {
		t.Errorf("expected --yes persistent flag on rootCmd")
	}

	if rootCmd.PersistentFlags().Lookup("concise") == nil {
		t.Errorf("expected --concise persistent flag on rootCmd")
	}

	if !rootCmd.SilenceUsage {
		t.Errorf("expected SilenceUsage to be true on rootCmd")
	}

	if !rootCmd.SilenceErrors {
		t.Errorf("expected SilenceErrors to be true on rootCmd")
	}

	if rootCmd.Version == "" || rootCmd.Version != Version {
		t.Errorf("expected rootCmd.Version to match cmd.Version (%q), got %q", Version, rootCmd.Version)
	}
}

func TestRootCmd_PreRunFailsWithoutPacman(t *testing.T) {
	// Keep PreRunE from reading whatever config file the developer happens to
	// have: a malformed one would fail this test for the wrong reason.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	origPath := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", origPath)
	})
	_ = os.Setenv("PATH", "")

	err := rootCmd.PersistentPreRunE(rootCmd, nil)
	if err == nil {
		t.Fatalf("expected PersistentPreRunE to fail when pacman is missing from PATH")
	}
	if !strings.Contains(err.Error(), "pacman not found") {
		t.Errorf("expected error to mention pacman not found, got: %v", err)
	}
}
