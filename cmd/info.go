package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/lookup"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"

	"github.com/spf13/cobra"
)

var (
	infoFromFlag     string
	infoOfficialFlag bool
	infoAURFlag      bool
)

func info(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Println("Please provide a package name.")
		return nil
	}

	sourceFilter := strings.ToLower(strings.TrimSpace(infoFromFlag))
	if infoOfficialFlag {
		sourceFilter = "official"
	} else if infoAURFlag {
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
		return fmt.Errorf("no valid package names provided")
	}

	var resolvedList []*resolve.ResolvedPackage

	if sourceFilter == "official" || sourceFilter == "repo" {
		for _, q := range validArgs {
			pPkg, err := pacman.SearchPacmanExact(q)
			res := &resolve.ResolvedPackage{Query: q, ChosenSource: resolve.SourceUnknown}
			if err != nil {
				res.Err = err
			} else if pPkg != nil {
				res.PacmanResult = pPkg
				res.ChosenSource = resolve.SourceOfficial
			}
			resolvedList = append(resolvedList, res)
		}
	} else if sourceFilter == "aur" {
		for _, q := range validArgs {
			aPkg, err := aur.SearchAURExact(q)
			res := &resolve.ResolvedPackage{Query: q, ChosenSource: resolve.SourceUnknown}
			if err != nil {
				res.Err = err
			} else if aPkg != nil {
				res.AURResult = aPkg
				res.ChosenSource = resolve.SourceAUR
			}
			resolvedList = append(resolvedList, res)
		}
	} else {
		resolvedList = lookup.ResolveBatch(validArgs)
	}

	var errs []error
	for _, resolved := range resolvedList {
		if resolved.Err != nil {
			fmt.Printf("\nWarning: could not check sources for \"%s\": %v\n", resolved.Query, resolved.Err)
			errs = append(errs, fmt.Errorf("%s: %w", resolved.Query, resolved.Err))
			continue
		}
		if resolved.ChosenSource == resolve.SourceOfficial && resolved.PacmanResult != nil {
			pkg := resolved.PacmanResult
			fmt.Printf("\n--- Package Details: %s ---\n", pkg.Name)
			fmt.Printf("Name        : %s\n", pkg.Name)
			fmt.Printf("Version     : %s\n", pkg.Version)
			fmt.Printf("Repository  : %s\n", pkg.Repository)
			fmt.Printf("Description : %s\n", pkg.Description)
			fmt.Printf("Source      : official repositories\n")
		} else if resolved.ChosenSource == resolve.SourceAUR && resolved.AURResult != nil {
			pkg := resolved.AURResult
			fmt.Printf("\n--- Package Details: %s ---\n", pkg.Name)
			fmt.Printf("Name        : %s\n", pkg.Name)
			fmt.Printf("Version     : %s\n", pkg.Version)
			fmt.Printf("Description : %s\n", pkg.Description)
			fmt.Printf("URL         : %s\n", pkg.URL)
			fmt.Printf("Votes       : %d\n", pkg.NumVotes)
			fmt.Printf("Source      : AUR\n")
			if !conciseFlag {
				fmt.Println("\nNote: installing from AUR means Aurora will clone the build files and build locally using makepkg.")
			}
		} else {
			fmt.Printf("\nPackage \"%s\" was not found in specified sources.\n", resolved.Query)
			errs = append(errs, fmt.Errorf("package %q not found", resolved.Query))
		}
	}
	return errors.Join(errs...)
}

var infoCmd = &cobra.Command{
	Use:   "info <package> [package...]",
	Short: "Show detailed information about a package",
	Long: `Display detailed information about packages from official repositories or the AUR.
Use --from to restrict information lookups to official repositories or AUR.

Examples:
  aurora info neovim
  aurora info yay --from aur`,
	RunE: info,
}

func init() {
	infoCmd.Flags().StringVar(&infoFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	infoCmd.Flags().BoolVarP(&infoOfficialFlag, "official", "O", false, "inspect only official repositories")
	infoCmd.Flags().BoolVarP(&infoAURFlag, "aur", "A", false, "inspect only the Arch User Repository")
	rootCmd.AddCommand(infoCmd)
}
