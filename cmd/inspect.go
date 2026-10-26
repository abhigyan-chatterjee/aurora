package cmd

import (
	"fmt"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/spf13/cobra"
)

var (
	inspectRawFlag      bool
	inspectFromFlag     string
	inspectOfficialFlag bool
	inspectAURFlag      bool
)

// inspectJSON is the --json shape for `aurora inspect`.
type inspectJSON struct {
	Name         string   `json:"name"`
	Packages     []string `json:"packages,omitempty"`
	Source       string   `json:"source"`
	Repo         string   `json:"repo,omitempty"`
	Version      string   `json:"version,omitempty"`
	Release      string   `json:"release,omitempty"`
	Description  string   `json:"description,omitempty"`
	URL          string   `json:"url,omitempty"`
	Maintainer   string   `json:"maintainer,omitempty"`
	Votes        int      `json:"votes,omitempty"`
	OutOfDate    bool     `json:"out_of_date,omitempty"`
	Depends      []string `json:"depends,omitempty"`
	MakeDepends  []string `json:"make_depends,omitempty"`
	CheckDepends []string `json:"check_depends,omitempty"`
	OptDepends   []string `json:"opt_depends,omitempty"`
	Sources      []string `json:"sources,omitempty"`
	PKGBUILD     string   `json:"pkgbuild,omitempty"`
}

func inspect(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a package name to inspect")
	}

	pkgName := args[0]
	if !ui.IsValidPkgName(pkgName) {
		return fmt.Errorf("invalid package name: %q", pkgName)
	}

	filter, err := parseSourceFilter(inspectFromFlag, inspectOfficialFlag, inspectAURFlag)
	if err != nil {
		return err
	}

	if filter != filterOfficial {
		if found, err := inspectAUR(pkgName); err != nil {
			return err
		} else if found {
			return nil
		}
	}

	if filter == filterAUR {
		return fmt.Errorf("package %q was not found in the AUR", pkgName)
	}

	if found, err := inspectOfficial(pkgName); err != nil {
		return err
	} else if found {
		return nil
	}

	return fmt.Errorf("package %q was not found in the selected sources", pkgName)
}

// inspectAUR fetches and displays the AUR PKGBUILD. It reports whether the
// package was found so the caller can fall through to the official lookup.
func inspectAUR(pkgName string) (bool, error) {
	content, err := aur.FetchPKGBUILD(pkgName)
	if err != nil {
		return false, nil
	}

	parsed := aur.ParsePKGBUILD(content)
	votes := 0
	outOfDate := false
	if exact, err := aur.SearchAURExact(pkgName); err == nil && exact != nil {
		votes = exact.NumVotes
		outOfDate = exact.IsOutOfDate()
	}

	if jsonFlag {
		var splitNames []string
		if len(parsed.PackageNames) > 1 {
			splitNames = parsed.PackageNames
		}
		return true, ui.EmitJSON(inspectJSON{
			Name:         pkgName,
			Packages:     splitNames,
			Source:       "AUR",
			Version:      parsed.Version,
			Release:      parsed.Release,
			Description:  parsed.Description,
			URL:          parsed.URL,
			Maintainer:   parsed.Maintainer,
			Votes:        votes,
			OutOfDate:    outOfDate,
			Depends:      parsed.Depends,
			MakeDepends:  parsed.MakeDepends,
			CheckDepends: parsed.CheckDepends,
			OptDepends:   parsed.OptDepends,
			Sources:      parsed.Sources,
			PKGBUILD:     content,
		})
	}

	if inspectRawFlag {
		ui.Out("%s", content)
		return true, nil
	}

	ui.Out("\n=== AUR Package Inspection: %s ===\n", pkgName)
	if len(parsed.PackageNames) > 1 {
		ui.Out("Packages    : %s\n", strings.Join(parsed.PackageNames, ", "))
	}
	if parsed.Version != "" {
		version := parsed.Version
		if parsed.Release != "" {
			version += "-" + parsed.Release
		}
		ui.Out("Version     : %s\n", version)
	}
	if parsed.Description != "" {
		ui.Out("Description : %s\n", parsed.Description)
	}
	if parsed.URL != "" {
		ui.Out("Upstream URL: %s\n", parsed.URL)
	}
	if parsed.Maintainer != "" {
		ui.Out("Maintainer  : %s\n", parsed.Maintainer)
	}
	ui.Out("AUR Votes   : %d\n", votes)
	if outOfDate {
		ui.Outln("Status      : flagged out of date by the AUR maintainer")
	}
	if votes < 2 {
		ui.Outln("Security Tip: this package has low community votes. Verify its sources before building.")
	}
	if len(parsed.Depends) > 0 {
		ui.Out("Depends On  : %s\n", strings.Join(parsed.Depends, ", "))
	}
	if len(parsed.MakeDepends) > 0 {
		ui.Out("Make Depends: %s\n", strings.Join(parsed.MakeDepends, ", "))
	}
	if len(parsed.CheckDepends) > 0 {
		ui.Out("Check Depends: %s\n", strings.Join(parsed.CheckDepends, ", "))
	}
	if len(parsed.OptDepends) > 0 {
		ui.Out("Optional    : %s\n", strings.Join(parsed.OptDepends, ", "))
	}
	if len(parsed.Sources) > 0 {
		ui.Outln("Source URLs :")
		for _, src := range parsed.Sources {
			ui.Out("  - %s\n", src)
		}
	}

	ui.Out("\n--- PKGBUILD Script (%s) ---\n", pkgName)
	for i, line := range strings.Split(content, "\n") {
		ui.Out("%4d | %s\n", i+1, line)
	}
	return true, nil
}

