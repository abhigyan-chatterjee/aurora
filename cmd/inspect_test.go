package cmd

import (
	"strings"
	"testing"
)

func TestInspect_Help(t *testing.T) {
	out := captureStdout(func() {
		_ = inspectCmd.Help()
	})
	if !strings.Contains(out, "PKGBUILD") {
		t.Errorf("expected PKGBUILD mention in inspect help, got: %s", out)
	}
}

func TestInspect_OfficialPackage(t *testing.T) {
	out := captureStdout(func() {
		inspect(inspectCmd, []string{"bash"})
	})
	if !strings.Contains(out, "Official Repository Inspection: bash") && !strings.Contains(out, "Official Arch Linux binary repository") {
		t.Errorf("expected official inspection output for bash, got: %s", out)
	}
}

func TestInspect_InvalidPackage(t *testing.T) {
	var err error
	out := captureStdout(func() {
		err = inspect(inspectCmd, []string{"../invalid"})
	})
	if err == nil {
		t.Errorf("expected non-nil error for invalid package name")
	}
	if !strings.Contains(out, "invalid package name format") {
		t.Errorf("expected invalid format warning, got: %s", out)
	}
}

func TestInspect_NoArgs(t *testing.T) {
	var err error
	out := captureStdout(func() {
		err = inspect(inspectCmd, []string{})
	})
	if err != nil {
		t.Errorf("expected nil error for no args, got: %v", err)
	}
	if !strings.Contains(out, "Please provide a package name to inspect.") {
		t.Errorf("expected prompt to provide package name, got: %s", out)
	}
}
