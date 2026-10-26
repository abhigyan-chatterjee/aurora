package cmd

import (
	"fmt"

	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/abhigyan-chatterjee/aurora/internal/updates"
	"github.com/spf13/cobra"
)

// silentExitError asks Execute to exit with a specific status without printing
// an error. It carries meaning: "there is work to do", not "something broke".
type silentExitError struct {
	code int
}

func (e *silentExitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// ExitCode lets Execute unwrap the intended status.
func (e *silentExitError) ExitCode() int { return e.code }

var (
	outdatedAUROnlyFlag bool
	outdatedReposFlag   bool
)

func outdated(cmd *cobra.Command, args []string) error {
	report := updates.Collect(appConfig.Parallelism)
	for _, w := range report.Warnings {
		ui.Warn("%v", w)
	}

	entries := report.Entries
	switch {
	case outdatedAUROnlyFlag:
		entries = report.AUR()
	case outdatedReposFlag:
		entries = report.Repositories()
	}

	if jsonFlag {
		list := make([]listEntryJSON, 0, len(entries))
		for _, e := range entries {
			list = append(list, listEntryJSON{
				Name:    e.Name,
				Version: e.Current,
				Source:  e.SourceLabel(),
				Detail:  e.Candidate,
			})
		}
		payload := struct {
			Updates               []listEntryJSON `json:"updates"`
			Count                 int             `json:"count"`
			AURCheckFailed        bool            `json:"aur_check_failed"`
			CheckupdatesAvailable bool            `json:"checkupdates_available"`
		}{
			Updates:               list,
			Count:                 len(entries),
			AURCheckFailed:        report.AURCheckFailed,
			CheckupdatesAvailable: report.CheckupdatesAvailable,
		}
		if err := ui.EmitJSON(payload); err != nil {
			return err
		}
		if len(entries) > 0 {
			return &silentExitError{code: 1}
		}
		return nil
	}

	if len(entries) == 0 {
		if report.AURCheckFailed {
			ui.Outln("No repository updates found, but the AUR could not be checked.")
			return fmt.Errorf("AUR update check failed")
		}
		ui.Outln("Everything is up to date.")
		return nil
	}

	ui.Out("\n%d update(s) available:\n", len(entries))
	for _, e := range entries {
		note := ""
		if e.OutOfDate {
			note = " (flagged out of date)"
		}
		ui.Out("  %-30s %-18s -> %-18s [%s]%s\n", e.Name, e.Current, e.Candidate, e.SourceLabel(), note)
	}
	ui.Outln("\nRun 'aurora update' to apply them, or 'aurora update --dry-run' to preview.")

	return &silentExitError{code: 1}
}

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "List available updates without changing anything (read-only)",
	Long: `Outdated checks the AUR and the configured repositories for upgrades and prints
them. It changes nothing, needs no sudo, and exits 1 when updates are available so
it can be used from scripts and cron.

Examples:
  aurora outdated
  aurora outdated --aur
  aurora outdated --json`,
	RunE: outdated,
}

func init() {
	outdatedCmd.Flags().BoolVar(&outdatedAUROnlyFlag, "aur", false, "only report AUR packages")
	outdatedCmd.Flags().BoolVar(&outdatedReposFlag, "repos", false, "only report repository packages")
	rootCmd.AddCommand(outdatedCmd)
}
