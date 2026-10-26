// Package updates answers one question for the whole program: what can be
// upgraded right now.
//
// `aurora outdated`, `aurora list --upgradable` and `aurora update` all need the
// same answer. Before this package existed each command had its own copy of the
// version-check loop, which is how the third-party repository mislabelling and
// the unbounded vercmp process storm ended up duplicated too.
package updates

import (
	"sort"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/run"
)

// Kind describes where an upgrade would come from.
type Kind int

const (
	// KindOfficial is a package from a repository maintained by Arch.
	KindOfficial Kind = iota
	// KindThirdParty is a package from a repository the machine subscribes to
	// that Arch does not maintain, such as chaotic-aur.
	KindThirdParty
	// KindAUR is a package that no sync database provides.
	KindAUR
)

// Label returns a short human label for the kind.
func (k Kind) Label() string {
	switch k {
	case KindOfficial:
		return "official"
	case KindThirdParty:
		return "third-party"
	case KindAUR:
		return "AUR"
	}
	return "unknown"
}

// Entry is one package with an available upgrade.
type Entry struct {
	Name      string `json:"name"`
	Current   string `json:"current_version"`
	Candidate string `json:"new_version"`
	Repo      string `json:"repo,omitempty"`
	Kind      Kind   `json:"-"`
	// OutOfDate marks an AUR package its maintainer flagged as stale.
	OutOfDate bool `json:"out_of_date,omitempty"`
}

// SourceLabel is the label shown in listings.
func (e Entry) SourceLabel() string {
	if e.Kind == KindAUR {
		return "AUR"
	}
	if e.Repo != "" {
		return e.Repo
	}
	return e.Kind.Label()
}

// Report is the result of one check across both sources.
type Report struct {
	Entries []Entry

	// CheckupdatesAvailable is false when pacman-contrib is not installed, in
	// which case repository upgrades cannot be checked at all.
	CheckupdatesAvailable bool
	// AURCheckFailed records that the AUR could not be queried, so an empty
	// AUR list means "unknown" rather than "up to date".
	AURCheckFailed bool

	Warnings []error
}

// AUR returns only the AUR entries, in build order-friendly alphabetical form.
func (r Report) AUR() []Entry { return r.filter(KindAUR) }

// Repositories returns official and third-party entries.
func (r Report) Repositories() []Entry {
	out := r.filter(KindOfficial)
	return append(out, r.filter(KindThirdParty)...)
}

func (r Report) filter(kind Kind) []Entry {
	var out []Entry
	for _, e := range r.Entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// Collect checks both sources. It never returns an error: a failed source is
// reported in Warnings plus the matching flag, because a partial answer is
// still useful and the caller decides how loud to be.
func Collect(parallelism int) Report {
	var report Report
	if parallelism < 1 {
		parallelism = 1
	}

	report.Entries = append(report.Entries, collectAUR(&report, parallelism)...)
	report.Entries = append(report.Entries, collectRepositories(&report)...)

	sort.SliceStable(report.Entries, func(i, j int) bool {
		if report.Entries[i].Kind != report.Entries[j].Kind {
			return report.Entries[i].Kind < report.Entries[j].Kind
		}
		return report.Entries[i].Name < report.Entries[j].Name
	})

	return report
}

func collectAUR(report *Report, parallelism int) []Entry {
	foreign, err := pacman.GetForeignPackages()
	if err != nil {
		report.AURCheckFailed = true
		report.Warnings = append(report.Warnings, err)
		return nil
	}
	if len(foreign) == 0 {
		return nil
	}

	names := make([]string, 0, len(foreign))
	for name := range foreign {
		names = append(names, name)
	}
	sort.Strings(names)

	results, err := aur.GetAURInfoBatch(names)
	if err != nil {
		report.AURCheckFailed = true
		report.Warnings = append(report.Warnings, err)
	}
	if len(results) == 0 {
		return nil
	}

	// One vercmp per package, bounded. A goroutine per package spawned a
	// process per package, which is a fork storm on a large foreign set.
	newer := make([]bool, len(results))
	_ = run.Parallel(len(results), parallelism, func(i int) error {
		newer[i] = pacman.IsNewerVersion(foreign[results[i].Name], results[i].Version)
		return nil
	})

	out := make([]Entry, 0, len(results))
	for i, r := range results {
		if !newer[i] {
			continue
		}
		out = append(out, Entry{
			Name:      r.Name,
			Current:   foreign[r.Name],
			Candidate: r.Version,
			Kind:      KindAUR,
			OutOfDate: r.IsOutOfDate(),
		})
	}
	return out
}

func collectRepositories(report *Report) []Entry {
	available, hasCheckupdates, err := pacman.CheckUpdates()
	if err != nil {
		report.Warnings = append(report.Warnings, err)
	}
	report.CheckupdatesAvailable = hasCheckupdates
	if len(available) == 0 {
		return nil
	}

	// Attribution comes from one `pacman -Sl` call rather than a probe per
	// package, and it names third-party repositories instead of calling
	// everything outside the AUR "official".
	repoMap, err := pacman.RepoMap()
	if err != nil {
		report.Warnings = append(report.Warnings, err)
	}

	out := make([]Entry, 0, len(available))
	for name, pair := range available {
		repo := repoMap[name]
		kind := KindOfficial
		if repo != "" && !pacman.IsOfficialRepo(repo) {
			kind = KindThirdParty
		}
		out = append(out, Entry{
			Name:      name,
			Current:   pair[0],
			Candidate: pair[1],
			Repo:      repo,
			Kind:      kind,
		})
	}
	return out
}

// ResolveAURNames is a small helper for callers that only need names.
func ResolveAURNames(entries []Entry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names
}

// compile-time assertion that the report shape stays compatible with
// resolve.AURResult usage elsewhere.
var _ = resolve.AURResult{}
