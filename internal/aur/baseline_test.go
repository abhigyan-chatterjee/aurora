package aur

import (
	"os"
	"testing"
)

// The diff feature is only useful if a baseline actually survives between runs.
func TestBaselineRoundTrip(t *testing.T) {
	original := CacheDirOverride
	CacheDirOverride = t.TempDir()
	t.Cleanup(func() { CacheDirOverride = original })

	if _, ok := ReadBaseline("demo"); ok {
		t.Fatal("expected no baseline before the first build")
	}

	saveBaseline("demo", "pkgname=demo\npkgver=1.0\n")

	got, ok := ReadBaseline("demo")
	if !ok {
		t.Fatal("expected the baseline to be readable after saving")
	}
	if got != "pkgname=demo\npkgver=1.0\n" {
		t.Errorf("baseline = %q", got)
	}
}

func TestBaselineIsPerPackage(t *testing.T) {
	original := CacheDirOverride
	CacheDirOverride = t.TempDir()
	t.Cleanup(func() { CacheDirOverride = original })

	saveBaseline("one", "pkgver=1\n")
	saveBaseline("two", "pkgver=2\n")

	one, ok := ReadBaseline("one")
	if !ok || one != "pkgver=1\n" {
		t.Errorf("baseline for one = %q", one)
	}
	two, ok := ReadBaseline("two")
	if !ok || two != "pkgver=2\n" {
		t.Errorf("baseline for two = %q", two)
	}
}

func TestBaselineUnreadableWhenCacheDirInvalid(t *testing.T) {
	original := CacheDirOverride
	// A path that cannot be created because a file already occupies it.
	dir := t.TempDir()
	blocker := dir + "/blocked"
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	CacheDirOverride = blocker
	t.Cleanup(func() { CacheDirOverride = original })

	if _, ok := ReadBaseline("demo"); ok {
		t.Error("expected no baseline when the cache directory is unusable")
	}
}
