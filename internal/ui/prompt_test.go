package ui

import (
	"bufio"
	"strings"
	"testing"
)

func TestIsValidPkgName(t *testing.T) {
	valid := []string{
		"neovim",
		"git",
		"gtk+3",
		"libsigc++",
		"gcc@11",
		"clang@14",
		"python-pip",
		"lib32-glibc",
		"a.b-c_d+e@f",
	}

	for _, name := range valid {
		if !IsValidPkgName(name) {
			t.Errorf("expected %q to be a valid package name, but got false", name)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"-Syu",
		"-foo",
		".hidden",
		"foo..bar",
		"../escaped",
		"foo bar",
		"foo;rm",
		"foo/bar",
		"foo*bar",
		"foo$bar",
		strings.Repeat("a", 256),
	}

	for _, name := range invalid {
		if IsValidPkgName(name) {
			t.Errorf("expected %q to be an invalid package name, but got true", name)
		}
	}
}

func TestPromptSelection(t *testing.T) {
	items := []SelectionItem{
		{Index: 1, Name: "neovim", FullName: "extra/neovim", SourceLabel: "official"},
		{Index: 2, Name: "neovim-git", SourceLabel: "AUR"},
		{Index: 3, Name: "gtk+3", SourceLabel: "official"},
	}

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "by number", input: "2\n", want: "neovim-git"},
		{name: "by name", input: "neovim\n", want: "neovim"},
		{name: "by full name", input: "extra/neovim\n", want: "neovim"},
		{name: "case insensitive", input: "NEOVIM-GIT\n", want: "neovim-git"},
		{name: "empty skips", input: "\n", want: ""},
		{name: "invalid skips", input: "nonexistent\n", want: ""},
		{name: "out of range skips", input: "99\n", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PromptSelection(bufio.NewReader(strings.NewReader(tc.input)), "prompt", items)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil || got.Name != tc.want {
				t.Fatalf("expected %q, got %+v", tc.want, got)
			}
		})
	}

	if got := PromptSelection(bufio.NewReader(strings.NewReader("1\n")), "prompt", nil); got != nil {
		t.Errorf("expected nil for an empty item list, got %+v", got)
	}
}

func TestConfirmAction(t *testing.T) {
	// Auto-confirm
	r := bufio.NewReader(strings.NewReader(""))
	if !ConfirmAction(r, "Proceed?", true) {
		t.Errorf("expected auto-confirmation when autoConfirm is true")
	}

	// Input = "y"
	r = bufio.NewReader(strings.NewReader("y\n"))
	if !ConfirmAction(r, "Proceed?", false) {
		t.Errorf("expected confirmation on 'y'")
	}

	// Input = "yes"
	r = bufio.NewReader(strings.NewReader("yes\n"))
	if !ConfirmAction(r, "Proceed?", false) {
		t.Errorf("expected confirmation on 'yes'")
	}

	// Input = "n"
	r = bufio.NewReader(strings.NewReader("n\n"))
	if ConfirmAction(r, "Proceed?", false) {
		t.Errorf("expected rejection on 'n'")
	}

	// Input = empty
	r = bufio.NewReader(strings.NewReader("\n"))
	if ConfirmAction(r, "Proceed?", false) {
		t.Errorf("expected rejection on empty input")
	}
}
