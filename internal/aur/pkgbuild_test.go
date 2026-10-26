package aur

import "testing"

// Fixtures are ordered to match real AUR packages. The first mirrors paru,
// whose makedepends line precedes depends: that ordering is what the old
// unanchored-regex parser got wrong.

const fixtureMakeDependsFirst = `# Maintainer: Jguer <pkgbuilds at jguer.space>
pkgname=paru
pkgver=13.0.1
pkgrel=1
pkgdesc="AUR helper"
url="https://github.com/Jguer/paru"
makedepends=('cargo')
depends=('git' 'pacman' 'libalpm.so>=14')
`

func TestParsePKGBUILD_MakeDependsDoesNotLeakIntoDepends(t *testing.T) {
	info := ParsePKGBUILD(fixtureMakeDependsFirst)

	assertStrings(t, "Depends", info.Depends, []string{"git", "pacman", "libalpm.so>=14"})
	assertStrings(t, "MakeDepends", info.MakeDepends, []string{"cargo"})
}

func TestParsePKGBUILD_AbsentDependsStaysEmpty(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=bar
pkgver=1.0
pkgrel=1
makedepends=(cmake ninja)
`)

	assertStrings(t, "Depends", info.Depends, nil)
	assertStrings(t, "MakeDepends", info.MakeDepends, []string{"cmake", "ninja"})
}

func TestParsePKGBUILD_IgnoresCommentedAssignments(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=baz
pkgver=1.0
pkgrel=1
#depends=(commented-out-dep)
depends=(real-dep)
`)

	assertStrings(t, "Depends", info.Depends, []string{"real-dep"})
}

func TestParsePKGBUILD_SplitPackageName(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=(
  "electron"
  "electron-extra"
)
pkgver=30.0.0
pkgrel=1
`)

	if info.PackageName != "electron" {
		t.Errorf("PackageName = %q, want %q", info.PackageName, "electron")
	}
	assertStrings(t, "PackageNames", info.PackageNames, []string{"electron", "electron-extra"})
}

func TestParsePKGBUILD_ParenthesisInsideQuotedSource(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=qux
pkgver=1.0
pkgrel=1
source=("foo(1).tar.gz::https://example.com/foo(1).tar.gz")
depends=(glibc)
`)

	assertStrings(t, "Sources",
		info.Sources,
		[]string{"foo(1).tar.gz::https://example.com/foo(1).tar.gz"})
	assertStrings(t, "Depends", info.Depends, []string{"glibc"})
}

func TestParsePKGBUILD_AppendOperator(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=quux
pkgver=1.0
pkgrel=1
depends=('glibc')
depends+=('openssl')
optdepends=('bash: for completion')
optdepends+=('zsh: for completion')
`)

	assertStrings(t, "Depends", info.Depends, []string{"glibc", "openssl"})
	assertStrings(t, "OptDepends", info.OptDepends,
		[]string{"bash: for completion", "zsh: for completion"})
}

func TestParsePKGBUILD_MultilineArrayIgnoresTrailingComments(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=multy
pkgver=1.0
depends=(
  'glibc'
  'openssl>=1.1'   # crypto
  "libfoo"
)
checkdepends=('bats')
`)

	assertStrings(t, "Depends", info.Depends, []string{"glibc", "openssl>=1.1", "libfoo"})
	assertStrings(t, "CheckDepends", info.CheckDepends, []string{"bats"})
}

func TestParsePKGBUILD_PrivateVariableDoesNotShadow(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=priv
pkgver=1.0
_my_depends=(not-a-real-dep)
depends=(glibc)
`)

	assertStrings(t, "Depends", info.Depends, []string{"glibc"})
}

func TestParsePKGBUILD_ScalarFields(t *testing.T) {
	info := ParsePKGBUILD(`# Maintainer: Jane Doe <jane@example.com>
pkgname=sample-pkg
pkgver=1.2.3
pkgrel=1
pkgdesc="A sample package for testing"
url="https://example.com/sample"
`)

	cases := map[string]struct{ got, want string }{
		"PackageName": {info.PackageName, "sample-pkg"},
		"Version":     {info.Version, "1.2.3"},
		"Release":     {info.Release, "1"},
		"Description": {info.Description, "A sample package for testing"},
		"URL":         {info.URL, "https://example.com/sample"},
		"Maintainer":  {info.Maintainer, "Jane Doe <jane@example.com>"},
	}
	for field, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", field, c.got, c.want)
		}
	}
	if info.RawContent == "" {
		t.Error("expected RawContent to be preserved")
	}
}

func TestParsePKGBUILD_IgnoresFunctionBodies(t *testing.T) {
	info := ParsePKGBUILD(`pkgname=fn
pkgver=1.0
depends=(glibc)

build() {
  depends=(not-a-dependency)
  make DESTDIR="$pkgdir" install
}
`)

	assertStrings(t, "Depends", info.Depends, []string{"glibc"})
}

func TestStripVersionConstraint(t *testing.T) {
	cases := map[string]string{
		"glibc":            "glibc",
		"libalpm.so>=14":   "libalpm.so",
		"openssl<3":        "openssl",
		"pacman=6.1.0":     "pacman",
		"java-runtime":     "java-runtime",
		"":                 "",
		">=only-versioned": ">=only-versioned",
	}
	for in, want := range cases {
		if got := StripVersionConstraint(in); got != want {
			t.Errorf("StripVersionConstraint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetchPKGBUILD_InvalidName(t *testing.T) {
	if _, err := FetchPKGBUILD("../invalid"); err == nil {
		t.Fatal("expected error for invalid package name, got nil")
	}
}

func assertStrings(t *testing.T, field string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", field, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", field, got, want)
		}
	}
}
