package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestExplain_HelpFlags(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		conciseFlag = false
		yesFlag = false

		output := captureStdout(func() {
			explain(explainCmd, []string{flag})
		})

		if !strings.Contains(output, "Teaching mode command") {
			t.Errorf("expected help output for %s, got: %s", flag, output)
		}
		if strings.Contains(output, "No exact explanation topic found") {
			t.Errorf("help flag %s was wrongly treated as a topic query", flag)
		}
		if strings.Contains(output, "=== Aurora Teaching Mode — Concepts & Walkthroughs ===") {
			t.Errorf("help flag %s wrongly entered interactive picker", flag)
		}
	}
}

func TestExplain_ConciseFlag(t *testing.T) {
	// Test --concise before topic
	conciseFlag = false
	out1 := captureStdout(func() {
		explain(explainCmd, []string{"--concise", "-Syu"})
	})
	if !strings.Contains(out1, "Refreshes database indexes") {
		t.Errorf("expected summary in concise output, got: %s", out1)
	}
	if strings.Contains(out1, "rolling-release") {
		t.Errorf("did not expect full teaching description in concise mode, got: %s", out1)
	}

	// Test -c shorthand after topic
	conciseFlag = false
	out2 := captureStdout(func() {
		explain(explainCmd, []string{"-Syu", "-c"})
	})
	if !strings.Contains(out2, "Refreshes database indexes") {
		t.Errorf("expected summary in concise output with -c, got: %s", out2)
	}
	if strings.Contains(out2, "rolling-release") {
		t.Errorf("did not expect full teaching description with -c, got: %s", out2)
	}
}

func TestExplain_FullDescription(t *testing.T) {
	conciseFlag = false
	out := captureStdout(func() {
		explain(explainCmd, []string{"-Syu"})
	})
	if !strings.Contains(out, "Refreshes database indexes") && !strings.Contains(out, "rolling-release") {
		t.Errorf("expected full description in standard mode, got: %s", out)
	}
	if !strings.Contains(out, "Arch Linux is a rolling-release") {
		t.Errorf("expected full teaching explanation in standard mode, got: %s", out)
	}
}
