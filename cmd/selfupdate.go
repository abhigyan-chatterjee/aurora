package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/spf13/cobra"
)

var (
	selfUpdateForceFlag bool
	selfUpdateCheckFlag bool
)

var (
	githubLatestReleaseURL = "https://api.github.com/repos/abhigyan-chatterjee/aurora/releases/latest"
	executablePathFn       = os.Executable
	// currentArch is a variable so tests can simulate a non-amd64 machine.
	currentArch = runtime.GOARCH
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

var semverPattern = regexp.MustCompile(`v?(\d+\.\d+(?:\.\d+)?)`)

// extractReleaseVersion pulls a numeric version out of a tag or release title.
func extractReleaseVersion(tagName, releaseName string) string {
	for _, candidate := range []string{tagName, releaseName} {
		trimmed := strings.TrimSpace(candidate)
		if m := semverPattern.FindStringSubmatch(trimmed); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

// verifyFileDigest verifies a downloaded binary against a "sha256:<hex>" digest.
// An empty expected digest is an error: skipping verification silently is how a
// supply-chain check turns into decoration.
func verifyFileDigest(filePath, expectedDigest string) error {
	expectedDigest = strings.TrimSpace(expectedDigest)
	if expectedDigest == "" {
		return fmt.Errorf("no SHA256 digest is available for this download")
	}

	wantHex := strings.TrimPrefix(strings.ToLower(expectedDigest), "sha256:")
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	gotHex := hex.EncodeToString(h.Sum(nil))
	if gotHex != wantHex {
		return fmt.Errorf("SHA256 checksum mismatch: got %s, expected %s", gotHex, wantHex)
	}
	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", githubLatestReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "aurora/"+rootCmd.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %s", resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// findAuroraAsset selects the release binary built for this machine.
//
// Release assets are named per architecture. Selecting by name alone handed an
// amd64 binary to aarch64 users, which replaced a working executable with one
// that cannot run. A bare "aurora" asset is accepted only on amd64, where it is
// what older releases published.
func findAuroraAsset(assets []githubAsset, goarch string) *githubAsset {
	preferred := []string{
		"aurora-linux-" + goarch,
		"aurora_linux_" + goarch,
	}
	for _, want := range preferred {
		for i := range assets {
			if strings.EqualFold(assets[i].Name, want) {
				return &assets[i]
			}
		}
	}

	if goarch == "amd64" {
		for i := range assets {
			if strings.EqualFold(assets[i].Name, "aurora") {
				return &assets[i]
			}
		}
	}
	return nil
}

// resolveDigest returns the digest to verify the binary against.
//
// GitHub populates the asset `digest` field only for assets uploaded after that
// field existed. Aurora's release workflow also publishes a .sha256 file, so the
// fallback reads it instead of skipping verification.
func resolveDigest(client *http.Client, assets []githubAsset, asset *githubAsset) (string, error) {
	if strings.TrimSpace(asset.Digest) != "" {
		return asset.Digest, nil
	}

	candidates := []string{
		asset.Name + ".sha256",
		"aurora-linux-" + currentArch + ".sha256",
		"aurora.sha256",
		"checksums.txt",
	}
	for _, candidate := range candidates {
		for i := range assets {
			if !strings.EqualFold(assets[i].Name, candidate) {
				continue
			}
			digest, err := fetchDigestFile(client, assets[i].BrowserDownloadURL, asset.Name)
			if err != nil {
				return "", err
			}
			return digest, nil
		}
	}

	return "", fmt.Errorf("release %s has no digest and no .sha256 asset to fall back on", asset.Name)
}

// fetchDigestFile downloads a checksum file and returns the "sha256:<hex>"
// entry for targetName, or the first checksum in the file when the file holds a
// single unsuffixed entry.
func fetchDigestFile(client *http.Client, url, targetName string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "aurora/"+rootCmd.Version)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download returned status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	hexRe := regexp.MustCompile(`\b([0-9a-fA-F]{64})\b`)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := hexRe.FindString(line)
		if m == "" {
			continue
		}
		// Prefer the line naming our asset; accept any line if the file is the
		// single-entry form the release workflow publishes.
		if strings.Contains(line, targetName) || len(strings.Split(strings.TrimSpace(string(body)), "\n")) == 1 {
			return "sha256:" + strings.ToLower(m), nil
		}
	}

	return "", fmt.Errorf("no SHA256 checksum for %s found in the checksum file", targetName)
}

func downloadToTempFile(url string) (string, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "aurora/"+rootCmd.Version)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "aurora-selfupdate-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

// installBinaryToTarget replaces the running executable, falling back to sudo
// when the target directory is not writable.
func installBinaryToTarget(tmpPath, targetPath string) error {
	targetDir := filepath.Dir(targetPath)
	stagedPath := filepath.Join(targetDir, ".aurora-new")

	if source, err := os.Open(tmpPath); err == nil {
		if out, createErr := os.OpenFile(stagedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755); createErr == nil {
			_, copyErr := io.Copy(out, source)
			closeErr := out.Close()
			source.Close()
			if copyErr == nil && closeErr == nil {
				if renErr := os.Rename(stagedPath, targetPath); renErr == nil {
					return nil
				}
			}
			_ = os.Remove(stagedPath)
		} else {
			source.Close()
		}
	}

	if !conciseFlag {
		ui.Out("Elevated permissions are required to update %s; running sudo install...\n", targetPath)
	}
	// Deliberately not run.Interactive: sudo may prompt for a password and the
	// prompt has no bounded duration, so a deadline here would abort a valid
	// authentication.
	cmd := exec.Command("sudo", "install", "-m", "755", tmpPath, targetPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func fallbackGoInstall(reader *bufio.Reader) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("could not fetch a release binary for linux/%s and Go is not installed on PATH", currentArch)
	}

	ui.Outln("\nFalling back to building from source with Go:")
	ui.Outln("  go install github.com/abhigyan-chatterjee/aurora@latest")

	if !ui.ConfirmAction(reader, "\nProceed with 'go install'?", yesFlag) {
		ui.Outln("Self-update cancelled.")
		return nil
	}

	// Deliberately not run.Interactive: compiling from source can legitimately
	// take minutes and the Go toolchain manages its own network timeouts.
	cmd := exec.Command("go", "install", "github.com/abhigyan-chatterjee/aurora@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}

	ui.Outln("\nAurora updated via go install.")
	return nil
}

func selfUpdate(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	currentVer := rootCmd.Version

	ui.Out("Checking for Aurora updates (current version: v%s)...\n", currentVer)

	rel, err := fetchLatestRelease()
	if err != nil {
		ui.Warn("could not query GitHub Releases: %v", err)
		if selfUpdateCheckFlag {
			return err
		}
		return fallbackGoInstall(reader)
	}

	remoteVer := extractReleaseVersion(rel.TagName, rel.Name)
	displayVer := rel.TagName
	if remoteVer != "" {
		displayVer = "v" + remoteVer
	}

	if !selfUpdateForceFlag {
		if remoteVer == "" {
			ui.Out("Note: release tag %q has no version number; use --force to reinstall it.\n", rel.TagName)
			ui.Out("Aurora is already up to date (v%s).\n", currentVer)
			return nil
		}
		if !pacman.IsNewerVersion(currentVer, remoteVer) {
			ui.Out("Aurora is already up to date (v%s).\n", currentVer)
			return nil
		}
	}

	if selfUpdateCheckFlag {
		ui.Out("New Aurora version available: %s (current: v%s)\n", displayVer, currentVer)
		return nil
	}

	asset := findAuroraAsset(rel.Assets, currentArch)
	if asset == nil {
		return fmt.Errorf("release %s has no binary for linux/%s; build from source with 'go install github.com/abhigyan-chatterjee/aurora@latest'", displayVer, currentArch)
	}

	exePath, err := executablePathFn()
	if err != nil {
		return fmt.Errorf("could not determine the current executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	ui.Out("\nAurora self-update plan:\n")
	ui.Out("  Current version : v%s\n", currentVer)
	ui.Out("  Target release  : %s\n", displayVer)
	ui.Out("  Target binary   : %s\n", asset.Name)
	ui.Out("  Target path     : %s\n", exePath)

	if !ui.ConfirmAction(reader, "\nProceed with self-update?", yesFlag) {
		ui.Outln("Self-update cancelled.")
		return nil
	}

	client := &http.Client{Timeout: 60 * time.Second}
	digest, err := resolveDigest(client, rel.Assets, asset)
	if err != nil {
		return fmt.Errorf("refusing to install an unverified binary: %w", err)
	}

	ui.Out("Downloading %s...\n", asset.BrowserDownloadURL)
	tmpPath, err := downloadToTempFile(asset.BrowserDownloadURL)
	if err != nil {
		ui.Warn("binary download failed: %v", err)
		return fallbackGoInstall(reader)
	}
	defer os.Remove(tmpPath)

	if err := verifyFileDigest(tmpPath, digest); err != nil {
		return err
	}
	if !conciseFlag {
		ui.Outln("Verified SHA256 digest.")
	}

	if err := installBinaryToTarget(tmpPath, exePath); err != nil {
		return fmt.Errorf("replacing the executable at %s: %w", exePath, err)
	}

	ui.Out("\nAurora successfully updated to %s!\n", displayVer)
	return nil
}

var selfUpdateCmd = &cobra.Command{
	Use:     "selfupdate",
	Aliases: []string{"self-update"},
	Short:   "Update Aurora itself to the latest release",
	Long: `Checks GitHub Releases for a newer Aurora, selects the build for this machine's
architecture, verifies its SHA256 digest, and replaces the current executable.

A download that cannot be verified against a digest is refused, not installed. If no
matching release binary is available, Aurora falls back to
'go install github.com/abhigyan-chatterjee/aurora@latest'.

Examples:
  aurora selfupdate          # upgrade if a newer release exists
  aurora selfupdate --check  # report whether a newer version exists
  aurora selfupdate --force  # reinstall the latest release`,
	RunE: selfUpdate,
}

func init() {
	selfUpdateCmd.Flags().BoolVarP(&selfUpdateForceFlag, "force", "f", false, "reinstall the latest release even if already up to date")
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheckFlag, "check", false, "only check whether an update is available")
	rootCmd.AddCommand(selfUpdateCmd)
}
