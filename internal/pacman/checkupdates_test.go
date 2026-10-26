package pacman

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeCheckupdates puts a stub `checkupdates` on PATH that exits with the given
// status and prints the given body.
func fakeCheckupdates(t *testing.T, exitCode int, body string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "checkupdates")
	content := "#!/bin/sh\n"
	if body != "" {
		content += "cat <<'EOF'\n" + body + "\nEOF\n"
	}
	content += "exit " + itoa(exitCode) + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("could not write the stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// checkupdates exits 2 when there is nothing to upgrade. That is a result, not a
// failure: treating it as an error makes a fully updated system look broken.
func TestCheckUpdates_ExitTwoMeansNothingToUpgrade(t *testing.T) {
	fakeCheckupdates(t, 2, "")

	updates, available, err := CheckUpdates()
	if err != nil {
		t.Fatalf("exit 2 must not be an error, got: %v", err)
	}
	if !available {
		t.Error("expected checkupdates to be reported as available")
	}
	if len(updates) != 0 {
		t.Errorf("expected no updates, got %v", updates)
	}
}

func TestCheckUpdates_ParsesUpgrades(t *testing.T) {
	fakeCheckupdates(t, 0, "bash 5.3.0-1 -> 5.3.1-1\ngit 2.50.0-1 -> 2.51.0-1")

	updates, available, err := CheckUpdates()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !available {
		t.Fatal("expected checkupdates to be available")
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %v", updates)
	}
	if got := updates["bash"]; got[0] != "5.3.0-1" || got[1] != "5.3.1-1" {
		t.Errorf("bash = %v, want [5.3.0-1 5.3.1-1]", got)
	}
}

// Exit 1 is checkupdates reporting "cannot fetch updates", which is a real
// failure and must stay an error.
func TestCheckUpdates_ExitOneIsAnError(t *testing.T) {
	fakeCheckupdates(t, 1, "")

	if _, _, err := CheckUpdates(); err == nil {
		t.Fatal("exit 1 must remain an error")
	}
}
