package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The release ships one binary per architecture. Selecting by name alone handed
// an amd64 binary to aarch64 machines, which replaced a working executable with
// one that cannot run.
func TestFindAuroraAsset_SelectsMatchingArchitecture(t *testing.T) {
	assets := []githubAsset{
		{Name: "aurora-linux-amd64", BrowserDownloadURL: "http://example/amd64"},
		{Name: "aurora-linux-arm64", BrowserDownloadURL: "http://example/arm64"},
		{Name: "aurora-linux-amd64.sha256"},
		{Name: "aurora-linux-arm64.sha256"},
	}

	got := findAuroraAsset(assets, "arm64")
	if got == nil || got.Name != "aurora-linux-arm64" {
		t.Fatalf("expected the arm64 asset, got %+v", got)
	}

	got = findAuroraAsset(assets, "amd64")
	if got == nil || got.Name != "aurora-linux-amd64" {
		t.Fatalf("expected the amd64 asset, got %+v", got)
	}
}

func TestFindAuroraAsset_NoAssetForArchitecture(t *testing.T) {
	assets := []githubAsset{{Name: "aurora-linux-amd64"}}

	if got := findAuroraAsset(assets, "riscv64"); got != nil {
		t.Fatalf("expected nil for an unsupported architecture, got %+v", got)
	}
}

// Older releases published a single asset called "aurora" built for amd64.
// That is still accepted on amd64 and never anywhere else.
func TestFindAuroraAsset_BareNameOnlyOnAmd64(t *testing.T) {
	assets := []githubAsset{{Name: "aurora"}}

	if got := findAuroraAsset(assets, "amd64"); got == nil {
		t.Fatal("expected the bare aurora asset to be accepted on amd64")
	}
	if got := findAuroraAsset(assets, "arm64"); got != nil {
		t.Fatal("a bare aurora asset must not be offered to a non-amd64 machine")
	}
}

func TestResolveDigest_PrefersAssetDigest(t *testing.T) {
	client := &http.Client{}
	asset := &githubAsset{Name: "aurora-linux-amd64", Digest: "sha256:abc"}

	got, err := resolveDigest(client, nil, asset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "sha256:abc" {
		t.Errorf("digest = %q, want sha256:abc", got)
	}
}

// When GitHub leaves the digest field empty, the .sha256 asset the release
// workflow publishes must be used instead of skipping verification.
func TestResolveDigest_FallsBackToChecksumAsset(t *testing.T) {
	const hexSum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  aurora-linux-amd64\n", hexSum)
	}))
	defer srv.Close()

	asset := &githubAsset{Name: "aurora-linux-amd64", Digest: ""}
	assets := []githubAsset{
		*asset,
		{Name: "aurora-linux-amd64.sha256", BrowserDownloadURL: srv.URL},
	}

	got, err := resolveDigest(srv.Client(), assets, asset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "sha256:"+hexSum {
		t.Errorf("digest = %q, want sha256:%s", got, hexSum)
	}
}

func TestResolveDigest_ErrorsWhenNothingToVerifyAgainst(t *testing.T) {
	asset := &githubAsset{Name: "aurora-linux-amd64", Digest: ""}

	if _, err := resolveDigest(&http.Client{}, []githubAsset{*asset}, asset); err == nil {
		t.Fatal("expected an error when neither a digest nor a checksum file exists")
	}
}

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "seconds"},
		{2 * time.Minute, "minutes"},
		{2 * time.Hour, "hours"},
		{48 * time.Hour, "days"},
	}
	for _, tc := range cases {
		if got := humanDuration(tc.d); !strings.Contains(got, tc.want) {
			t.Errorf("humanDuration(%s) = %q, want it to mention %q", tc.d, got, tc.want)
		}
	}
}
