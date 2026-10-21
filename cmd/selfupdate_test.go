package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfUpdate_Help(t *testing.T) {
	out := captureStdout(func() {
		_ = selfUpdateCmd.Help()
	})
	if !strings.Contains(out, "selfupdate") {
		t.Errorf("expected selfupdate in help output, got: %s", out)
	}
	if !strings.Contains(out, "--force") {
		t.Errorf("expected --force flag in help output, got: %s", out)
	}
}

func TestNormalizeVersionTag(t *testing.T) {
	tests := []struct {
		raw  string
		name string
		want string
	}{
		{"v0.3.0", "Aurora v0.3.0", "0.3.0"},
		{"0.4.1", "", "0.4.1"},
		{"major", "v0.2", "0.2"},
		{"release-v1.2.3", "", "1.2.3"},
		{"major", "major", ""},
	}

	for _, tc := range tests {
		got := extractReleaseVersion(tc.raw, tc.name)
		if got != tc.want {
			t.Errorf("extractReleaseVersion(%q, %q) = %q, want %q", tc.raw, tc.name, got, tc.want)
		}
	}
}

func TestVerifySHA256Digest(t *testing.T) {
	payload := []byte("aurora-binary-test-payload")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	tmp := filepath.Join(t.TempDir(), "aurora")
	if err := os.WriteFile(tmp, payload, 0755); err != nil {
		t.Fatalf("failed writing temp file: %v", err)
	}

	if err := verifyFileDigest(tmp, "sha256:"+hexSum); err != nil {
		t.Errorf("expected valid digest to pass, got: %v", err)
	}
	if err := verifyFileDigest(tmp, ""); err != nil {
		t.Errorf("expected empty digest to be skipped without error, got: %v", err)
	}
	if err := verifyFileDigest(tmp, "sha256:0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Errorf("expected mismatched digest to return error")
	}
}

func TestSelfUpdate_UpToDateSkipsUnlessForced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"tag_name": "v%s",
			"name": "v%s",
			"assets": [{"name": "aurora", "browser_download_url": "http://%s/aurora", "digest": ""}]
		}`, rootCmd.Version, rootCmd.Version, r.Host)
	}))
	defer srv.Close()

	origAPI := githubLatestReleaseURL
	githubLatestReleaseURL = srv.URL
	t.Cleanup(func() { githubLatestReleaseURL = origAPI })

	selfUpdateForceFlag = false
	selfUpdateCheckFlag = false

	var err error
	out := captureStdout(func() {
		err = selfUpdate(selfUpdateCmd, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error when already up to date: %v", err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("expected 'already up to date' message, got: %s", out)
	}
}

func TestSelfUpdate_DownloadsAndReplacesBinary(t *testing.T) {
	newBinContent := []byte("#!/bin/sh\necho aurora-0.9.9\n")
	sum := sha256.Sum256(newBinContent)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download/aurora" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(newBinContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"tag_name": "v99.0.0",
			"name": "v99.0.0",
			"assets": [{"name": "aurora", "browser_download_url": "%s/download/aurora", "digest": %q}]
		}`, srv.URL, digest)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	fakeExe := filepath.Join(tmpDir, "aurora")
	if err := os.WriteFile(fakeExe, []byte("old-binary"), 0755); err != nil {
		t.Fatalf("failed creating fake executable: %v", err)
	}

	origAPI := githubLatestReleaseURL
	origExeFn := executablePathFn
	origYes := yesFlag
	githubLatestReleaseURL = srv.URL
	executablePathFn = func() (string, error) { return fakeExe, nil }
	yesFlag = true
	selfUpdateForceFlag = false
	selfUpdateCheckFlag = false
	t.Cleanup(func() {
		githubLatestReleaseURL = origAPI
		executablePathFn = origExeFn
		yesFlag = origYes
	})

	var err error
	out := captureStdout(func() {
		err = selfUpdate(selfUpdateCmd, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error during selfUpdate: %v (output: %s)", err, out)
	}

	gotBytes, err := os.ReadFile(fakeExe)
	if err != nil {
		t.Fatalf("failed reading updated binary: %v", err)
	}
	if string(gotBytes) != string(newBinContent) {
		t.Errorf("updated binary content = %q, want %q", string(gotBytes), string(newBinContent))
	}
}
