package diff

import (
	"strings"
	"testing"
)

func TestUnified_IdenticalIsEmpty(t *testing.T) {
	if got := UnifiedText("a", "b", "same\nlines\n", "same\nlines\n"); got != "" {
		t.Errorf("expected empty diff for identical text, got:\n%s", got)
	}
}

func TestUnified_DetectsChange(t *testing.T) {
	a := "pkgname=foo\npkgver=1.0\npkgrel=1\ndepends=(glibc)\n"
	b := "pkgname=foo\npkgver=1.1\npkgrel=1\ndepends=(glibc)\n"

	got := UnifiedText("old", "new", a, b)
	if got == "" {
		t.Fatal("expected a diff, got empty string")
	}
	for _, want := range []string{"--- old", "+++ new", "-pkgver=1.0", "+pkgver=1.1", " pkgrel=1"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected diff to contain %q, got:\n%s", want, got)
		}
	}
}

func TestUnified_DetectsAddition(t *testing.T) {
	a := "depends=(glibc)\n"
	b := "depends=(glibc)\ndepends+=(openssl)\n"

	got := UnifiedText("old", "new", a, b)
	if !strings.Contains(got, "+depends+=(openssl)") {
		t.Errorf("expected added line in diff, got:\n%s", got)
	}
	if strings.Contains(got, "-depends=(glibc)") {
		t.Errorf("unchanged line must not be reported as removed, got:\n%s", got)
	}
}

func TestUnified_EmptyToContent(t *testing.T) {
	got := UnifiedText("old", "", "", "brand new\n")
	if got == "" {
		t.Fatal("expected a diff when a document is created")
	}
	if !strings.Contains(got, "+brand new") {
		t.Errorf("expected inserted line, got:\n%s", got)
	}
}

func TestHunkHeaderShape(t *testing.T) {
	lines := make([]string, 20)
	b := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
		b[i] = "line"
	}
	lines[10] = "old"
	b[10] = "new"

	got := UnifiedText("a", "b", strings.Join(lines, "\n"), strings.Join(b, "\n"))
	if !strings.Contains(got, "@@ -") {
		t.Fatalf("expected hunk header, got:\n%s", got)
	}
	// The change is at line 11 with 3 lines of context either side.
	if !strings.Contains(got, "@@ -8,7 +8,7 @@") {
		t.Errorf("expected hunk header @@ -8,7 +8,7 @@, got:\n%s", got)
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"one\n", 1},
		{"one\ntwo\n", 2},
		{"one\r\ntwo\r\n", 2},
	}
	for _, tc := range cases {
		if got := len(SplitLines(tc.in)); got != tc.want {
			t.Errorf("SplitLines(%q) returned %d lines, want %d", tc.in, got, tc.want)
		}
	}
}
