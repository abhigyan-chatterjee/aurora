package aur

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setCgitBase points FetchPKGBUILD at a local server for the duration of a test.
func setCgitBase(t *testing.T, base string) {
	t.Helper()
	original := aurCgitBase
	aurCgitBase = base
	t.Cleanup(func() { aurCgitBase = original })
}

// A missing package and an unreachable AUR need different handling: the first
// falls through to the official lookup, the second must be reported.
func TestFetchPKGBUILD_NotFoundIsSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setCgitBase(t, srv.URL)

	_, err := FetchPKGBUILD("ghost")
	if err == nil {
		t.Fatal("expected an error for a 404")
	}
	if !errors.Is(err, ErrPackageNotFound) {
		t.Errorf("expected ErrPackageNotFound, got: %v", err)
	}
}

func TestFetchPKGBUILD_ServerErrorIsNotSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	setCgitBase(t, srv.URL)

	_, err := FetchPKGBUILD("flaky")
	if err == nil {
		t.Fatal("expected an error for a 500")
	}
	if errors.Is(err, ErrPackageNotFound) {
		t.Errorf("a server error must not be reported as 'package not found': %v", err)
	}
}

func TestFetchPKGBUILD_EmptyBodyIsSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setCgitBase(t, srv.URL)

	if _, err := FetchPKGBUILD("empty"); !errors.Is(err, ErrPackageNotFound) {
		t.Errorf("expected an empty PKGBUILD to be reported as missing, got: %v", err)
	}
}

func TestFetchPKGBUILD_ReturnsContent(t *testing.T) {
	const body = "pkgname=demo\npkgver=1.0\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	setCgitBase(t, srv.URL)

	got, err := FetchPKGBUILD("demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("content = %q, want %q", got, body)
	}
}
