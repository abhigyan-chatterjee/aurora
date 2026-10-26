package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/config"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/run"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/spf13/cobra"
)

type doctorStatus string

const (
	statusOK   doctorStatus = "ok"
	statusWarn doctorStatus = "warn"
	statusFail doctorStatus = "fail"
	statusInfo doctorStatus = "info"
)

type doctorCheck struct {
	Name   string       `json:"name"`
	Status doctorStatus `json:"status"`
	Detail string       `json:"detail"`
}

func doctor(cmd *cobra.Command, args []string) error {
	checks := []doctorCheck{
		checkPacman(),
		checkTool("git", "needed to clone AUR build files", true),
		checkTool("makepkg", "needed to build AUR packages", true),
		checkTool("fakeroot", "needed by makepkg to package files with correct ownership", false),
		checkTool("sudo", "needed to install and remove packages", true),
		checkTool("vercmp", "needed to compare package versions", true),
		checkTool("checkupdates", "needed to check repository upgrades (pacman-contrib)", false),
		checkDatabaseFreshness(),
		checkAURReachable(),
		checkCacheDir(),
		checkOrphans(),
		checkConfig(),
	}

	var failures, warnings int
	for _, c := range checks {
		switch c.Status {
		case statusFail:
			failures++
		case statusWarn:
			warnings++
		}
	}

	if jsonFlag {
		payload := struct {
			Version  string        `json:"version"`
			Checks   []doctorCheck `json:"checks"`
			Failures int           `json:"failures"`
			Warnings int           `json:"warnings"`
		}{Version: Version, Checks: checks, Failures: failures, Warnings: warnings}
		if err := ui.EmitJSON(payload); err != nil {
			return err
		}
		if failures > 0 {
			return &silentExitError{code: 1}
		}
		return nil
	}

	ui.Out("\nAurora doctor (v%s, %s/%s)\n\n", Version, runtime.GOOS, runtime.GOARCH)
	for _, c := range checks {
		ui.Out("  [%s] %-22s %s\n", doctorMarker(c.Status), c.Name, c.Detail)
	}

	ui.Outln("")
	switch {
	case failures > 0:
		ui.Out("%d check(s) failed. Aurora cannot perform every operation on this machine.\n", failures)
		return &silentExitError{code: 1}
	case warnings > 0:
		ui.Out("Everything essential is present. %d optional check(s) warned.\n", warnings)
	default:
		ui.Outln("All checks passed.")
	}
	return nil
}

func doctorMarker(s doctorStatus) string {
	switch s {
	case statusOK:
		return "ok  "
	case statusWarn:
		return "warn"
	case statusFail:
		return "FAIL"
	default:
		return "info"
	}
}

func checkPacman() doctorCheck {
	if !run.Exists("pacman") {
		return doctorCheck{"pacman", statusFail, "not found on PATH; Aurora only runs on Arch-based systems"}
	}
	out, err := run.Output(run.QueryTimeout, "pacman", "-V")
	if err != nil {
		return doctorCheck{"pacman", statusWarn, "found, but `pacman -V` failed: " + err.Error()}
	}
	// `pacman -V` prints ASCII art on the same line as the version, so report
	// only the version substring and keep the column readable.
	for _, line := range strings.Split(string(out), "\n") {
		if idx := strings.Index(line, "Pacman v"); idx >= 0 {
			return doctorCheck{"pacman", statusOK, strings.TrimSpace(line[idx:])}
		}
	}
	return doctorCheck{"pacman", statusOK, "found"}
}

func checkTool(name, why string, required bool) doctorCheck {
	if run.Exists(name) {
		return doctorCheck{name, statusOK, "found"}
	}
	if required {
		return doctorCheck{name, statusFail, "missing: " + why}
	}
	return doctorCheck{name, statusWarn, "missing (optional): " + why}
}

func checkDatabaseFreshness() doctorCheck {
	const syncDir = "/var/lib/pacman/sync"
	entries, err := os.ReadDir(syncDir)
	if err != nil {
		return doctorCheck{"package databases", statusWarn, "could not read " + syncDir + ": " + err.Error()}
	}

	var newest time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if newest.IsZero() {
		return doctorCheck{"package databases", statusFail, "no sync databases found; run 'sudo pacman -Sy'"}
	}

	age := time.Since(newest)
	detail := fmt.Sprintf("last refreshed %s ago", humanDuration(age))
	if age > 14*24*time.Hour {
		return doctorCheck{"package databases", statusFail, detail + "; run 'sudo pacman -Syu'"}
	}
	if age > 7*24*time.Hour {
		return doctorCheck{"package databases", statusWarn, detail + "; consider refreshing"}
	}
	return doctorCheck{"package databases", statusOK, detail}
}

func checkAURReachable() doctorCheck {
	if _, err := aur.SearchAURExact("yay"); err != nil {
		return doctorCheck{"AUR reachable", statusWarn, "could not query aur.archlinux.org: " + err.Error()}
	}
	return doctorCheck{"AUR reachable", statusOK, "aur.archlinux.org responded"}
}

func checkCacheDir() doctorCheck {
	dir, err := aur.CacheDir()
	if err != nil {
		return doctorCheck{"build cache", statusFail, err.Error()}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return doctorCheck{"build cache", statusFail, "cannot create " + dir + ": " + err.Error()}
	}

	probe, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		return doctorCheck{"build cache", statusFail, "not writable: " + err.Error()}
	}
	name := probe.Name()
	probe.Close()
	_ = os.Remove(name)

	human := "empty"
	if size, err := dirSize(dir); err == nil && size > 0 {
		human = humanSize(size) + " used"
	}
	return doctorCheck{"build cache", statusOK, dir + " writable, " + human}
}

func checkOrphans() doctorCheck {
	orphans, err := pacman.Orphans()
	if err != nil {
		return doctorCheck{"orphan packages", statusWarn, "could not query: " + err.Error()}
	}
	if len(orphans) == 0 {
		return doctorCheck{"orphan packages", statusOK, "none"}
	}
	return doctorCheck{"orphan packages", statusInfo,
		fmt.Sprintf("%d orphan(s); run 'aurora autoremove' to clean up", len(orphans))}
}

func checkConfig() doctorCheck {
	path, err := config.Path()
	if err != nil {
		return doctorCheck{"configuration", statusWarn, err.Error()}
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return doctorCheck{"configuration", statusInfo, "no config file at " + path + " (defaults in use)"}
	}
	return doctorCheck{"configuration", statusOK, "loaded from " + path}
}

// humanDuration renders an age without pulling in a formatting dependency.
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that this machine can use every Aurora feature",
	Long: `Doctor verifies the things Aurora silently depends on: pacman and its tools,
git, makepkg, sudo, version comparison, database freshness, AUR reachability, a
writable build cache and a valid config file.

It changes nothing. Exits 1 when a required check fails.

Examples:
  aurora doctor
  aurora doctor --json`,
	RunE: doctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
