package ui

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestParseBinChoiceMatrix(t *testing.T) {
	cases := []struct {
		input string
		want  BinPromptChoice
		ok    bool
		skip  bool
	}{
		{"", BinChoiceBinary, true, false},
		{"1", BinChoiceBinary, true, false},
		{"b", BinChoiceBinary, true, false},
		{"bin", BinChoiceBinary, true, false},
		{"binary", BinChoiceBinary, true, false},
		{"y", BinChoiceBinary, true, false},
		{"yes", BinChoiceBinary, true, false},
		{"foo-bin", BinChoiceBinary, true, false},
		{"2", BinChoiceSource, true, false},
		{"s", BinChoiceSource, true, false},
		{"src", BinChoiceSource, true, false},
		{"source", BinChoiceSource, true, false},
		{"n", BinChoiceSource, true, false},
		{"no", BinChoiceSource, true, false},
		{"foo", BinChoiceSource, true, false},
		{"q", BinChoiceSkip, true, true},
		{"quit", BinChoiceSkip, true, true},
		{"abort", BinChoiceSkip, true, true},
		{"cancel", BinChoiceSkip, true, true},
		{"xyz", BinChoiceBinary, false, false},
		{"3", BinChoiceBinary, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			choice, ok, skip := parseBinChoice(tc.input, "foo", "foo-bin")
			if choice != tc.want || ok != tc.ok || skip != tc.skip {
				t.Errorf("parseBinChoice(%q) = (%v,%v,%v), want (%v,%v,%v)",
					tc.input, choice, ok, skip, tc.want, tc.ok, tc.skip)
			}
		})
	}

}

func TestPromptBinSelection_CaseInsensitive(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("YES\n"))
	got := PromptBinSelection(r, "foo", "foo-bin", false, false)
	if got != BinChoiceBinary {
		t.Errorf("YES: got %v, want BinChoiceBinary", got)
	}
	r2 := bufio.NewReader(strings.NewReader("Source\n"))
	got2 := PromptBinSelection(r2, "foo", "foo-bin", false, false)
	if got2 != BinChoiceSource {
		t.Errorf("Source: got %v, want BinChoiceSource", got2)
	}
}

func TestPromptBinSelection_AutoConfirm(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(""))
	got := PromptBinSelection(r, "foo", "foo-bin", false, true)
	if got != BinChoiceBinary {
		t.Errorf("autoConfirm: got %v, want BinChoiceBinary", got)
	}
}

func TestPromptBinSelection_DefaultEnter(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got := PromptBinSelection(r, "foo", "foo-bin", false, false)
	if got != BinChoiceBinary {
		t.Errorf("default enter: got %v, want BinChoiceBinary", got)
	}
}

func TestPromptBinSelection_ChoiceSource(t *testing.T) {
	for _, in := range []string{"2\n", "s\n", "src\n", "source\n", "n\n", "no\n", "foo\n"} {
		r := bufio.NewReader(strings.NewReader(in))
		got := PromptBinSelection(r, "foo", "foo-bin", false, false)
		if got != BinChoiceSource {
			t.Errorf("input %q: got %v, want BinChoiceSource", in, got)
		}
	}
}

func TestPromptBinSelection_ChoiceBinary(t *testing.T) {
	for _, in := range []string{"1\n", "b\n", "bin\n", "binary\n", "y\n", "yes\n", "foo-bin\n"} {
		r := bufio.NewReader(strings.NewReader(in))
		got := PromptBinSelection(r, "foo", "foo-bin", false, false)
		if got != BinChoiceBinary {
			t.Errorf("input %q: got %v, want BinChoiceBinary", in, got)
		}
	}
}

func TestPromptBinSelection_Skip(t *testing.T) {
	for _, in := range []string{"q\n", "quit\n", "abort\n", "cancel\n"} {
		r := bufio.NewReader(strings.NewReader(in))
		got := PromptBinSelection(r, "foo", "foo-bin", false, false)
		if got != BinChoiceSkip {
			t.Errorf("input %q: got %v, want BinChoiceSkip", in, got)
		}
	}
}

func TestPromptBinSelection_InvalidThenValid(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("xyz\n1\n"))
	got := PromptBinSelection(r, "foo", "foo-bin", false, false)
	if got != BinChoiceBinary {
		t.Errorf("invalid-then-valid: got %v, want BinChoiceBinary", got)
	}
}

func TestPromptBinSelection_ThreeInvalidDefaults(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("xyz\nabc\ndef\n"))
	got := PromptBinSelection(r, "foo", "foo-bin", false, false)
	if got != BinChoiceBinary {
		t.Errorf("three-invalid: got %v, want BinChoiceBinary", got)
	}
}

func TestPromptBinSelection_Concise(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	out, _ := captureOutput(t, func() {
		got := PromptBinSelection(r, "foo", "foo-bin", true, false)
		if got != BinChoiceBinary {
			t.Errorf("concise: got %v, want BinChoiceBinary", got)
		}
	})
	if !strings.Contains(out, "AUR binary package available for 'foo':") {
		t.Errorf("concise stdout missing header: %q", out)
	}
	if !strings.Contains(out, "[1] foo-bin (binary, recommended)") {
		t.Errorf("concise stdout missing option: %q", out)
	}
}

func TestPromptBinSelection_Quiet(t *testing.T) {
	prev := Quiet
	Quiet = true
	defer func() { Quiet = prev }()
	out, _ := captureOutput(t, func() {
		r := bufio.NewReader(strings.NewReader("y\n"))
		got := PromptBinSelection(r, "foo", "foo-bin", false, false)
		if got != BinChoiceBinary {
			t.Errorf("quiet: got %v, want BinChoiceBinary", got)
		}
		r2 := bufio.NewReader(strings.NewReader("n\n"))
		got2 := PromptBinSelection(r2, "foo", "foo-bin", false, false)
		if got2 != BinChoiceSource {
			t.Errorf("quiet n: got %v, want BinChoiceSource", got2)
		}
	})
	if !strings.Contains(out, "Install 'foo-bin' instead of source 'foo'? [Y/n]: ") {
		t.Errorf("quiet stdout missing prompt: %q", out)
	}
}

func TestPromptBinSelection_EOF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(""))
	out, errOut := captureOutput(t, func() {
		got := PromptBinSelection(r, "foo", "foo-bin", false, false)
		if got != BinChoiceBinary {
			t.Errorf("EOF: got %v, want BinChoiceBinary", got)
		}
	})
	if !strings.Contains(out, "Enter choice [1/2] (default: 1): ") {
		t.Errorf("EOF stdout missing prompt: %q", out)
	}
	if !strings.Contains(errOut, "non-interactive session detected; defaulting to 'foo-bin'") {
		t.Errorf("EOF stderr missing warning: %q", errOut)
	}
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	fn()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	var bufOut, bufErr bytes.Buffer
	io.Copy(&bufOut, rOut)
	io.Copy(&bufErr, rErr)
	rOut.Close()
	rErr.Close()
	return bufOut.String(), bufErr.String()
}
