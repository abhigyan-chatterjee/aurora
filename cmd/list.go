package cmd

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/spf13/cobra"
)

var (
	listOfficialFlag   bool
	listAURFlag        bool
	listOrphansFlag    bool
	listUpgradableFlag bool
	listAllFlag        bool
)

type listEntry struct {
	Name    string
	Version string
	Source  string
	Detail  string
}

// runPacmanQuery runs a read-only pacman query. Pacman's exit code 1 means
// "no matching packages", which is treated as an empty result, not an error.
func runPacmanQuery(args ...string) (string, error) {
	out, err := exec.Command("pacman", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return string(out), nil
}

func listPackages(cmd *cobra.Command, args []string) {
	var query string
	if len(args) > 0 {
		query = strings.ToLower(strings.TrimSpace(args[0]))
	}

	var entries []listEntry

	if listOrphansFlag {
		// 1. List orphans
		cmdOut, err := runPacmanQuery("-Qtd")
		if err != nil {
			fmt.Printf("Warning: error listing orphan packages: %v\n", err)
		} else {
			lines := strings.Split(strings.TrimSpace(cmdOut), "\n")
			for _, l := range lines {
				parts := strings.Fields(l)
				if len(parts) >= 2 {
					name := parts[0]
					ver := parts[1]
					if query == "" || strings.Contains(strings.ToLower(name), query) {
						entries = append(entries, listEntry{
							Name:    name,
							Version: ver,
							Source:  "orphan",
						})
					}
				}
			}
		}
	} else if listUpgradableFlag {
		// 2. List upgradable packages (Foreign/AUR + Official)
		fmt.Println("Checking for upgradable packages...")

		// Check foreign/AUR packages
		foreign, err := pacman.GetForeignPackages()
		if err != nil {
			fmt.Printf("Warning: error checking foreign packages: %v\n", err)
		} else if len(foreign) > 0 {
			var pkgNames []string
			for name := range foreign {
				pkgNames = append(pkgNames, name)
			}
			sort.Strings(pkgNames)

			aurResults, aErr := aur.GetAURInfoBatch(pkgNames)
			if aErr != nil {
				fmt.Printf("Warning: error checking AUR updates: %v\n", aErr)
			}
			if len(aurResults) > 0 {
				var mu sync.Mutex
				var wg sync.WaitGroup
				for _, p := range aurResults {
					localVer := foreign[p.Name]
					wg.Add(1)
					go func(pkg resolve.AURResult, lVer string) {
						defer wg.Done()
						if pacman.IsNewerVersion(lVer, pkg.Version) {
							if query == "" || strings.Contains(strings.ToLower(pkg.Name), query) {
								mu.Lock()
								entries = append(entries, listEntry{
									Name:    pkg.Name,
									Version: lVer,
									Source:  "AUR",
									Detail:  fmt.Sprintf("upgradable to %s", pkg.Version),
								})
								mu.Unlock()
							}
						}
					}(p, localVer)
				}
				wg.Wait()
			}
		}

		// Check official updates via checkupdates if installed
		if checkUpdatesPath, err := exec.LookPath("checkupdates"); err == nil {
			out, err := exec.Command(checkUpdatesPath).Output()
			if err == nil {
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				for _, line := range lines {
					parts := strings.Fields(line)
					if len(parts) >= 4 && parts[2] == "->" {
						pkgName := parts[0]
						currVer := parts[1]
						newVer := parts[3]
						if query == "" || strings.Contains(strings.ToLower(pkgName), query) {
							entries = append(entries, listEntry{
								Name:    pkgName,
								Version: currVer,
								Source:  "official",
								Detail:  fmt.Sprintf("upgradable to %s", newVer),
							})
						}
					}
				}
			}
		}
	} else if listAURFlag {
		// 3. List foreign / AUR packages
		subFlag := "-Qme"
		if listAllFlag {
			subFlag = "-Qm"
		}
		out, err := runPacmanQuery(subFlag)
		if err != nil {
			fmt.Printf("Warning: error querying foreign packages: %v\n", err)
		} else {
			lines := strings.Split(strings.TrimSpace(out), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					name := parts[0]
					ver := parts[1]
					if query == "" || strings.Contains(strings.ToLower(name), query) {
						entries = append(entries, listEntry{
							Name:    name,
							Version: ver,
							Source:  "AUR",
						})
					}
				}
			}
		}
	} else if listOfficialFlag {
		// 4. List official packages
		subFlag := "-Qne"
		if listAllFlag {
			subFlag = "-Qn"
		}
		out, err := runPacmanQuery(subFlag)
		if err != nil {
			fmt.Printf("Warning: error querying official packages: %v\n", err)
		} else {
			lines := strings.Split(strings.TrimSpace(out), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					name := parts[0]
					ver := parts[1]
					if query == "" || strings.Contains(strings.ToLower(name), query) {
						entries = append(entries, listEntry{
							Name:    name,
							Version: ver,
							Source:  "official",
						})
					}
				}
			}
		}
	} else {
		// 5. Default: List installed packages (official + AUR)
		foreignMap, _ := pacman.GetForeignPackages()
		subFlag := "-Qe"
		if listAllFlag {
			subFlag = "-Q"
		}
		out, err := runPacmanQuery(subFlag)
		if err != nil {
			fmt.Printf("Warning: error querying installed packages: %v\n", err)
		} else {
			lines := strings.Split(strings.TrimSpace(out), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					name := parts[0]
					ver := parts[1]
					source := "official"
					if _, isForeign := foreignMap[name]; isForeign {
						source = "AUR"
					}
					if query == "" || strings.Contains(strings.ToLower(name), query) {
						entries = append(entries, listEntry{
							Name:    name,
							Version: ver,
							Source:  source,
						})
					}
				}
			}
		}
	}

	if len(entries) == 0 {
		if query != "" {
			fmt.Printf("No matching installed packages found for %q.\n", query)
		} else {
			fmt.Println("No packages found.")
		}
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	fmt.Printf("\nListing %d package(s):\n", len(entries))
	for _, e := range entries {
		extra := ""
		if e.Detail != "" {
			extra = fmt.Sprintf(" (%s)", e.Detail)
		}
		fmt.Printf("  %-30s %-18s [%s]%s\n", e.Name, e.Version, e.Source, extra)
	}
}

