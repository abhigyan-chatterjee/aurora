package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"

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

const (
	maxOfficialDisplay = 15
	maxAURDisplay      = 10
)

// searchResultJSON is one query's results, shaped for --json consumers.
type searchResultJSON struct {
	Query    string             `json:"query"`
	Official []pacmanResultJSON `json:"official"`
	AUR      []aurResultJSON    `json:"aur"`
}

type pacmanResultJSON struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Repo        string `json:"repo"`
	Description string `json:"description"`
}

type aurResultJSON struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Votes       int    `json:"votes"`
	Description string `json:"description"`
	URL         string `json:"url"`
	OutOfDate   bool   `json:"out_of_date"`
}

func search(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a package name to search for")
	}

	filter, err := parseSourceFilter(searchFromFlag, searchOfficialFlag, searchAURFlag)
	if err != nil {
		return err
	}

	validArgs, nameErrs := collectValidArgs(args)
	if len(validArgs) == 0 {
		return errors.Join(nameErrs...)
	}

	reader := bufio.NewReader(os.Stdin)

	var (
		items       []ui.SelectionItem
		pacmanByID  = make(map[int]resolve.PacmanResult)
		aurByID     = make(map[int]resolve.AURResult)
		jsonResults []searchResultJSON
		searchErrs  []error
		nextID      = 1
	)

	for _, query := range validArgs {
		if !jsonFlag {
			if len(validArgs) > 1 {
				ui.Out("\n--- Search: %s ---\n", query)
			} else {
				ui.Out("\nResults for %q\n", query)
			}
		}

		var (
			pacmanResults []resolve.PacmanResult
			aurResults    []resolve.AURResult
			pacmanErr     error
			aurErr        error
		)

		switch filter {
		case filterOfficial:
			pacmanResults, pacmanErr = pacman.SearchPacman(query)
			pacmanResults = filterOfficialResults(pacmanResults)
		case filterAUR:
			aurResults, aurErr = aur.SearchAUR(query)
		default:
			pacmanResults, aurResults, pacmanErr, aurErr = lookup.SearchBoth(query)
		}

		if pacmanErr != nil {
			ui.Warn("official repositories: %v", pacmanErr)
			searchErrs = append(searchErrs, pacmanErr)
		}
		if aurErr != nil {
			ui.Warn("AUR: %v", aurErr)
			searchErrs = append(searchErrs, aurErr)
		}

		entry := searchResultJSON{
			Query:    query,
			Official: []pacmanResultJSON{},
			AUR:      []aurResultJSON{},
		}
		for _, r := range pacmanResults {
			entry.Official = append(entry.Official, pacmanResultJSON{
				Name: r.Name, Version: r.Version, Repo: r.Repository, Description: r.Description,
			})
		}
		for _, r := range aurResults {
			entry.AUR = append(entry.AUR, aurResultJSON{
				Name: r.Name, Version: r.Version, Votes: r.NumVotes,
				Description: r.Description, URL: r.URL, OutOfDate: r.IsOutOfDate(),
			})
		}
		jsonResults = append(jsonResults, entry)

		if jsonFlag {
			continue
		}

		if len(pacmanResults) == 0 && len(aurResults) == 0 {
			if pacmanErr == nil && aurErr == nil {
				ui.Out("No results found for %q in the selected sources.\n", query)
			}
			continue
		}

		if len(pacmanResults) > 0 {
			ui.Outln("\nOfficial repositories")
			for i, r := range pacmanResults {
				if i >= maxOfficialDisplay {
					ui.Out("  ... and %d more (refine the query for specific results)\n", len(pacmanResults)-maxOfficialDisplay)
					break
				}
				item := ui.SelectionItem{
					Index:       nextID,
					Name:        r.Name,
					FullName:    r.Repository + "/" + r.Name,
					SourceLabel: r.Repository,
					Version:     r.Version,
					Description: r.Description,
				}
				items = append(items, item)
				pacmanByID[nextID] = r

				ui.Out("  [%d] %-25s %s\n", nextID, r.Repository+"/"+r.Name, r.Version)
				if r.Description != "" {
					ui.Out("      %s\n", r.Description)
				}
				nextID++
			}
		}

		if len(aurResults) > 0 {
			ui.Outln("\nAUR")
			for i, r := range aurResults {
				if i >= maxAURDisplay {
					ui.Out("  ... and %d more (refine the query for specific results)\n", len(aurResults)-maxAURDisplay)
					break
				}
				item := ui.SelectionItem{
					Index:       nextID,
					Name:        r.Name,
					SourceLabel: "AUR",
					Version:     r.Version,
					Description: r.Description,
				}
				items = append(items, item)
				aurByID[nextID] = r

				ui.Out("  [%d] %-25s %s\n", nextID, r.Name, r.Version)
				if r.Description != "" {
					ui.Out("      %s\n", r.Description)
				}
				ui.Out("      Votes: %d\n", r.NumVotes)
				nextID++
			}
		}
	}

	found := 0
	for _, r := range jsonResults {
		found += len(r.Official) + len(r.AUR)
	}

	if jsonFlag {
		if err := ui.EmitJSON(jsonResults); err != nil {
			return err
		}
		if joined := errors.Join(append(nameErrs, searchErrs...)...); joined != nil {
			return joined
		}
		if found == 0 {
			return &silentExitError{code: 1}
		}
		return nil
	}

	if len(items) == 0 {
		if joined := errors.Join(append(nameErrs, searchErrs...)...); joined != nil {
			return joined
		}
		// Nothing found is a usable result for a human but a distinct status
		// for a script, the way grep reports "no match".
		return &silentExitError{code: 1}
	}

	selected := ui.PromptSelection(reader, "\nEnter item number or package name to view details (or press Enter to skip): ", items)
	if selected != nil {
		if pkg, ok := pacmanByID[selected.Index]; ok {
			displayPacmanDetail(pkg)
		} else if pkg, ok := aurByID[selected.Index]; ok {
			displayAURDetail(pkg)
		}
	}

	return errors.Join(append(nameErrs, searchErrs...)...)
}

