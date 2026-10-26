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
	installDiffFlag     bool
	installNoDepsFlag   bool
	installDryRunFlag   bool
)

type installPlanItem struct {
	Name    string
	Source  resolve.PackageSource
	Repo    string
	IsDep   bool
	Version string
}

func install(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a package name to install")
	}

	reader := bufio.NewReader(os.Stdin)

	filter, err := parseSourceFilter(installFromFlag, installOfficialFlag, installAURFlag)
	if err != nil {
		return err
	}

	validArgs, nameErrs := collectValidArgs(args)
	if len(validArgs) == 0 {
		return errors.Join(nameErrs...)
	}

	resolvedList := resolveForInstall(validArgs, filter)

	officialPlan, aurPlan, resolveErrs := buildInstallPlan(resolvedList, reader, filter)

	if len(officialPlan) == 0 && len(aurPlan) == 0 {
		allErrs := append(nameErrs, resolveErrs...)
		if len(allErrs) > 0 {
			return errors.Join(allErrs...)
		}
		return fmt.Errorf("no packages to install")
	}

	// Resolve the AUR build order before printing the plan, so dependencies
	// appear in the list the user approves rather than appearing mid-build.
	if len(aurPlan) > 0 && !installNoDepsFlag && appConfig.AURDeps {
		aurPlan = orderAURInstallPlan(aurPlan)
	}

	printInstallPlan(officialPlan, aurPlan)

	if installDryRunFlag {
		ui.Outln("\nDry run: nothing was installed.")
		return errors.Join(append(nameErrs, resolveErrs...)...)
	}

	if !ui.ConfirmAction(reader, "\nProceed with installation?", yesFlag) {
		ui.Outln("Install cancelled.")
		return nil
	}

	var installErrs []error

	if len(officialPlan) > 0 {
		names := make([]string, 0, len(officialPlan))
		for _, item := range officialPlan {
			names = append(names, item.Name)
		}
		ui.Out("\nInstalling from official repositories: %s\n", strings.Join(names, ", "))
		if err := pacman.InstallPacman(names, appConfig.InteractivePacman); err != nil {
			installErrs = append(installErrs, fmt.Errorf("installing official packages: %w", err))
		}
	}

	for _, item := range aurPlan {
		err := aur.InstallAUR(item.Name, aur.BuildOptions{
			Diff:   installDiffFlag || appConfig.PKGBUILDDiff,
			Review: installReviewer(item.Name, reader),
		})
		switch {
		case err == nil:
		case errors.Is(err, aur.ErrReviewDeclined):
			ui.Out("Skipped building %s.\n", item.Name)
		default:
			installErrs = append(installErrs, fmt.Errorf("installing %s from AUR: %w", item.Name, err))
		}
	}

	return errors.Join(append(nameErrs, append(resolveErrs, installErrs...)...)...)
}

// installReviewer returns the PKGBUILD review callback for --inspect, or nil
// when no review was requested.
//
// The review happens after the clone and before makepkg. A clone failure aborts
// that package, so a broken fetch can never fall through to an unreviewed
// build, which is what used to happen when inspection failed.
func installReviewer(name string, reader *bufio.Reader) func(string) error {
	if !installInspectFlag {
		return nil
	}
	return func(pkgbuild string) error {
		ui.Out("\n--- PKGBUILD for %s ---\n", name)
		for i, line := range strings.Split(pkgbuild, "\n") {
			ui.Out("%4d | %s\n", i+1, line)
		}
		if !ui.ConfirmAction(reader, fmt.Sprintf("Continue building %s after PKGBUILD review?", name), yesFlag) {
			return aur.ErrReviewDeclined
		}
		return nil
	}
}