var listCmd = &cobra.Command{
	Use:   "list [filter]",
	Short: "List installed packages, updates, or orphans (apt-style query)",
	Long: `List displays installed packages on your system with their versions and origins (Official vs AUR).
It supports apt-style filters like --upgradable, --orphans, --official, --aur, and --all.

Examples:
  aurora list                 # List explicitly installed packages
  aurora list --all           # List all installed packages (including dependencies)
  aurora list python          # Search installed packages containing 'python'
  aurora list --upgradable    # List packages with available updates
  aurora list --foreign       # List installed AUR packages
  aurora list --orphans       # List unused orphan dependencies`,
	Run: listPackages,
}

func init() {
	listCmd.Flags().BoolVarP(&listOfficialFlag, "official", "o", false, "list only official repository packages")
	listCmd.Flags().BoolVarP(&listAURFlag, "foreign", "m", false, "list only foreign / AUR packages")
	listCmd.Flags().BoolVar(&listAURFlag, "aur", false, "alias for --foreign")
	listCmd.Flags().BoolVar(&listOrphansFlag, "orphans", false, "list unneeded orphan dependencies")
	listCmd.Flags().BoolVarP(&listUpgradableFlag, "upgradable", "u", false, "list packages with available upgrades")
	listCmd.Flags().BoolVarP(&listAllFlag, "all", "a", false, "list all installed packages (including dependencies)")
	rootCmd.AddCommand(listCmd)
}
