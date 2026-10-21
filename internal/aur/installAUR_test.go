package aur

import "testing"

func TestIsValidPackageName(t *testing.T) {
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
		if !isValidPackageName(name) {
			t.Errorf("expected %q to be valid AUR package name, got false", name)
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
	}

	for _, name := range invalid {
		if isValidPackageName(name) {
			t.Errorf("expected %q to be invalid AUR package name, got true", name)
		}
	}
}

func TestInstallAUR_InvalidName(t *testing.T) {
	err := InstallAUR("invalid package name with spaces")
	if err == nil {
		t.Fatalf("expected error for invalid package name, got nil")
	}
}
