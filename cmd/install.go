package cmd

import (
	"bufio"
	"errors"
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
	installFromFlag     string
	installOfficialFlag bool
	installAURFlag      bool
	installInspectFlag  bool
)

type installPlanItem struct {
	Name         string
	Source       resolve.PackageSource
	PacmanResult *resolve.PacmanResult
	AURResult    *resolve.AURResult
}

func install(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Println("Please provide a package name to install.")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	sourceFilter := strings.ToLower(strings.TrimSpace(installFromFlag))
	if installOfficialFlag {
		sourceFilter = "official"
	} else if installAURFlag {
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

	var officialPlan []installPlanItem
	var aurPlan []installPlanItem
	var resolveErrs []error

	for _, res := range resolvedList {
		if res.Err != nil {
			fmt.Printf("Warning: could not check sources for \"%s\": %v; skipping.\n", res.Query, res.Err)
			resolveErrs = append(resolveErrs, fmt.Errorf("%s: %w", res.Query, res.Err))
			continue
		}
		if res.ChosenSource == resolve.SourceOfficial && res.PacmanResult != nil {
			officialPlan = append(officialPlan, installPlanItem{
				Name:         res.PacmanResult.Name,
				Source:       resolve.SourceOfficial,
				PacmanResult: res.PacmanResult,
			})
		} else if res.ChosenSource == resolve.SourceAUR && res.AURResult != nil {
			aurPlan = append(aurPlan, installPlanItem{
				Name:      res.AURResult.Name,
				Source:    resolve.SourceAUR,
				AURResult: res.AURResult,
			})
		} else {
			// Package not found by exact match
			if len(validArgs) == 1 {
				// Offer fuzzy search / Did you mean suggestions for single-package installs
				var pacmanResults []resolve.PacmanResult
				var aurResults []resolve.AURResult
				var pacmanErr, aurErr error

				if sourceFilter == "official" || sourceFilter == "repo" {
					pacmanResults, pacmanErr = pacman.SearchPacman(res.Query)
				} else if sourceFilter == "aur" {
					aurResults, aurErr = aur.SearchAUR(res.Query)
				} else {
					pacmanResults, aurResults, pacmanErr, aurErr = lookup.SearchBoth(res.Query)
				}

				if pacmanErr != nil {
					fmt.Printf("Warning (official repos): %v\n", pacmanErr)
				}
				if aurErr != nil {
					fmt.Printf("Warning (AUR): %v\n", aurErr)
				}

				var suggestions []ui.SelectionItem
				idxCounter := 1

				pacmanMap := make(map[int]resolve.PacmanResult)
				aurMap := make(map[int]resolve.AURResult)

				if len(pacmanResults) > 0 {
					for i, r := range pacmanResults {
						if i >= 3 {
							break
						}
						suggestions = append(suggestions, ui.SelectionItem{
							Index:       idxCounter,
							Name:        r.Name,
							SourceLabel: "official: " + r.Repository,
							Version:     r.Version,
							Description: r.Description,
						})
						pacmanMap[idxCounter] = r
						idxCounter++
					}
				}

				if len(aurResults) > 0 {
					for i, r := range aurResults {
						if i >= 3 {
							break
						}
						suggestions = append(suggestions, ui.SelectionItem{
							Index:       idxCounter,
							Name:        r.Name,
							SourceLabel: "AUR",
							Version:     r.Version,
							Description: r.Description,
						})
						aurMap[idxCounter] = r
						idxCounter++
					}
				}

				if len(suggestions) == 0 {
					fmt.Printf("\n\"%s\" was not found in specified sources.\n", res.Query)
					return fmt.Errorf("package %q not found in specified sources", res.Query)
				}

				fmt.Printf("\n\"%s\" was not found. Did you mean:\n", res.Query)
				for _, item := range suggestions {
					fmt.Printf("  [%d] %s (%s) - %s\n", item.Index, item.Name, item.SourceLabel, item.Version)
				}

				_, selected := ui.PromptSelection(reader, "\nEnter number or package name to install (or press Enter to skip): ", suggestions)
				if selected == nil {
					fmt.Println("Install cancelled.")
					return nil
				}

				if pPkg, ok := pacmanMap[selected.Index]; ok {
					pPkgCopy := pPkg
					officialPlan = append(officialPlan, installPlanItem{
						Name:         pPkgCopy.Name,
						Source:       resolve.SourceOfficial,
						PacmanResult: &pPkgCopy,
					})
				} else if aPkg, ok := aurMap[selected.Index]; ok {
					aPkgCopy := aPkg
					aurPlan = append(aurPlan, installPlanItem{
						Name:      aPkgCopy.Name,
						Source:    resolve.SourceAUR,
						AURResult: &aPkgCopy,
					})
				}
			} else {
				fmt.Printf("Warning: package \"%s\" was not found in specified sources; skipping.\n", res.Query)
				resolveErrs = append(resolveErrs, fmt.Errorf("package %q not found", res.Query))
			}
		}
	}

	plan := append(officialPlan, aurPlan...)

	if len(plan) == 0 {
		if len(resolveErrs) > 0 {
			return errors.Join(resolveErrs...)
		}
		return fmt.Errorf("no packages to install")
	}

	fmt.Println("\nAurora is about to install:")
	for _, item := range plan {
		if item.Source == resolve.SourceOfficial {
			fmt.Printf("  - %s (official: %s)\n", item.Name, item.PacmanResult.Repository)
		} else {
			votes := 0
			if item.AURResult != nil {
				votes = item.AURResult.NumVotes
			}
			fmt.Printf("  - %s (AUR, %d votes)\n", item.Name, votes)
		}
	}

	if !ui.ConfirmAction(reader, "\nProceed with installation?", yesFlag) {
		fmt.Println("Install cancelled.")
		return nil
	}

	var installErrs []error
	if len(officialPlan) > 0 {
		var officialNames []string
		for _, item := range officialPlan {
			officialNames = append(officialNames, item.Name)
		}
		err := pacman.InstallPacmanBatch(officialNames)
		if err != nil {
			fmt.Printf("Error installing official packages: %v\n", err)
			installErrs = append(installErrs, err)
		}
	}

	for _, item := range aurPlan {
		// PKGBUILD inspection support (Paru-style power)
		if installInspectFlag {
			fmt.Printf("\n--- Inspecting PKGBUILD for %s ---\n", item.Name)
			content, err := aur.FetchPKGBUILD(item.Name)
			if err == nil {
				lines := strings.Split(content, "\n")
				for i, l := range lines {
					fmt.Printf("%4d | %s\n", i+1, l)
				}
				if !ui.ConfirmAction(reader, fmt.Sprintf("Continue building %s after PKGBUILD review?", item.Name), yesFlag) {
					fmt.Printf("Skipped building %s.\n", item.Name)
					continue
				}
			} else {
				fmt.Printf("Warning: could not fetch PKGBUILD for review: %v\n", err)
			}
		}

		fmt.Printf("\nInstalling %s from AUR...\n", item.Name)
		err := aur.InstallAUR(item.Name)
		if err != nil {
			fmt.Printf("Error installing %s from AUR: %v\n", item.Name, err)
			installErrs = append(installErrs, fmt.Errorf("%s: %w", item.Name, err))
		}
	}

	return errors.Join(append(resolveErrs, installErrs...)...)
}

var installCmd = &cobra.Command{
	Use:   "install <package> [package...]",
	Short: "Installs specified packages from official repositories or AUR",
	Long: `Installs packages from official repositories (preferred) or the Arch User Repository.
Supports batch installation, automatic Did-You-Mean suggestions, source filtering, and PKGBUILD inspection.

Examples:
  aurora install neovim git ripgrep
  aurora install yay --from aur
  aurora install paru --inspect`,
	RunE: install,
}

func init() {
	installCmd.Flags().StringVar(&installFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	installCmd.Flags().BoolVarP(&installOfficialFlag, "official", "O", false, "install only from official repositories")
	installCmd.Flags().BoolVarP(&installAURFlag, "aur", "A", false, "install only from the Arch User Repository")
	installCmd.Flags().BoolVarP(&installInspectFlag, "inspect", "i", false, "review PKGBUILD script before building AUR packages")
	rootCmd.AddCommand(installCmd)
}