// inspectOfficial displays sync database metadata plus pacman's full record.
func inspectOfficial(pkgName string) (bool, error) {
	pkg, err := pacman.SearchPacmanExact(pkgName)
	if err != nil {
		return false, fmt.Errorf("error querying pacman: %w", err)
	}
	if pkg == nil {
		return false, nil
	}

	raw, _ := pacman.RawPackageInfo(pkgName)

	if jsonFlag {
		return true, ui.EmitJSON(inspectJSON{
			Name:        pkg.Name,
			Source:      "official",
			Repo:        pkg.Repository,
			Version:     pkg.Version,
			Description: pkg.Description,
			PKGBUILD:    "",
		})
	}

	ui.Out("\n=== Official Repository Inspection: %s ===\n", pkg.Name)
	ui.Out("Repository  : %s\n", pkg.Repository)
	ui.Out("Version     : %s\n", pkg.Version)
	ui.Out("Description : %s\n", pkg.Description)
	ui.Outln("Source      : Official Arch Linux binary repository")

	if strings.TrimSpace(raw) != "" {
		ui.Outln("\nPackage diagnostics:")
		ui.Outln(strings.TrimSpace(raw))
	}
	return true, nil
}

var inspectCmd = &cobra.Command{
	Use:     "inspect <package>",
	Aliases: []string{"pkgbuild", "cat"},
	Short:   "Inspect the PKGBUILD script and metadata of an AUR or official package",
	Long: `Inspect fetches and displays the PKGBUILD build recipe for a package in the
Arch User Repository (AUR) without downloading or executing it.

For official packages it shows verified metadata and build diagnostics from pacman.

Examples:
  aurora inspect paru
  aurora inspect paru --json
  aurora inspect paru --raw | less`,
	RunE:              inspect,
	ValidArgsFunction: completePackageNames,
}

func init() {
	inspectCmd.Flags().BoolVarP(&inspectRawFlag, "raw", "r", false, "output raw PKGBUILD text directly (suitable for piping)")
	inspectCmd.Flags().StringVar(&inspectFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	inspectCmd.Flags().BoolVarP(&inspectOfficialFlag, "official", "O", false, "inspect only official repositories")
	inspectCmd.Flags().BoolVarP(&inspectAURFlag, "aur", "A", false, "inspect only the Arch User Repository")
	rootCmd.AddCommand(inspectCmd)
}
