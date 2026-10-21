package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"

	"github.com/spf13/cobra"
)

type outdatedAURPkg struct {
	name     string
	localVer string
	aurVer   string
}

func update(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Checking for updates...")

	var outdated []outdatedAURPkg
	var aurCheckFailed bool

	installed, err := pacman.GetForeignPackages()
	if err != nil {
		aurCheckFailed = true
		fmt.Printf("Warning: error checking foreign packages: %v\n", err)
	} else if len(installed) > 0 {
		var pkgNames []string
		for name := range installed {
			pkgNames = append(pkgNames, name)
		}
		sort.Strings(pkgNames)

		aurResults, err := aur.GetAURInfoBatch(pkgNames)
		if err != nil {
			aurCheckFailed = true
			fmt.Printf("Warning: error checking AUR updates: %v\n", err)
		}
		if len(aurResults) > 0 {
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, pkg := range aurResults {
				localVer := installed[pkg.Name]
				wg.Add(1)
				go func(p resolve.AURResult, lVer string) {
					defer wg.Done()
					if pacman.IsNewerVersion(lVer, p.Version) {
						mu.Lock()
						outdated = append(outdated, outdatedAURPkg{
							name:     p.Name,
							localVer: lVer,
							aurVer:   p.Version,
						})
						mu.Unlock()
					}
				}(pkg, localVer)
			}
			wg.Wait()

			sort.Slice(outdated, func(i, j int) bool {
				return outdated[i].name < outdated[j].name
			})
		}
	}

	fmt.Println("\nAurora update plan:")
	fmt.Println("  Step 1 - System upgrade: refresh package databases and upgrade official packages")
	if len(outdated) > 0 {
		fmt.Printf("  Step 2 - AUR packages to rebuild (%d):\n", len(outdated))
		for _, o := range outdated {
			fmt.Printf("    - %s (%s -> %s)\n", o.name, o.localVer, o.aurVer)
		}
	} else if aurCheckFailed {
		fmt.Println("  Step 2 - AUR packages: could not check for updates (see warning above)")
	} else {
		fmt.Println("  Step 2 - AUR packages: all up to date")
	}

	if !ui.ConfirmAction(reader, "\nProceed with update?", yesFlag) {
		fmt.Println("Update cancelled.")
		return nil
	}

	// The system upgrade runs first so AUR packages are rebuilt against the
	// newest libraries (building before -Syu risks linking against libraries
	// that the upgrade immediately replaces).
	fmt.Println("\n--- Upgrading System Packages ---")
	err = pacman.SystemUpdate()
	if err != nil {
		fmt.Printf("Error during system upgrade: %v\n", err)
		fmt.Println("Skipping AUR rebuilds: build against a fully upgraded system by re-running 'aurora update'.")
		return err
	}
	fmt.Println("System upgrade complete.")

	var aurBuildErrs []error
	if len(outdated) > 0 {
		fmt.Println("\n--- Upgrading AUR Packages ---")
		for _, o := range outdated {
			fmt.Printf("\nUpdating %s...\n", o.name)
			err := aur.InstallAUR(o.name)
			if err != nil {
				fmt.Printf("Error updating %s: %v\n", o.name, err)
				aurBuildErrs = append(aurBuildErrs, fmt.Errorf("%s: %w", o.name, err))
			}
		}
	}
	return errors.Join(aurBuildErrs...)
}

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"upgrade"},
	Short:   "Checks for AUR updates and performs a full system upgrade",
	Long: `Checks for AUR updates and performs a full system upgrade using 'sudo pacman -Syu --noconfirm'.

The system upgrade runs FIRST, then outdated AUR packages are rebuilt. This ordering
ensures AUR builds link against the newest system libraries (paru-style behavior).

Underlying pacman command flags:
  -S          : synchronize/install packages from repositories
  -y          : refresh package databases
  -u          : upgrade installed packages
  --noconfirm : skip pacman's per-package confirmation prompts`,
	RunE: update,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
