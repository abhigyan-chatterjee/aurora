package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/lookup"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"

	"github.com/spf13/cobra"
)

var (
	searchFromFlag     string
	searchOfficialFlag bool
	searchAURFlag      bool
)

func search(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Println("Please provide a package name to search for.")
		return
	}

	reader := bufio.NewReader(os.Stdin)

	// Evaluate source filter
	sourceFilter := strings.ToLower(strings.TrimSpace(searchFromFlag))
	if searchOfficialFlag {
		sourceFilter = "official"
	} else if searchAURFlag {
		sourceFilter = "aur"
	}

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

	var allItems []ui.SelectionItem
	pacmanPkgMap := make(map[int]resolve.PacmanResult)
	aurPkgMap := make(map[int]resolve.AURResult)

	itemCounter := 1

	for _, query := range validArgs {
		if len(validArgs) > 1 {
			fmt.Printf("\n--- Search: %s ---\n", query)
		} else {
			fmt.Printf("\nResults for \"%s\"\n", query)
		}

		var pacmanResults []resolve.PacmanResult
		var aurResults []resolve.AURResult
		var pacmanErr, aurErr error

		if sourceFilter == "official" || sourceFilter == "repo" {
			pacmanResults, pacmanErr = pacman.SearchPacman(query)
		} else if sourceFilter == "aur" {
			aurResults, aurErr = aur.SearchAUR(query)
		} else {
			pacmanResults, aurResults, pacmanErr, aurErr = lookup.SearchBoth(query)
		}

		if pacmanErr != nil {
			fmt.Printf("Warning (official repos): %v\n", pacmanErr)
		}
		if aurErr != nil {
			fmt.Printf("Warning (AUR): %v\n", aurErr)
		}

		if len(pacmanResults) == 0 && len(aurResults) == 0 {
			if pacmanErr == nil && aurErr == nil {
				fmt.Printf("No results found for \"%s\" in specified sources.\n", query)
			}
			continue
		}

		if len(pacmanResults) > 0 {
			fmt.Println("\nOfficial repositories")
			const maxOfficialDisplay = 15
			for i, r := range pacmanResults {
				if i >= maxOfficialDisplay {
					fmt.Printf("  ... and %d more official packages (refine query for specific results)\n", len(pacmanResults)-maxOfficialDisplay)
					break
				}
				item := ui.SelectionItem{
					Index:       itemCounter,
					Name:        r.Name,
					FullName:    r.Repository + "/" + r.Name,
					SourceLabel: "official repositories",
					Version:     r.Version,
					Description: r.Description,
				}
				allItems = append(allItems, item)
				pacmanPkgMap[itemCounter] = r

				fmt.Printf("  [%d] %-25s %s\n", itemCounter, r.Repository+"/"+r.Name, r.Version)
				if r.Description != "" {
					fmt.Printf("      %s\n", r.Description)
				}
				itemCounter++
			}
		}

		if len(aurResults) > 0 {
			fmt.Println("\nAUR")
			const maxAURDisplay = 10
			for i, r := range aurResults {
				if i >= maxAURDisplay {
					fmt.Printf("  ... and %d more AUR packages (refine query for specific results)\n", len(aurResults)-maxAURDisplay)
					break
				}
				item := ui.SelectionItem{
					Index:       itemCounter,
					Name:        r.Name,
					SourceLabel: "AUR",
					Version:     r.Version,
					Description: r.Description,
				}
				allItems = append(allItems, item)
				aurPkgMap[itemCounter] = r

				fmt.Printf("  [%d] %-25s %s\n", itemCounter, r.Name, r.Version)
				if r.Description != "" {
					fmt.Printf("      %s\n", r.Description)
				}
				fmt.Printf("      Votes: %d\n", r.NumVotes)
				itemCounter++
			}
		}
	}

	if len(allItems) == 0 {
		return
	}

	prompt := "\nEnter item number or package name to view details (or press Enter to skip): "
	_, selected := ui.PromptSelection(reader, prompt, allItems)
	if selected == nil {
		return
	}

	if pkg, found := pacmanPkgMap[selected.Index]; found {
		displayPacmanDetail(pkg)
		return
	}
	if pkg, found := aurPkgMap[selected.Index]; found {
		displayAURDetail(pkg)
		return
	}
}

func displayPacmanDetail(pkg resolve.PacmanResult) {
	fmt.Printf("\n--- Package Details ---\n")
	fmt.Printf("Name        : %s\n", pkg.Name)
	fmt.Printf("Version     : %s\n", pkg.Version)
	fmt.Printf("Repository  : %s\n", pkg.Repository)
	fmt.Printf("Description : %s\n", pkg.Description)
	fmt.Printf("Source      : official repositories\n")
	if !conciseFlag {
		fmt.Println("\nRecommended: install via official repositories using pacman.")
	}
}

func displayAURDetail(pkg resolve.AURResult) {
	fmt.Printf("\n--- Package Details ---\n")
	fmt.Printf("Name        : %s\n", pkg.Name)
	fmt.Printf("Version     : %s\n", pkg.Version)
	fmt.Printf("Description : %s\n", pkg.Description)
	fmt.Printf("URL         : %s\n", pkg.URL)
	fmt.Printf("Votes       : %d\n", pkg.NumVotes)
	fmt.Printf("Source      : AUR\n")
	if !conciseFlag {
		fmt.Println("\nNote: installing from AUR means Aurora will clone the build files and build locally using makepkg.")
	}
}

var searchCmd = &cobra.Command{
	Use:   "search <query> [query...]",
	Short: "Search for packages in official repositories and the AUR",
	Long: `Search queries official Arch Linux repositories and the Arch User Repository concurrently.
Use --from to restrict searches to official repositories or AUR.

Examples:
  aurora search neovim git
  aurora search neovim --from official
  aurora search yay -A`,
	Run: search,
}

func init() {
	searchCmd.Flags().StringVar(&searchFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	searchCmd.Flags().BoolVarP(&searchOfficialFlag, "official", "O", false, "search only official repositories")
	searchCmd.Flags().BoolVarP(&searchAURFlag, "aur", "A", false, "search only the Arch User Repository")
	rootCmd.AddCommand(searchCmd)
}
