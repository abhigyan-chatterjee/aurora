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
	captureStdout(func() {
		err = inspect(inspectCmd, []string{"../invalid"})
	})
	if err == nil {
		t.Fatal("expected a non-nil error for an invalid package name")
	}
	if !strings.Contains(err.Error(), "invalid package name") {
		t.Errorf("expected the error to name the problem, got: %v", err)
	}
}

// A missing argument is a usage error: the process must exit non-zero so a
// script cannot mistake "nothing was inspected" for success.
func TestInspect_NoArgsIsAnError(t *testing.T) {
	var err error
	captureStdout(func() {
		err = inspect(inspectCmd, []string{})
	})
	if err == nil {
		t.Fatal("expected an error when no package name is given")
	}
	if !strings.Contains(err.Error(), "package name") {
		t.Errorf("expected the error to mention the package name argument, got: %v", err)
	}
}