func displayPacmanDetail(pkg resolve.PacmanResult) {
	ui.Outln("\n--- Package Details ---")
	ui.Out("Name        : %s\n", pkg.Name)
	ui.Out("Version     : %s\n", pkg.Version)
	ui.Out("Repository  : %s\n", pkg.Repository)
	ui.Out("Description : %s\n", pkg.Description)
	ui.Out("Source      : %s\n", pacmanSourceLabel(pkg.Repository))
	if !conciseFlag {
		ui.Outln("\nRecommended: install via official repositories using pacman.")
	}
}

func displayAURDetail(pkg resolve.AURResult) {
	ui.Outln("\n--- Package Details ---")
	ui.Out("Name        : %s\n", pkg.Name)
	ui.Out("Version     : %s\n", pkg.Version)
	ui.Out("Description : %s\n", pkg.Description)
	ui.Out("URL         : %s\n", pkg.URL)
	ui.Out("Votes       : %d\n", pkg.NumVotes)
	ui.Outln("Source      : AUR")
	if !conciseFlag {
		ui.Outln("\nNote: installing from AUR means Aurora clones the build files and builds them locally with makepkg.")
	}
}

var searchCmd = &cobra.Command{
	Use:   "search <query> [query...]",
	Short: "Search for packages in official repositories and the AUR",
	Long: `Search queries official Arch Linux repositories and the Arch User Repository concurrently.
Use --from to restrict searches to official repositories or the AUR.

Examples:
  aurora search neovim git
  aurora search neovim --from official
  aurora search yay -A
  aurora search neovim --json`,
	RunE:              search,
	ValidArgsFunction: completeAnyPackageName,
}

func init() {
	searchCmd.Flags().StringVar(&searchFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	searchCmd.Flags().BoolVarP(&searchOfficialFlag, "official", "O", false, "search only official repositories")
	searchCmd.Flags().BoolVarP(&searchAURFlag, "aur", "A", false, "search only the Arch User Repository")
	rootCmd.AddCommand(searchCmd)
}
