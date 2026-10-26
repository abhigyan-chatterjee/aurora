package cmd

import (
	"fmt"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
)

// sourceFilter is the resolved meaning of --from / -O / -A.
type sourceFilter int

const (
	filterAny sourceFilter = iota
	filterOfficial
	filterAUR
)

// String makes the filter usable in messages.
func (f sourceFilter) String() string {
	switch f {
	case filterOfficial:
		return "official"
	case filterAUR:
		return "AUR"
	default:
		return "any"
	}
}

// parseSourceFilter turns the flags into a filter.
//
// An unrecognised --from value is an error rather than a silent fallback.
// `--from oficial` used to widen the search to both sources, which on install
// meant a typo could produce an AUR build when the user asked for an official
// package.
func parseSourceFilter(from string, officialOnly, aurOnly bool) (sourceFilter, error) {
	if officialOnly && aurOnly {
		return filterAny, fmt.Errorf("--official and --aur cannot be combined")
	}
	if officialOnly {
		return filterOfficial, nil
	}
	if aurOnly {
		return filterAUR, nil
	}

	switch strings.ToLower(strings.TrimSpace(from)) {
	case "", "any", "all", "both":
		return filterAny, nil
	case "official", "repo", "repos", "repository":
		return filterOfficial, nil
	case "aur":
		return filterAUR, nil
	default:
		return filterAny, fmt.Errorf("invalid --from value %q; use 'official' (or 'repo') or 'aur'", from)
	}
}

// collectValidArgs validates package names, rejects duplicates and returns any
// invalid names as errors.
//
// Invalid names are reported instead of silently dropped: `aurora install
// neovim ../etc/passwd` used to install neovim and exit 0, leaving the user
// believing everything they typed was handled.
func collectValidArgs(args []string) (valid []string, errs []error) {
	seen := make(map[string]bool, len(args))
	for _, arg := range args {
		if !ui.IsValidPkgName(arg) {
			errs = append(errs, fmt.Errorf("invalid package name: %q", arg))
			continue
		}
		norm := strings.ToLower(arg)
		if seen[norm] {
			continue
		}
		seen[norm] = true
		valid = append(valid, arg)
	}
	return valid, errs
}

// installedPackageNames is the completion source for commands that take an
// already-installed package. Completion must never block, so a pacman failure
// yields no suggestions rather than an error.
func installedPackageNames() ([]string, error) {
	return pacman.InstalledPackages()
}

// filterOfficialResults keeps only packages provided by an Arch-maintained
// repository.
//
// `pacman -Si` answers for every sync database, including chaotic-aur and
// distribution-specific repos. Without this filter `--from official` silently
// offers third-party packages, which is the opposite of what the flag says.
func filterOfficialResults(in []resolve.PacmanResult) []resolve.PacmanResult {
	out := make([]resolve.PacmanResult, 0, len(in))
	for _, r := range in {
		if pacman.IsOfficialRepo(r.Repository) {
			out = append(out, r)
		}
	}
	return out
}

// pacmanSourceLabel describes where a sync-database package came from.
// A package provided by chaotic-aur is not "official", and saying so sends the
// user looking in the wrong place when it breaks.
func pacmanSourceLabel(repo string) string {
	switch {
	case repo == "":
		return "Arch repositories"
	case pacman.IsOfficialRepo(repo):
		return "official repositories (" + repo + ")"
	default:
		return "third-party repository " + repo
	}
}

// warnResolution prints non-fatal lookup problems recorded on a result.
func warnResolution(query string, warnings []error) {
	for _, w := range warnings {
		ui.Warn("could not fully check sources for \"%s\": %v", query, w)
	}
}
