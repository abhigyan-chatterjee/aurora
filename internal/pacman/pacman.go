package pacman

import (
	"sort"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/run"
)

// OfficialRepositories are the repositories maintained by Arch Linux.
//
// Anything a sync database provides that is not in this set came from a
// third-party repository such as chaotic-aur, endeavouros or garuda. Aurora
// reports those by their real name instead of calling them "official", because
// a user debugging a broken package needs to know which repo it came from.
var OfficialRepositories = map[string]bool{
	"core":             true,
	"extra":            true,
	"multilib":         true,
	"core-testing":     true,
	"extra-testing":    true,
	"multilib-testing": true,
	"core-staging":     true,
	"extra-staging":    true,
	"multilib-staging": true,
	"gnome-unstable":   true,
	"kde-unstable":     true,
}

// IsOfficialRepo reports whether repo is an Arch-maintained repository.
func IsOfficialRepo(repo string) bool {
	return OfficialRepositories[repo]
}

// RepoMap maps a package name to the sync repository that provides it.
//
// This replaces per-package `pacman -Qi` probing: one `pacman -Sl` call gives
// attribution for every repository the machine is subscribed to.
func RepoMap() (map[string]string, error) {
	out, err := run.Output(run.QueryTimeout, "pacman", "-Sl")
	if err != nil {
		return nil, err
	}

	repos := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		repos[fields[1]] = fields[0]
	}
	return repos, nil
}

// GetForeignPackages returns installed packages that no sync database provides,
// mapped to their installed version. These are packages built from the AUR or
// installed as local files.
func GetForeignPackages() (map[string]string, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Qm")
	if err != nil {
		return nil, err
	}
	return parseNameVersionPairs(string(out)), nil
}

// IsNewerVersion reports whether candidate is a newer version than current,
// using pacman's own vercmp so epoch and pkgrel are compared correctly.
//
// When vercmp cannot be run the answer is false. Claiming an update that cannot
// be verified makes the tool rebuild packages that are already current, which
// is worse than reporting nothing.
func IsNewerVersion(current, candidate string) bool {
	if current == "" || candidate == "" || current == candidate {
		return false
	}

	out, err := run.Output(run.QueryTimeout, "vercmp", candidate, current)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}

// VercompAvailable reports whether vercmp can be executed at all.
func VercompAvailable() bool {
	return run.Exists("vercmp")
}

// SystemUpdate runs a full system upgrade. When interactive is true pacman
// keeps its own confirmation prompts.
func SystemUpdate(interactive bool) error {
	args := []string{"pacman", "-Syu"}
	if !interactive {
		args = append(args, "--noconfirm")
	}
	return run.Interactive(run.BuildTimeout, "sudo", args...)
}

// InstallPacman installs official packages in a single transaction.
//
// --needed stops pacman reinstalling a package that is already current, which
// is what a user means when they type `aurora install neovim` twice.
func InstallPacman(names []string, interactive bool) error {
	if len(names) == 0 {
		return nil
	}
	args := []string{"pacman", "-S", "--needed"}
	if !interactive {
		args = append(args, "--noconfirm")
	}
	args = append(args, names...)
	return run.Interactive(run.BuildTimeout, "sudo", args...)
}

// RemovePacman removes packages, their unneeded dependencies and their config.
func RemovePacman(names []string, interactive bool) error {
	if len(names) == 0 {
		return nil
	}
	args := []string{"pacman", "-Rns"}
	if !interactive {
		args = append(args, "--noconfirm")
	}
	args = append(args, names...)
	return run.Interactive(run.BuildTimeout, "sudo", args...)
}

// SearchInstalled returns installed package names containing query, matched
// case-insensitively.
func SearchInstalled(query string) ([]string, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Qq")
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	var matches []string
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if strings.Contains(strings.ToLower(name), lowerQuery) {
			matches = append(matches, name)
		}
	}
	return matches, nil
}

// InstalledPackages returns every installed package name, sorted.
func InstalledPackages() ([]string, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Qq")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Orphans returns packages installed as dependencies that nothing requires.
func Orphans() ([]string, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Qtdq")
	if err != nil {
		return nil, err
	}
	var orphans []string
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name = strings.TrimSpace(name); name != "" {
			orphans = append(orphans, name)
		}
	}
	return orphans, nil
}

// OrphanListing returns orphans with their installed versions, for `list --orphans`.
func OrphanListing() ([]resolve.PacmanResult, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Qtd")
	if err != nil {
		return nil, err
	}
	pairs := parseNameVersionPairs(string(out))

	names := make([]string, 0, len(pairs))
	for name := range pairs {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]resolve.PacmanResult, 0, len(names))
	for _, name := range names {
		results = append(results, resolve.PacmanResult{Name: name, Version: pairs[name]})
	}
	return results, nil
}

// ExplicitPackages returns the `-Qe`/`-Q` listing as name/version pairs.
func ExplicitPackages(all bool) (map[string]string, error) {
	flag := "-Qe"
	if all {
		flag = "-Q"
	}
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", flag)
	if err != nil {
		return nil, err
	}
	return parseNameVersionPairs(string(out)), nil
}

// ForeignListing returns the `-Qme`/`-Qm` listing as name/version pairs.
func ForeignListing(all bool) (map[string]string, error) {
	flag := "-Qme"
	if all {
		flag = "-Qm"
	}
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", flag)
	if err != nil {
		return nil, err
	}
	return parseNameVersionPairs(string(out)), nil
}

// OfficialListing returns the `-Qne`/`-Qn` listing as name/version pairs.
func OfficialListing(all bool) (map[string]string, error) {
	flag := "-Qne"
	if all {
		flag = "-Qn"
	}
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", flag)
	if err != nil {
		return nil, err
	}
	return parseNameVersionPairs(string(out)), nil
}

// CheckUpdates returns packages with an available upgrade from the configured
// repositories, using checkupdates so the real databases are never touched.
func CheckUpdates() (map[string][2]string, bool, error) {
	if !run.Exists("checkupdates") {
		return nil, false, nil
	}

	out, err := run.Output(2*run.QueryTimeout, "checkupdates")
	if err != nil {
		// checkupdates exits 2 when there is nothing to upgrade, which is a
		// result rather than a failure. Exit 1 means it could not fetch the
		// database at all, and treating that as "no updates" would hide a
		// broken mirror behind "everything is up to date".
		if run.IsExitCode(err, 2) {
			return nil, true, nil
		}
		return nil, true, err
	}

	updates := make(map[string][2]string)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// Format: name current -> candidate
		if len(fields) >= 4 && fields[2] == "->" {
			updates[fields[0]] = [2]string{fields[1], fields[3]}
		}
	}
	return updates, true, nil
}

// RawPackageInfo returns pacman's full sync-database record for a package,
// used by `inspect` to show build diagnostics.
func RawPackageInfo(name string) (string, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Si", name)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// RunSudo runs a pacman command under sudo with stdio attached. It exists for
// cache pruning, which has no dedicated pacman helper above.
func RunSudo(args ...string) error {
	return run.Interactive(run.BuildTimeout, "sudo", args...)
}

func parseNameVersionPairs(out string) map[string]string {
	pairs := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pairs[fields[0]] = fields[1]
		}
	}
	return pairs
}
