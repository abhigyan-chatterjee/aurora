package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/spf13/cobra"
)

var (
	cleanCacheFlag  bool
	cleanPacmanFlag bool
	cleanAllFlag    bool
	cleanDryRunFlag bool
)

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// cleanScope decides which caches to clean for a given flag combination.
// Default (no flags): Aurora's AUR build cache only.
func cleanScope(cacheFlag, pacmanFlag, allFlag bool) (doAUR, doPacman bool) {
	if allFlag {
		return true, true
	}
	if !cacheFlag && !pacmanFlag {
		return true, false
	}
	return cacheFlag, pacmanFlag
}

func clean(cmd *cobra.Command, args []string) {
	reader := bufio.NewReader(os.Stdin)

	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Error getting current user: %v\n", err)
		return
	}

	aurCacheDir := filepath.Join(usr.HomeDir, ".cache", "aurora")
	pacmanCacheDir := "/var/cache/pacman/pkg"

	doAUR, doPacman := cleanScope(cleanCacheFlag, cleanPacmanFlag, cleanAllFlag)

	var aurSize int64
	if doAUR {
		var err error
		aurSize, err = dirSize(aurCacheDir)
		if err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: error reading AUR cache size: %v\n", err)
		}
	}

	var pacmanSize int64
	if doPacman {
		var err error
		pacmanSize, err = dirSize(pacmanCacheDir)
		if err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: error reading pacman cache size: %v\n", err)
		}
	}

	if cleanDryRunFlag {
		fmt.Println("=== Aurora Cache Summary (Dry Run) ===")
		if doAUR {
			fmt.Printf("AUR Build Cache  (%s): %s\n", aurCacheDir, humanSize(aurSize))
		}
		if doPacman {
			fmt.Printf("Pacman Pkg Cache (%s): %s\n", pacmanCacheDir, humanSize(pacmanSize))
		}
		fmt.Println("\nRun 'aurora clean' without --dry-run to reclaim this space.")
		return
	}

	fmt.Println("Aurora cache cleanup plan:")
	if doAUR {
		fmt.Printf("  - Purge AUR build cache in %s (%s)\n", aurCacheDir, humanSize(aurSize))
	}
	if doPacman {
		fmt.Printf("  - Clean uninstalled packages from pacman cache (%s)\n", humanSize(pacmanSize))
	}

	if !ui.ConfirmAction(reader, "\nProceed with cache cleanup?", yesFlag) {
		fmt.Println("Cleanup cancelled.")
		return
	}

	if doAUR {
		fmt.Println("Cleaning AUR build cache...")
		_ = os.RemoveAll(aurCacheDir)
		_ = os.MkdirAll(aurCacheDir, 0755)
		fmt.Println("AUR build cache cleared.")
	}

	if doPacman {
		fmt.Println("\nCleaning pacman cache: sudo pacman -Sc --noconfirm")
		execCmd := exec.Command("sudo", "pacman", "-Sc", "--noconfirm")
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin
		if err := execCmd.Run(); err != nil {
			fmt.Printf("Error cleaning pacman cache: %v\n", err)
		}
	}

	fmt.Println("\nCache cleanup completed.")
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean AUR build files and package caches",
	Long: `Clean purges temporary build files and downloaded git repositories in ~/.cache/aurora.
It can also trigger 'sudo pacman -Sc' to prune old uninstalled packages from pacman's package cache.

Examples:
  aurora clean            # Clean AUR build cache (~/.cache/aurora)
  aurora clean --pacman   # Clean pacman's package cache only
  aurora clean --dry-run  # Check how much disk space can be reclaimed
  aurora clean --all      # Clean both AUR build cache and pacman package cache`,
	Run: clean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanCacheFlag, "cache", false, "clean Aurora's AUR build cache (~/.cache/aurora); default when no flags are given")
	cleanCmd.Flags().BoolVar(&cleanPacmanFlag, "pacman", false, "clean uninstalled packages from pacman's cache (sudo pacman -Sc)")
	cleanCmd.Flags().BoolVarP(&cleanAllFlag, "all", "a", false, "clean both AUR build cache and pacman package cache")
	cleanCmd.Flags().BoolVarP(&cleanDryRunFlag, "dry-run", "n", false, "show reclaimable disk space without deleting anything")
	rootCmd.AddCommand(cleanCmd)
}
