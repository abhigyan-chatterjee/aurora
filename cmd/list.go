package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/abhigyan-chatterjee/aurora/internal/updates"
	"github.com/spf13/cobra"
)

var (
	listOfficialFlag   bool
	listAURFlag        bool
	listOrphansFlag    bool
	listUpgradableFlag bool
	listAllFlag        bool
)

// listEntryJSON is the --json shape for one listed package.
type listEntryJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Detail  string `json:"detail,omitempty"`
}

type listEntry struct {
	Name    string
	Version string
	Source  string
	Detail  string
}

func listPackages(cmd *cobra.Command, args []string) error {
	var query string
	if len(args) > 0 {
		query = strings.ToLower(strings.TrimSpace(args[0]))
	}

	matches := func(name string) bool {
		return query == "" || strings.Contains(strings.ToLower(name), query)
	}

	var (
		entries  []listEntry
		listErrs []error
	)

	switch {
	case listOrphansFlag:
		orphans, err := pacman.OrphanListing()
		if err != nil {
			ui.Warn("could not list orphan packages: %v", err)
			listErrs = append(listErrs, err)
			break
		}
		for _, o := range orphans {
			if matches(o.Name) {
				entries = append(entries, listEntry{Name: o.Name, Version: o.Version, Source: "orphan"})
			}
		}

	case listUpgradableFlag:
		ui.Outln("Checking for upgradable packages...")
		report := updates.Collect(appConfig.Parallelism)
		for _, w := range report.Warnings {
			ui.Warn("%v", w)
		}
		if !report.CheckupdatesAvailable {
			ui.Warn("checkupdates is not installed (pacman-contrib), so repository upgrades were not checked")
		}
		for _, e := range report.Entries {
			if !matches(e.Name) {
				continue
			}
			entries = append(entries, listEntry{
				Name:    e.Name,
				Version: e.Current,
				Source:  e.SourceLabel(),
				Detail:  fmt.Sprintf("upgradable to %s", e.Candidate),
			})
		}

	case listAURFlag:
		foreign, err := pacman.ForeignListing(listAllFlag)
		if err != nil {
			ui.Warn("could not query foreign packages: %v", err)
			listErrs = append(listErrs, err)
			break
		}
		for name, version := range foreign {
			if matches(name) {
				entries = append(entries, listEntry{Name: name, Version: version, Source: "AUR"})
			}
		}

	case listOfficialFlag:
		repoMap, err := pacman.RepoMap()
		if err != nil {
			ui.Warn("could not read the repository list: %v", err)
			listErrs = append(listErrs, err)
		}
		official, err := pacman.OfficialListing(listAllFlag)
		if err != nil {
			ui.Warn("could not query official packages: %v", err)
			listErrs = append(listErrs, err)
			break
		}
		for name, version := range official {
			if !matches(name) {
				continue
			}
			repo := repoMap[name]
			if repo == "" {
				repo = "official"
			}
			// Only list packages that really come from an Arch repository.
			if !pacman.IsOfficialRepo(repo) {
				continue
			}
			entries = append(entries, listEntry{Name: name, Version: version, Source: repo})
		}

	default:
		repoMap, err := pacman.RepoMap()
		if err != nil {
			ui.Warn("could not read the repository list: %v", err)
			listErrs = append(listErrs, err)
		}
		installed, err := pacman.ExplicitPackages(listAllFlag)
		if err != nil {
			ui.Warn("could not query installed packages: %v", err)
			listErrs = append(listErrs, err)
			break
		}
		for name, version := range installed {
			if !matches(name) {
				continue
			}
			// Attribution comes from the repository actually providing the
			// package. Calling everything outside the AUR "official" mislabels
			// chaotic-aur, endeavouros and garuda packages, which is most of a
			// third-party-repo user's system.
			source := repoMap[name]
			if source == "" {
				source = "AUR"
			}
			entries = append(entries, listEntry{Name: name, Version: version, Source: source})
		}
	}

	if len(entries) == 0 {
		if jsonFlag {
			if err := ui.EmitJSON([]listEntryJSON{}); err != nil {
				return err
			}
			return errors.Join(listErrs...)
		}
		if query != "" {
			ui.Out("No matching installed packages found for %q.\n", query)
		} else {
			ui.Outln("No packages found.")
		}
		return errors.Join(listErrs...)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	if jsonFlag {
		out := make([]listEntryJSON, 0, len(entries))
		for _, e := range entries {
			out = append(out, listEntryJSON(e))
		}
		if err := ui.EmitJSON(out); err != nil {
			return err
		}
		return errors.Join(listErrs...)
	}

	ui.Out("\nListing %d package(s):\n", len(entries))
	for _, e := range entries {
		extra := ""
		if e.Detail != "" {
			extra = fmt.Sprintf(" (%s)", e.Detail)
		}
		ui.Out("  %-30s %-18s [%s]%s\n", e.Name, e.Version, e.Source, extra)
	}
	return errors.Join(listErrs...)
}

var listCmd = &cobra.Command{
	Use:   "list [filter]",
	Short: "List installed packages, updates, or orphans (apt-style query)",
	Long: `List displays installed packages with their versions and the repository that
provides them. Third-party repositories are named as themselves rather than being
called official.

Examples:
  aurora list                 # packages you explicitly installed
  aurora list --all           # every installed package, including dependencies
  aurora list python          # installed packages whose name contains 'python'
  aurora list --upgradable    # packages with an available upgrade
  aurora list --foreign       # installed AUR packages
  aurora list --orphans       # unneeded orphan dependencies
  aurora list --json          # machine-readable output`,
	RunE: listPackages,
}

func init() {
	listCmd.Flags().BoolVarP(&listOfficialFlag, "official", "o", false, "list only packages from official Arch repositories")
	listCmd.Flags().BoolVarP(&listAURFlag, "foreign", "m", false, "list only foreign / AUR packages (alias: --aur)")
	// --aur is an alias for --foreign, registered on the same variable so the
	// two flags cannot disagree. Registering two BoolVars on one variable let
	// `--aur=false` clobber an earlier `--foreign`.
	listCmd.Flags().BoolVar(&listAURFlag, "aur", false, "alias for --foreign")
	listCmd.Flags().Lookup("aur").Hidden = true
	listCmd.Flags().BoolVar(&listOrphansFlag, "orphans", false, "list unneeded orphan dependencies")
	listCmd.Flags().BoolVarP(&listUpgradableFlag, "upgradable", "u", false, "list packages with available upgrades")
	listCmd.Flags().BoolVarP(&listAllFlag, "all", "a", false, "list every installed package, including dependencies")
	rootCmd.AddCommand(listCmd)
}
