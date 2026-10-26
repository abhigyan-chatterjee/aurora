package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"

	"github.com/spf13/cobra"
)

func remove(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please specify a package name to remove")
	}

	reader := bufio.NewReader(os.Stdin)

	validArgs, nameErrs := collectValidArgs(args)
	if len(validArgs) == 0 {
		return errors.Join(nameErrs...)
	}

	var (
		removeList []string
		removeSeen = make(map[string]bool)
		removeErrs []error
	)

	add := func(name string) {
		norm := strings.ToLower(name)
		if removeSeen[norm] {
			return
		}
		removeSeen[norm] = true
		removeList = append(removeList, name)
	}

	for _, pkgQuery := range validArgs {
		matches, err := pacman.SearchInstalled(pkgQuery)
		if err != nil {
			ui.Errf("Error checking installed packages for %q: %v", pkgQuery, err)
			removeErrs = append(removeErrs, err)
			continue
		}

		if len(matches) == 0 {
			ui.Out("No installed package found matching %q.\n", pkgQuery)
			removeErrs = append(removeErrs, fmt.Errorf("no installed package matching %q", pkgQuery))
			continue
		}

		// An exact name match wins outright.
		exact := ""
		for _, m := range matches {
			if strings.EqualFold(m, pkgQuery) {
				exact = m
				break
			}
		}
		if exact != "" {
			add(exact)
			continue
		}

		if len(matches) == 1 {
			add(matches[0])
			continue
		}

		// Several partial matches. Offer a picker for a single argument and
		// refuse to guess in a batch.
		if len(validArgs) > 1 {
			ui.Warn("multiple packages match %q; run `aurora remove %s` with the exact name to disambiguate; skipping", pkgQuery, pkgQuery)
			removeErrs = append(removeErrs, fmt.Errorf("ambiguous package name %q", pkgQuery))
			continue
		}

		ui.Out("\nFound multiple packages matching %q:\n", pkgQuery)
		items := make([]ui.SelectionItem, 0, len(matches))
		for i, m := range matches {
			items = append(items, ui.SelectionItem{Index: i + 1, Name: m})
			ui.Out("  [%d] %s\n", i+1, m)
		}

		selected := ui.PromptSelection(reader, "\nEnter item number or exact package name to remove (or press Enter to skip): ", items)
		if selected == nil {
			ui.Outln("Removal cancelled.")
			return errors.Join(removeErrs...)
		}
		add(selected.Name)
	}

	if len(removeList) == 0 {
		return errors.Join(append(nameErrs, removeErrs...)...)
	}

	ui.Preview("\nAurora is about to remove:\n")
	for _, pkg := range removeList {
		ui.Preview("  - %s\n", pkg)
	}

	if !ui.ConfirmAction(reader, "\nAre you sure?", yesFlag) {
		ui.Outln("Removal cancelled.")
		return errors.Join(removeErrs...)
	}

	if err := pacman.RemovePacman(removeList, appConfig.InteractivePacman); err != nil {
		removeErrs = append(removeErrs, fmt.Errorf("removing packages: %w", err))
		return errors.Join(append(nameErrs, removeErrs...)...)
	}

	ui.Out("Successfully removed: %s\n", strings.Join(removeList, ", "))
	return errors.Join(append(nameErrs, removeErrs...)...)
}

var removeCmd = &cobra.Command{
	Use:   "remove <package> [package...]",
	Short: "Removes specified packages from the system",
	Long: `Removes packages safely with 'sudo pacman -Rns'.
This removes the target packages as well as dependencies that nothing else needs.

Examples:
  aurora remove neovim
  aurora remove neovim git`,
	RunE:              remove,
	ValidArgsFunction: completePackageNames,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
