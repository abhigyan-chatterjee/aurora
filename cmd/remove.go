package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"

	"github.com/spf13/cobra"
)

func remove(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Println("Please specify a package name to remove.")
		return
	}

	reader := bufio.NewReader(os.Stdin)

	var validArgs []string
	seen := make(map[string]bool)
	for _, arg := range args {
		if !ui.IsValidPkgName(arg) {
			fmt.Printf("Warning: invalid package name format: %s\n", arg)
			continue
		}
		norm := strings.ToLower(arg)
		if seen[norm] {
			continue
		}
		seen[norm] = true
		validArgs = append(validArgs, arg)
	}

	if len(validArgs) == 0 {
		return
	}

	var removeList []string
	removeSeen := make(map[string]bool)

	for _, pkgQuery := range validArgs {
		matches, err := pacman.SearchInstalled(pkgQuery)
		if err != nil {
			fmt.Printf("Error checking installed packages for \"%s\": %v\n", pkgQuery, err)
			continue
		}

		if len(matches) == 0 {
			fmt.Printf("No installed package found matching \"%s\".\n", pkgQuery)
			continue
		}

		var exactMatch string
		hasExact := false
		for _, m := range matches {
			if strings.EqualFold(m, pkgQuery) {
				exactMatch = m
				hasExact = true
				break
			}
		}

		if hasExact {
			norm := strings.ToLower(exactMatch)
			if !removeSeen[norm] {
				removeSeen[norm] = true
				removeList = append(removeList, exactMatch)
			}
		} else if len(matches) == 1 {
			pkgName := matches[0]
			norm := strings.ToLower(pkgName)
			if !removeSeen[norm] {
				removeSeen[norm] = true
				removeList = append(removeList, pkgName)
			}
		} else {
			if len(validArgs) == 1 {
				fmt.Printf("\nFound multiple packages matching \"%s\":\n", pkgQuery)
				var items []ui.SelectionItem
				for i, m := range matches {
					items = append(items, ui.SelectionItem{
						Index: i + 1,
						Name:  m,
					})
					fmt.Printf("  [%d] %s\n", i+1, m)
				}
				_, selected := ui.PromptSelection(reader, "\nEnter item number or exact package name to remove (or press Enter to skip): ", items)
				if selected == nil {
					fmt.Println("Removal cancelled.")
					return
				}
				if !removeSeen[selected.Name] {
					removeSeen[selected.Name] = true
					removeList = append(removeList, selected.Name)
				}
			} else {
				fmt.Printf("Warning: multiple packages match \"%s\" — run `aurora remove <exact>` to disambiguate; skipping.\n", pkgQuery)
			}
		}
	}

	if len(removeList) == 0 {
		return
	}

	fmt.Println("\nAurora is about to remove:")
	for _, pkg := range removeList {
		fmt.Printf("  - %s\n", pkg)
	}

	if !ui.ConfirmAction(reader, "\nAre you sure?", yesFlag) {
		fmt.Println("Removal cancelled.")
		return
	}

	err := pacman.RemovePacman(removeList...)
	if err != nil {
		fmt.Printf("Error removing packages: %v\n", err)
	} else {
		fmt.Printf("Successfully removed: %s\n", strings.Join(removeList, ", "))
	}
}

var removeCmd = &cobra.Command{
	Use:   "remove <package> [package...]",
	Short: "Removes specified packages from the system",
	Long: `Removes packages safely by running 'sudo pacman -Rns --noconfirm'.
This removes the target package as well as any unneeded dependencies left behind.

Examples:
  aurora remove neovim
  aurora remove neovim git`,
	Run: remove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
