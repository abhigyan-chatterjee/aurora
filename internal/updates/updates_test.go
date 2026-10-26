package updates

import "testing"

func TestKindLabel(t *testing.T) {
	cases := map[Kind]string{
		KindOfficial:   "official",
		KindThirdParty: "third-party",
		KindAUR:        "AUR",
		Kind(99):       "unknown",
	}
	for kind, want := range cases {
		if got := kind.Label(); got != want {
			t.Errorf("Kind(%d).Label() = %q, want %q", kind, got, want)
		}
	}
}

func TestEntrySourceLabel(t *testing.T) {
	cases := []struct {
		entry Entry
		want  string
	}{
		{Entry{Kind: KindAUR}, "AUR"},
		{Entry{Kind: KindOfficial, Repo: "extra"}, "extra"},
		{Entry{Kind: KindThirdParty, Repo: "chaotic-aur"}, "chaotic-aur"},
		{Entry{Kind: KindOfficial}, "official"},
	}
	for _, tc := range cases {
		if got := tc.entry.SourceLabel(); got != tc.want {
			t.Errorf("SourceLabel() = %q, want %q", got, tc.want)
		}
	}
}

// The report splits by source so `outdated --aur` and `outdated --repos`
// cannot drift from what `outdated` shows in total.
func TestReportFiltering(t *testing.T) {
	report := Report{Entries: []Entry{
		{Name: "a", Kind: KindOfficial, Repo: "extra"},
		{Name: "b", Kind: KindAUR},
		{Name: "c", Kind: KindThirdParty, Repo: "chaotic-aur"},
		{Name: "d", Kind: KindAUR},
	}}

	aur := report.AUR()
	if len(aur) != 2 || aur[0].Name != "b" || aur[1].Name != "d" {
		t.Errorf("AUR() = %v, want b and d", names(aur))
	}

	repos := report.Repositories()
	if len(repos) != 2 {
		t.Fatalf("Repositories() = %v, want two entries", names(repos))
	}
	// Official entries must come before third-party ones.
	if repos[0].Kind != KindOfficial || repos[1].Kind != KindThirdParty {
		t.Errorf("Repositories() = %v, want official then third-party", names(repos))
	}

	if total := len(report.AUR()) + len(report.Repositories()); total != len(report.Entries) {
		t.Errorf("split covers %d of %d entries", total, len(report.Entries))
	}
}

func TestResolveAURNames(t *testing.T) {
	entries := []Entry{{Name: "one"}, {Name: "two"}}
	got := ResolveAURNames(entries)
	if len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("ResolveAURNames() = %v, want [one two]", got)
	}
	if empty := ResolveAURNames(nil); len(empty) != 0 {
		t.Errorf("expected no names for nil input, got %v", empty)
	}
}

func names(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}