// orderAURInstallPlan resolves dependencies and returns the plan in build order,
// with dependencies that were not explicitly requested appended as IsDep items.
func orderAURInstallPlan(plan []installPlanItem) []installPlanItem {
	requested := make([]string, 0, len(plan))
	byName := make(map[string]installPlanItem, len(plan))
	for _, item := range plan {
		requested = append(requested, item.Name)
		byName[item.Name] = item
	}

	repoMap, err := pacman.RepoMap()
	if err != nil {
		ui.Warn("could not read the repository list, installing without dependency ordering: %v", err)
		return plan
	}
	isOfficial := func(name string) bool {
		_, exists := repoMap[name]
		return exists
	}

	order, warnings := aur.ResolveBuildOrder(requested, isOfficial, appConfig.Parallelism)
	for _, w := range warnings {
		// Dependency-resolution notes are context, not problems: most are
		// virtual provides that pacman settles itself.
		ui.Out("Note: %v\n", w)
	}
	if len(order) == 0 {
		return plan
	}

	ordered := make([]installPlanItem, 0, len(order))
	for _, name := range order {
		// byName is keyed by the requested name, so a hit is the real plan item
		// and a miss is a dependency pulled in by the resolver.
		if item, ok := byName[name]; ok {
			ordered = append(ordered, item)
			continue
		}
		ordered = append(ordered, installPlanItem{
			Name:   name,
			Source: resolve.SourceAUR,
			IsDep:  true,
		})
	}
	return ordered
}

// resolveForInstall resolves each argument against the requested sources.
func resolveForInstall(validArgs []string, filter sourceFilter) []*resolve.ResolvedPackage {
	switch filter {
	case filterOfficial:
		out := make([]*resolve.ResolvedPackage, 0, len(validArgs))
		for _, q := range validArgs {
			pkg, err := pacman.SearchPacmanExact(q)
			res := &resolve.ResolvedPackage{Query: q, ChosenSource: resolve.SourceUnknown}
			switch {
			case err != nil:
				res.Err = err
			case pkg != nil && !pacman.IsOfficialRepo(pkg.Repository):
				res.Warnings = append(res.Warnings, fmt.Errorf(
					"%q is provided by %s, which is not an official Arch repository", pkg.Name, pkg.Repository))
			case pkg != nil:
				res.PacmanResult = pkg
				res.ChosenSource = resolve.SourceOfficial
			}
			out = append(out, res)
		}
		return out

	case filterAUR:
		out := make([]*resolve.ResolvedPackage, 0, len(validArgs))
		for _, q := range validArgs {
			pkg, err := aur.SearchAURExact(q)
			res := &resolve.ResolvedPackage{Query: q, ChosenSource: resolve.SourceUnknown}
			if err != nil {
				res.Err = err
			} else if pkg != nil {
				res.AURResult = pkg
				res.ChosenSource = resolve.SourceAUR
			}
			out = append(out, res)
		}
		return out

	default:
		return lookup.ResolveBatch(validArgs)
	}
}

// buildInstallPlan turns resolved packages into official and AUR plans,
// offering suggestions for a single package that was not found.
func buildInstallPlan(resolvedList []*resolve.ResolvedPackage, reader *bufio.Reader, filter sourceFilter) (official, aurPlan []installPlanItem, errs []error) {
	for _, res := range resolvedList {
		warnResolution(res.Query, res.Warnings)

		if res.Err != nil {
			ui.Warn("could not check sources for %q; skipping: %v", res.Query, res.Err)
			errs = append(errs, fmt.Errorf("%s: %w", res.Query, res.Err))
			continue
		}

		switch {
		case res.ChosenSource == resolve.SourceOfficial && res.PacmanResult != nil:
			official = append(official, installPlanItem{
				Name:    res.PacmanResult.Name,
				Source:  resolve.SourceOfficial,
				Repo:    res.PacmanResult.Repository,
				Version: res.PacmanResult.Version,
			})
		case res.ChosenSource == resolve.SourceAUR && res.AURResult != nil:
			aurPlan = append(aurPlan, installPlanItem{
				Name:    res.AURResult.Name,
				Source:  resolve.SourceAUR,
				Version: res.AURResult.Version,
			})
		default:
			// Not found by exact name. Offer suggestions only for a single
			// package; in a batch, silently skip and report at the end.
			if len(resolvedList) == 1 {
				if item, ok := promptSuggestion(res, reader, filter); ok {
					if item.Source == resolve.SourceOfficial {
						official = append(official, item)
					} else {
						aurPlan = append(aurPlan, item)
					}
					continue
				}
				return official, aurPlan, errs
			}
			ui.Warn("package %q was not found in the selected sources; skipping", res.Query)
			errs = append(errs, fmt.Errorf("package %q not found", res.Query))
		}
	}
	return official, aurPlan, errs
}

