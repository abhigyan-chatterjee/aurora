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

// extractReleaseVersion extracts a clean numeric version string from a GitHub release
// tag_name or title (e.g. "v0.3.0" -> "0.3.0", or tag "major" with name "v0.2" -> "0.2").
func extractReleaseVersion(tagName, releaseName string) string {
	for _, candidate := range []string{tagName, releaseName} {
		trimmed := strings.TrimSpace(candidate)
		if m := semverPattern.FindStringSubmatch(trimmed); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

// verifyFileDigest verifies a downloaded binary against a "sha256:<hex>" digest string
// provided by GitHub Releases. If expectedDigest is empty, verification is skipped.
func verifyFileDigest(filePath, expectedDigest string) error {
	expectedDigest = strings.TrimSpace(expectedDigest)
	if expectedDigest == "" {
		return nil
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

func findAuroraAsset(assets []githubAsset) *githubAsset {
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if name == "aurora" || strings.HasPrefix(name, "aurora-linux") || strings.HasPrefix(name, "aurora_linux") {
			return &assets[i]
		}
	}
	return nil
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
	if err := os.Chmod(tmpPath, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func installBinaryToTarget(tmpPath, targetPath string) error {
	targetDir := filepath.Dir(targetPath)
	stagedPath := filepath.Join(targetDir, ".aurora-new")

	if f, err := os.Open(tmpPath); err == nil {
		if out, createErr := os.OpenFile(stagedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755); createErr == nil {
			_, copyErr := io.Copy(out, f)
			closeErr := out.Close()
			f.Close()
			if copyErr == nil && closeErr == nil {
				if renErr := os.Rename(stagedPath, targetPath); renErr == nil {
					return nil
				}
			}
			_ = os.Remove(stagedPath)
		} else {
			f.Close()
		}
	}

	if !conciseFlag {
		fmt.Printf("Elevated permissions required to update %s; running sudo install...\n", targetPath)
	}
	cmd := exec.Command("sudo", "install", "-m", "755", tmpPath, targetPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func fallbackGoInstall(reader *bufio.Reader) error {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("could not fetch release binary and Go is not installed on PATH")
	}

	fmt.Println("\nFalling back to building from source via Go:")
	fmt.Println("  go install github.com/abhigyan-chatterjee/aurora@latest")

	if !ui.ConfirmAction(reader, "\nProceed with 'go install'?", yesFlag) {
		fmt.Println("Self-update cancelled.")
		return nil
	}

	cmd := exec.Command(goPath, "install", "github.com/abhigyan-chatterjee/aurora@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}

	fmt.Println("\nAurora updated via 'go install'.")
	return nil
}

func selfUpdate(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	currentVer := rootCmd.Version

	fmt.Printf("Checking for Aurora updates (current version: v%s)...\n", currentVer)

	rel, err := fetchLatestRelease()
	if err != nil {
		fmt.Printf("Warning: could not query GitHub Releases: %v\n", err)
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
		if remoteVer != "" {
			if !pacman.IsNewerVersion(currentVer, remoteVer) {
				fmt.Printf("Aurora is already up to date (v%s).\n", currentVer)
				return nil
			}
		} else {
			fmt.Printf("Note: release tag %q does not contain a semver number; use --force to reinstall from this release.\n", rel.TagName)
			fmt.Printf("Aurora is already up to date (v%s).\n", currentVer)
			return nil
		}
	}

	if selfUpdateCheckFlag {
		fmt.Printf("New Aurora version available: %s (current: v%s)\n", displayVer, currentVer)
		return nil
	}

	asset := findAuroraAsset(rel.Assets)
	if asset == nil {
		fmt.Println("Warning: no pre-built binary asset found in the latest GitHub release.")
		return fallbackGoInstall(reader)
	}

	exePath, err := executablePathFn()
	if err != nil {
		return fmt.Errorf("could not determine current executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	fmt.Printf("\nAurora self-update plan:\n")
	fmt.Printf("  Current version : v%s\n", currentVer)
	fmt.Printf("  Target release  : %s\n", displayVer)
	fmt.Printf("  Target path     : %s\n", exePath)

	if !ui.ConfirmAction(reader, "\nProceed with self-update?", yesFlag) {
		fmt.Println("Self-update cancelled.")
		return nil
	}

	fmt.Printf("Downloading %s...\n", asset.BrowserDownloadURL)
	tmpPath, err := downloadToTempFile(asset.BrowserDownloadURL)
	if err != nil {
		fmt.Printf("Warning: binary download failed: %v\n", err)
		return fallbackGoInstall(reader)
	}
	defer os.Remove(tmpPath)

	if err := verifyFileDigest(tmpPath, asset.Digest); err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}
	if asset.Digest != "" && !conciseFlag {
		fmt.Println("Verified SHA256 digest against GitHub Release metadata.")
	}

	if err := installBinaryToTarget(tmpPath, exePath); err != nil {
		fmt.Printf("Error replacing executable at %s: %v\n", exePath, err)
		return err
	}

	fmt.Printf("\nAurora successfully updated to %s!\n", displayVer)
	return nil
}

var selfUpdateCmd = &cobra.Command{
	Use:     "selfupdate",
	Aliases: []string{"self-update"},
	Short:   "Update Aurora itself to the latest release",
	Long: `Checks GitHub Releases (abhigyan-chatterjee/aurora) for a newer version of Aurora,
verifies the binary's SHA256 digest when available, and replaces the current executable.
If no pre-built release binary is reachable, falls back to 'go install github.com/abhigyan-chatterjee/aurora@latest'.

Examples:
  aurora selfupdate          # Upgrade Aurora if a newer release is available
  aurora selfupdate --check  # Check if a newer version exists without installing
  aurora selfupdate --force  # Reinstall the latest release binary even if already up to date`,
	RunE: selfUpdate,
}

func init() {
	selfUpdateCmd.Flags().BoolVarP(&selfUpdateForceFlag, "force", "f", false, "reinstall the latest release even if already up to date")
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheckFlag, "check", false, "only check if an update is available without installing")
	rootCmd.AddCommand(selfUpdateCmd)
}
