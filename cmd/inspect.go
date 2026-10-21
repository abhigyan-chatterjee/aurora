package cmd

import (
	"fmt"
	"os/exec"
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

func inspect(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Println("Please provide a package name to inspect.")
		return nil
	}

	pkgName := args[0]
	if !ui.IsValidPkgName(pkgName) {
		fmt.Printf("Error: invalid package name format: %s\n", pkgName)
		return fmt.Errorf("invalid package name format: %s", pkgName)
	}

	sourceFilter := strings.ToLower(strings.TrimSpace(inspectFromFlag))
	if inspectOfficialFlag {
		sourceFilter = "official"
	} else if inspectAURFlag {
		sourceFilter = "aur"
	}

	// 1. AUR inspection (where PKGBUILD lives) unless restricted to official
	if sourceFilter != "official" && sourceFilter != "repo" {
		pkgbuildContent, aurErr := aur.FetchPKGBUILD(pkgName)
		if aurErr == nil {
			if inspectRawFlag {
				fmt.Print(pkgbuildContent)
				return nil
			}

			parsed := aur.ParsePKGBUILD(pkgbuildContent)
			exactAUR, _ := aur.SearchAURExact(pkgName)

			fmt.Printf("\n=== AUR Package Inspection: %s ===\n", pkgName)
			if parsed.Version != "" {
				rel := ""
				if parsed.Release != "" {
					rel = "-" + parsed.Release
				}
				fmt.Printf("Version     : %s%s\n", parsed.Version, rel)
			}
			if parsed.Description != "" {
				fmt.Printf("Description : %s\n", parsed.Description)
			}
			if parsed.URL != "" {
				fmt.Printf("Upstream URL: %s\n", parsed.URL)
			}
			if parsed.Maintainer != "" {
				fmt.Printf("Maintainer  : %s\n", parsed.Maintainer)
			}
			if exactAUR != nil {
				fmt.Printf("AUR Votes   : %d\n", exactAUR.NumVotes)
				if exactAUR.NumVotes < 2 {
					fmt.Println("Security Tip: This package has low community votes. Verify sources before building.")
				}
			}

			if len(parsed.Depends) > 0 {
				fmt.Printf("Depends On  : %s\n", strings.Join(parsed.Depends, ", "))
			}
			if len(parsed.MakeDepends) > 0 {
				fmt.Printf("Make Depends: %s\n", strings.Join(parsed.MakeDepends, ", "))
			}
			if len(parsed.Sources) > 0 {
				fmt.Println("Source URLs :")
				for _, src := range parsed.Sources {
					fmt.Printf("  - %s\n", src)
				}
			}

			fmt.Printf("\n--- PKGBUILD Script (%s) ---\n", pkgName)
			lines := strings.Split(pkgbuildContent, "\n")
			for i, line := range lines {
				fmt.Printf("%4d | %s\n", i+1, line)
			}
			return nil
		}
	}

	// 2. Official repository inspection unless restricted to AUR
	if sourceFilter != "aur" {
		out, err := exec.Command("pacman", "-Si", pkgName).Output()
		if err == nil {
			if pacmanPkg := pacman.ParsePacmanExactOutput(string(out), pkgName); pacmanPkg != nil {
				fmt.Printf("\n=== Official Repository Inspection: %s ===\n", pacmanPkg.Name)
				fmt.Printf("Repository  : %s\n", pacmanPkg.Repository)
				fmt.Printf("Version     : %s\n", pacmanPkg.Version)
				fmt.Printf("Description : %s\n", pacmanPkg.Description)
				fmt.Println("Source      : Official Arch Linux binary repository")
				fmt.Println("\nPackage diagnostics:")
				fmt.Println(strings.TrimSpace(string(out)))
				return nil
			}
		} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
			// Exit code 1 simply means "package not found"; anything else is a real error.
			fmt.Printf("Warning: error querying pacman: %v\n", err)
			return err
		}
	}

	fmt.Printf("Package \"%s\" was not found in specified sources.\n", pkgName)
	return fmt.Errorf("package %q not found in specified sources", pkgName)
}

var inspectCmd = &cobra.Command{
	Use:     "inspect <package>",
	Aliases: []string{"pkgbuild", "cat"},
	Short:   "Inspect the PKGBUILD script and metadata of an AUR or official package",
	Long: `Inspect fetches and displays the PKGBUILD script and build recipe for any package
in the Arch User Repository (AUR) without downloading or executing it.

For official packages, it displays verified package metadata and build diagnostics from pacman.`,
	RunE: inspect,
}

func init() {
	inspectCmd.Flags().BoolVarP(&inspectRawFlag, "raw", "r", false, "output raw PKGBUILD text directly (suitable for piping)")
	inspectCmd.Flags().StringVar(&inspectFromFlag, "from", "any", "filter source: 'official' (or 'repo') or 'aur'")
	inspectCmd.Flags().BoolVarP(&inspectOfficialFlag, "official", "O", false, "inspect only official repositories")
	inspectCmd.Flags().BoolVarP(&inspectAURFlag, "aur", "A", false, "inspect only the Arch User Repository")
	rootCmd.AddCommand(inspectCmd)
}
