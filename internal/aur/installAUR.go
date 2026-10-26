package aur

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/diff"
	"github.com/abhigyan-chatterjee/aurora/internal/run"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
)

// ErrReviewDeclined is returned when the user reviews a PKGBUILD and decides
// not to build it. Callers treat it as a skip, not a failure.
var ErrReviewDeclined = errors.New("build declined after PKGBUILD review")

// CacheDirOverride replaces the default build cache location. It comes from the
// config file so a user can move the cache onto another filesystem.
var CacheDirOverride string

// CacheDir returns the AUR build cache directory.
func CacheDir() (string, error) {
	if CacheDirOverride != "" {
		return CacheDirOverride, nil
	}
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("could not determine current user: %w", err)
	}
	return filepath.Join(usr.HomeDir, ".cache", "aurora"), nil
}

// BuildOptions controls what happens between cloning a PKGBUILD and building it.
type BuildOptions struct {
	// Review is called with the cloned PKGBUILD before the build. A non-nil
	// return aborts this package. Returning ErrReviewDeclined records a skip.
	Review func(pkgbuild string) error

	// Diff prints a unified diff of the PKGBUILD against the version that was
	// last built successfully. Off by default: Aurora's audience is people
	// switching from other distributions, and a raw diff is noise to them.
	Diff bool
}

// InstallAUR clones an AUR package, optionally reviews or diffs it, and builds
// it with makepkg.
func InstallAUR(packageName string, opts BuildOptions) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("running Aurora as root (e.g. via sudo) is not allowed when building AUR packages; makepkg requires a non-root user")
	}

	if !isValidPackageName(packageName) {
		return fmt.Errorf("invalid package name: %s", packageName)
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("could not create cache directory: %w", err)
	}

	// packageName is validated above to contain no path separators, no leading
	// dot and no "..", so the join cannot escape cacheDir. An extra symlink
	// check here would be unreachable code pretending to be defence.
	pkgDir := filepath.Join(cacheDir, packageName)

	if err := os.RemoveAll(pkgDir); err != nil {
		return fmt.Errorf("could not clear previous build directory %s: %w", pkgDir, err)
	}

	repoURL := fmt.Sprintf("https://aur.archlinux.org/%s.git", packageName)
	ui.Out("Cloning %s...\n", repoURL)
	// The AUR repository holds only build files, so a shallow single-branch
	// clone is enough and much faster than the full history.
	if err := run.Interactive(run.BuildTimeout, "git", "clone", "--depth", "1", "--single-branch", repoURL, pkgDir); err != nil {
		return fmt.Errorf("failed to clone %s: %w", packageName, err)
	}

	pkgbuildPath := filepath.Join(pkgDir, "PKGBUILD")
	pkgbuildBytes, err := os.ReadFile(pkgbuildPath)
	if err != nil {
		return fmt.Errorf("could not read the cloned PKGBUILD: %w", err)
	}
	pkgbuild := string(pkgbuildBytes)

	if opts.Diff {
		showPKGBUILDDiff(packageName, pkgbuild)
	}

	if opts.Review != nil {
		if err := opts.Review(pkgbuild); err != nil {
			return err
		}
	}

	ui.Out("Building package...\n")
	if err := run.InteractiveDir(run.BuildTimeout, pkgDir, "makepkg", "-si", "--needed", "--noconfirm"); err != nil {
		return fmt.Errorf("makepkg failed: %w", err)
	}

	// Record what was actually built so the next --diff has a baseline.
	saveBaseline(packageName, pkgbuild)
	return nil
}

// baselinePath is where the last successfully built PKGBUILD is kept.
func baselinePath(packageName string) (string, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "pkgbuilds", packageName+".PKGBUILD"), nil
}

// ReadBaseline returns the last successfully built PKGBUILD for a package.
func ReadBaseline(packageName string) (string, bool) {
	path, err := baselinePath(packageName)
	if err != nil {
		return "", false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(content), true
}

func saveBaseline(packageName, pkgbuild string) {
	path, err := baselinePath(packageName)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(pkgbuild), 0o644)
}

func showPKGBUILDDiff(packageName, current string) {
	previous, ok := ReadBaseline(packageName)
	if !ok {
		ui.Outln("\nFirst build of " + packageName + "; no previous PKGBUILD to compare against.")
		return
	}

	d := diff.UnifiedText(packageName+" (last built)", packageName+" (now)", previous, current)
	if d == "" {
		ui.Out("\nPKGBUILD for %s is unchanged since the last build.\n", packageName)
		return
	}
	ui.Out("\n--- PKGBUILD changes for %s ---\n%s", packageName, d)
}

// isValidPackageName enforces Arch package naming and blocks path traversal and
// flag injection before a name reaches git or the filesystem.
func isValidPackageName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") || strings.Contains(name, "..") {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '+' || c == '@') {
			return false
		}
	}
	return true
}