// promptSuggestion offers the closest matches for a single missing package.
func promptSuggestion(res *resolve.ResolvedPackage, reader *bufio.Reader, filter sourceFilter) (installPlanItem, bool) {
	var (
		pacmanResults []resolve.PacmanResult
		aurResults    []resolve.AURResult
		pacmanErr     error
		aurErr        error
	)

	switch filter {
	case filterOfficial:
		pacmanResults, pacmanErr = pacman.SearchPacman(res.Query)
	case filterAUR:
		aurResults, aurErr = aur.SearchAUR(res.Query)
	default:
		pacmanResults, aurResults, pacmanErr, aurErr = lookup.SearchBoth(res.Query)
	}
	if pacmanErr != nil {
		ui.Warn("official repositories: %v", pacmanErr)
	}
	if aurErr != nil {
		ui.Warn("AUR: %v", aurErr)
	}

	const maxSuggestions = 3
	var (
		suggestions []ui.SelectionItem
		items       = make(map[int]installPlanItem)
		index       = 1
	)

	for i, r := range pacmanResults {
		if i >= maxSuggestions {
			break
		}
		suggestions = append(suggestions, ui.SelectionItem{
			Index:       index,
			Name:        r.Name,
			SourceLabel: r.Repository,
			Version:     r.Version,
			Description: r.Description,
		})
		items[index] = installPlanItem{Name: r.Name, Source: resolve.SourceOfficial, Repo: r.Repository, Version: r.Version}
		index++
	}
	for i, r := range aurResults {
		if i >= maxSuggestions {
			break
		}
		suggestions = append(suggestions, ui.SelectionItem{
			Index:       index,
			Name:        r.Name,
			SourceLabel: "AUR",
			Version:     r.Version,
			Description: r.Description,
		})
		items[index] = installPlanItem{Name: r.Name, Source: resolve.SourceAUR, Version: r.Version}
		index++
	}

	if len(suggestions) == 0 {
		ui.Out("\n%q was not found in the selected sources.\n", res.Query)
		return installPlanItem{}, false
	}

	ui.Out("\n%q was not found. Did you mean:\n", res.Query)
	for _, item := range suggestions {
		ui.Out("  [%d] %s (%s) - %s\n", item.Index, item.Name, item.SourceLabel, item.Version)
	}

	selected := ui.PromptSelection(reader, "\nEnter number or package name to install (or press Enter to skip): ", suggestions)
	if selected == nil {
		return installPlanItem{}, false
	}
	item, ok := items[selected.Index]
	return item, ok
}

func printInstallPlan(official, aurPlan []installPlanItem) {
	ui.Preview("\nAurora is about to install:\n")
	for _, item := range official {
		repo := item.Repo
		if repo == "" {
			repo = "official"
		}
		ui.Preview("  - %s (%s)\n", item.Name, repo)
	}
	for _, item := range aurPlan {
		note := "AUR"
		if item.IsDep {
			note = "AUR dependency"
		}
		ui.Preview("  - %s (%s)\n", item.Name, note)
	}
}

var installCmd = &cobra.Command{
	Use:   "install <package> [package...]",
	Short: "Installs specified packages from official repositories or AUR",
	Long: `Installs packages from official repositories (preferred) or the Arch User Repository.

Supports batch installation, automatic Did-You-Mean suggestions, source filtering,
PKGBUILD review, and AUR dependency ordering.

Examples:
  aurora install neovim git ripgrep
  aurora install yay --from aur
  aurora install paru --inspect
  aurora install paru --dry-run     # show the plan without installing`,
	RunE:              install,
	ValidArgsFunction: completeAnyPackageName,
}

func init() {
	installCmd.Flags().StringVar(&installFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	installCmd.Flags().BoolVarP(&installOfficialFlag, "official", "O", false, "install only from official repositories")
	installCmd.Flags().BoolVarP(&installAURFlag, "aur", "A", false, "install only from the Arch User Repository")
	installCmd.Flags().BoolVarP(&installInspectFlag, "inspect", "i", false, "review the PKGBUILD before building AUR packages")
	installCmd.Flags().BoolVar(&installDiffFlag, "diff", false, "show PKGBUILD changes since the last build of each AUR package")
	installCmd.Flags().BoolVar(&installNoDepsFlag, "no-deps", false, "do not resolve or reorder AUR dependencies")
	installCmd.Flags().BoolVar(&installDryRunFlag, "dry-run", false, "show what would be installed and exit")
	rootCmd.AddCommand(installCmd)
}
