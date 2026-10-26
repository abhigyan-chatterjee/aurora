package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
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

func clean(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	aurCacheDir, err := aur.CacheDir()
	if err != nil {
		return err
	}
	pacmanCacheDir := "/var/cache/pacman/pkg"

	doAUR, doPacman := cleanScope(cleanCacheFlag, cleanPacmanFlag, cleanAllFlag)

	var aurSize, pacmanSize int64
	var sizeErrs []error

	if doAUR {
		aurSize, err = dirSize(aurCacheDir)
		if err != nil && !os.IsNotExist(err) {
			ui.Warn("could not measure the AUR cache: %v", err)
			sizeErrs = append(sizeErrs, err)
		}
	}
	if doPacman {
		pacmanSize, err = dirSize(pacmanCacheDir)
		if err != nil && !os.IsNotExist(err) {
			ui.Warn("could not measure the pacman cache: %v", err)
			sizeErrs = append(sizeErrs, err)
		}
	}

	if cleanDryRunFlag {
		ui.Outln("=== Aurora Cache Summary (Dry Run) ===")
		if doAUR {
			ui.Out("AUR Build Cache  (%s): %s\n", aurCacheDir, humanSize(aurSize))
		}
		if doPacman {
			ui.Out("Pacman Pkg Cache (%s): %s\n", pacmanCacheDir, humanSize(pacmanSize))
		}
		ui.Outln("\nRe-run without --dry-run to reclaim this space.")
		return errors.Join(sizeErrs...)
	}

	ui.Outln("Aurora cache cleanup plan:")
	if doAUR {
		ui.Out("  - Purge the AUR build cache in %s (%s)\n", aurCacheDir, humanSize(aurSize))
	}
	if doPacman {
		ui.Out("  - Remove uninstalled packages from the pacman cache (%s)\n", humanSize(pacmanSize))
	}

	if !ui.ConfirmAction(reader, "\nProceed with cache cleanup?", yesFlag) {
		ui.Outln("Cleanup cancelled.")
		return errors.Join(sizeErrs...)
	}

	var cleanErrs []error

	if doAUR {
		ui.Outln("Cleaning AUR build cache...")
		if err := os.RemoveAll(aurCacheDir); err != nil {
			cleanErrs = append(cleanErrs, fmt.Errorf("clearing the AUR cache: %w", err))
		} else if err := os.MkdirAll(aurCacheDir, 0o755); err != nil {
			cleanErrs = append(cleanErrs, fmt.Errorf("recreating the AUR cache directory: %w", err))
		} else {
			ui.Outln("AUR build cache cleared.")
		}
	}

	if doPacman {
		args := []string{"pacman", "-Sc"}
		if !appConfig.InteractivePacman {
			args = append(args, "--noconfirm")
		}
		ui.Out("\nCleaning pacman cache: sudo %v\n", args)
		if err := pacman.RunSudo(args...); err != nil {
			cleanErrs = append(cleanErrs, fmt.Errorf("cleaning the pacman cache: %w", err))
		}
	}

	if len(cleanErrs) == 0 {
		ui.Outln("\nCache cleanup completed.")
	}
	return errors.Join(append(sizeErrs, cleanErrs...)...)
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean AUR build files and package caches",
	Long: `Clean purges Aurora's cloned build files and downloaded AUR repositories in
~/.cache/aurora. It can also prune uninstalled packages from pacman's cache.

Examples:
  aurora clean            # clean the AUR build cache
  aurora clean --pacman   # clean pacman's package cache only
  aurora clean --dry-run  # report how much disk space can be reclaimed
  aurora clean --all      # clean both`,
	RunE: clean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanCacheFlag, "cache", false, "clean Aurora's AUR build cache; the default when no flags are given")
	cleanCmd.Flags().BoolVar(&cleanPacmanFlag, "pacman", false, "clean uninstalled packages from pacman's cache")
	cleanCmd.Flags().BoolVarP(&cleanAllFlag, "all", "a", false, "clean both the AUR build cache and the pacman cache")
	cleanCmd.Flags().BoolVarP(&cleanDryRunFlag, "dry-run", "n", false, "show reclaimable disk space without deleting anything")
	rootCmd.AddCommand(cleanCmd)
}
