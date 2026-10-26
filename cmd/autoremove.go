package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/spf13/cobra"
)

func autoremove(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	ui.Outln("Checking for unused orphan dependencies...")
	orphans, err := pacman.Orphans()
	if err != nil {
		ui.Errf("Error checking for orphan packages: %v", err)
		return err
	}

	if len(orphans) == 0 {
		ui.Outln("No orphan packages found. Your system is clean.")
		return nil
	}

	ui.Preview("\nFound %d orphan package(s) no longer required by any installed package:\n", len(orphans))
	for _, p := range orphans {
		ui.Preview("  - %s\n", p)
	}

	if !conciseFlag {
		ui.Outln("\nAurora will run: sudo pacman -Rns <packages>")
		ui.Outln("Meaning: remove those dependencies and their unused configuration files.")
	}

	if !ui.ConfirmAction(reader, "\nProceed with autoremove?", yesFlag) {
		ui.Outln("Autoremove cancelled.")
		return nil
	}

	if err := pacman.RemovePacman(orphans, appConfig.InteractivePacman); err != nil {
		return fmt.Errorf("autoremove failed: %w", err)
	}

	ui.Outln("\nAutoremove completed. System cleaned.")
	return nil
}

var autoremoveCmd = &cobra.Command{
	Use:     "autoremove",
	Aliases: []string{"orphans"},
	Short:   "Removes unused orphan packages and leftover dependencies",
	Long: `Autoremove scans for orphan dependencies (packages installed as dependencies for
software that has since been removed) and removes them with 'sudo pacman -Rns'.

This is the apt-style 'autoremove' for Arch.`,
	RunE: autoremove,
}

func init() {
	rootCmd.AddCommand(autoremoveCmd)
}
