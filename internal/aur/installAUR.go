package aur

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

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

func InstallAUR(packageName string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("running Aurora as root (e.g. via sudo) is not allowed when building AUR packages; makepkg requires a non-root user")
	}

	if !isValidPackageName(packageName) {
		return fmt.Errorf("invalid package name: %s", packageName)
	}

	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("could not determine current user: %v", err)
	}

	cacheDir := filepath.Join(usr.HomeDir, ".cache", "aurora")

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create cache directory: %v", err)
	}

	pkgDir := filepath.Join(cacheDir, packageName)

	if resolved, err := filepath.EvalSymlinks(cacheDir); err == nil {
		cacheDir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(pkgDir)); err == nil {
		if !strings.HasPrefix(resolved, cacheDir) {
			return fmt.Errorf("package path escapes cache directory: %s", packageName)
		}
	}

	os.RemoveAll(pkgDir)

	repoURL := fmt.Sprintf("https://aur.archlinux.org/%s.git", packageName)
	fmt.Printf("Cloning %s...\n", repoURL)
	cloneCmd := exec.Command("git", "clone", repoURL, pkgDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone: %v", err)
	}

	fmt.Println("Building package...")
	buildCmd := exec.Command("makepkg", "-si", "--noconfirm")
	buildCmd.Dir = pkgDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdin = os.Stdin

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("makepkg failed: %v", err)
	}

	return nil
}
