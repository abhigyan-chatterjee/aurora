package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/abhigyan-chatterjee/aurora/internal/updates"

	"github.com/spf13/cobra"
)

var (
	updateDryRunFlag bool
	updateDiffFlag   bool
	updateNoDepsFlag bool
)

func update(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	ui.Outln("Checking for updates...")
	report := updates.Collect(appConfig.Parallelism)
	for _, w := range report.Warnings {
		ui.Warn("%v", w)
	}

	repoEntries := report.Repositories()
	aurEntries := report.AUR()

	// Resolve AUR build order before showing the plan so the order the user
	// approves is the order that will run.
	aurPlan := make([]installPlanItem, 0, len(aurEntries))
	for _, e := range aurEntries {
		aurPlan = append(aurPlan, installPlanItem{
			Name:    e.Name,
			Source:  resolve.SourceAUR,
			Version: e.Candidate,
		})
	}
	if len(aurPlan) > 0 && !updateNoDepsFlag && appConfig.AURDeps {
		aurPlan = orderAURInstallPlan(aurPlan)
	}

	ui.Outln("\nAurora update plan:")
	ui.Outln("  Step 1 - System upgrade: refresh package databases and upgrade repository packages")
	if len(repoEntries) > 0 {
		ui.Out("           %d repository package(s) will be upgraded\n", len(repoEntries))
	}

	switch {
	case len(aurPlan) > 0:
		ui.Out("  Step 2 - AUR packages to rebuild (%d):\n", len(aurPlan))
		for _, item := range aurPlan {
			label := item.Version
			if item.IsDep {
				label += ", dependency"
			}
			ui.Out("    - %s (%s)\n", item.Name, label)
		}
	case report.AURCheckFailed:
		ui.Outln("  Step 2 - AUR packages: could not check for updates (see the warning above)")
	default:
		ui.Outln("  Step 2 - AUR packages: all up to date")
	}

	if !report.CheckupdatesAvailable {
		ui.Outln("\nNote: install pacman-contrib to have repository upgrades listed in this plan.")
	}

	if updateDryRunFlag {
		ui.Outln("\nDry run: nothing was upgraded.")
		return nil
	}

	if !ui.ConfirmAction(reader, "\nProceed with update?", yesFlag) {
		ui.Outln("Update cancelled.")
		return nil
	}

	// The system upgrade runs first so AUR packages are rebuilt against the
	// newest libraries. Building first risks linking against a library the
	// upgrade replaces moments later.
	ui.Outln("\n--- Upgrading System Packages ---")
	if err := pacman.SystemUpdate(appConfig.InteractivePacman); err != nil {
		ui.Outln("Skipping AUR rebuilds: re-run 'aurora update' once the system upgrade succeeds.")
		return fmt.Errorf("system upgrade failed: %w", err)
	}
	ui.Outln("System upgrade complete.")

	if len(aurPlan) == 0 {
		return nil
	}

	ui.Outln("\n--- Upgrading AUR Packages ---")
	var buildErrs []error
	for _, item := range aurPlan {
		ui.Out("\nUpdating %s...\n", item.Name)
		err := aur.InstallAUR(item.Name, aur.BuildOptions{
			Diff: updateDiffFlag || appConfig.PKGBUILDDiff,
		})
		if err != nil {
			buildErrs = append(buildErrs, fmt.Errorf("updating %s: %w", item.Name, err))
		}
	}
	return errors.Join(buildErrs...)
}

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"upgrade"},
	Short:   "Checks for updates and performs a full system upgrade",
	Long: `Checks the AUR and the configured repositories for upgrades, then performs a
full system upgrade with 'sudo pacman -Syu'.

The system upgrade runs FIRST, then outdated AUR packages are rebuilt, so AUR
builds link against the newest system libraries (paru-style behaviour).

Examples:
  aurora update              # upgrade everything
  aurora update --dry-run    # show the plan and stop
  aurora update --diff       # show PKGBUILD changes before each AUR rebuild`,
	RunE: update,
}

func init() {
	updateCmd.Flags().BoolVar(&updateDryRunFlag, "dry-run", false, "show the update plan without changing anything")
	updateCmd.Flags().BoolVar(&updateDiffFlag, "diff", false, "show PKGBUILD changes before each AUR rebuild")
	updateCmd.Flags().BoolVar(&updateNoDepsFlag, "no-deps", false, "do not resolve or reorder AUR dependencies")
	rootCmd.AddCommand(updateCmd)
}
