package cmd

import (
	"errors"
	"fmt"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"

	"github.com/spf13/cobra"
)

var (
	infoFromFlag     string
	infoOfficialFlag bool
	infoAURFlag      bool
)

// packageInfoJSON is the --json shape for `aurora info`.
type packageInfoJSON struct {
	Query       string   `json:"query"`
	Found       bool     `json:"found"`
	Name        string   `json:"name,omitempty"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	URL         string   `json:"url,omitempty"`
	Votes       int      `json:"votes,omitempty"`
	OutOfDate   bool     `json:"out_of_date,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

func info(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a package name")
	}

	filter, err := parseSourceFilter(infoFromFlag, infoOfficialFlag, infoAURFlag)
	if err != nil {
		return err
	}

	validArgs, nameErrs := collectValidArgs(args)
	if len(validArgs) == 0 {
		return errors.Join(nameErrs...)
	}

	resolvedList := resolveForInstall(validArgs, filter)

	var (
		errs    []error
		jsonOut []packageInfoJSON
	)

	for _, resolved := range resolvedList {
		entry := packageInfoJSON{Query: resolved.Query}
		for _, w := range resolved.Warnings {
			entry.Warnings = append(entry.Warnings, w.Error())
			ui.Warn("could not fully check sources for %q: %v", resolved.Query, w)
		}

		if resolved.Err != nil {
			ui.Warn("could not check sources for %q: %v", resolved.Query, resolved.Err)
			errs = append(errs, fmt.Errorf("%s: %w", resolved.Query, resolved.Err))
			jsonOut = append(jsonOut, entry)
			continue
		}

		switch {
		case resolved.ChosenSource == resolve.SourceOfficial && resolved.PacmanResult != nil:
			pkg := resolved.PacmanResult
			entry.Found = true
			entry.Name = pkg.Name
			entry.Version = pkg.Version
			entry.Description = pkg.Description
			entry.Source = "official"
			entry.Repo = pkg.Repository

			if !jsonFlag {
				ui.Out("\n--- Package Details: %s ---\n", pkg.Name)
				ui.Out("Name        : %s\n", pkg.Name)
				ui.Out("Version     : %s\n", pkg.Version)
				ui.Out("Repository  : %s\n", pkg.Repository)
				ui.Out("Description : %s\n", pkg.Description)
				ui.Out("Source      : %s\n", pacmanSourceLabel(pkg.Repository))
			}

		case resolved.ChosenSource == resolve.SourceAUR && resolved.AURResult != nil:
			pkg := resolved.AURResult
			entry.Found = true
			entry.Name = pkg.Name
			entry.Version = pkg.Version
			entry.Description = pkg.Description
			entry.Source = "AUR"
			entry.URL = pkg.URL
			entry.Votes = pkg.NumVotes
			entry.OutOfDate = pkg.IsOutOfDate()

			if !jsonFlag {
				ui.Out("\n--- Package Details: %s ---\n", pkg.Name)
				ui.Out("Name        : %s\n", pkg.Name)
				ui.Out("Version     : %s\n", pkg.Version)
				ui.Out("Description : %s\n", pkg.Description)
				ui.Out("URL         : %s\n", pkg.URL)
				ui.Out("Votes       : %d\n", pkg.NumVotes)
				if pkg.IsOutOfDate() {
					ui.Outln("Status      : flagged out of date by the AUR maintainer")
				}
				ui.Outln("Source      : AUR")
				if !conciseFlag {
					ui.Outln("\nNote: installing from AUR means Aurora clones the build files and builds them locally with makepkg.")
				}
			}

		default:
			if !jsonFlag {
				ui.Out("\nPackage %q was not found in the selected sources.\n", resolved.Query)
			}
			errs = append(errs, fmt.Errorf("package %q not found", resolved.Query))
		}

		jsonOut = append(jsonOut, entry)
	}

	if jsonFlag {
		if err := ui.EmitJSON(jsonOut); err != nil {
			return err
		}
	}

	return errors.Join(append(nameErrs, errs...)...)
}

var infoCmd = &cobra.Command{
	Use:   "info <package> [package...]",
	Short: "Show detailed information about a package",
	Long: `Display detailed information about packages from official repositories or the AUR.
Use --from to restrict lookups to official repositories or the AUR.

Examples:
  aurora info neovim
  aurora info yay --from aur
  aurora info neovim --json`,
	RunE:              info,
	ValidArgsFunction: completePackageNames,
}

func init() {
	infoCmd.Flags().StringVar(&infoFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	infoCmd.Flags().BoolVarP(&infoOfficialFlag, "official", "O", false, "inspect only official repositories")
	infoCmd.Flags().BoolVarP(&infoAURFlag, "aur", "A", false, "inspect only the Arch User Repository")
	rootCmd.AddCommand(infoCmd)
}
