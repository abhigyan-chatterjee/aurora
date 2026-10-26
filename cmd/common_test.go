package cmd

import "testing"

func TestParseSourceFilter(t *testing.T) {
	cases := []struct {
		name         string
		from         string
		officialOnly bool
		aurOnly      bool
		want         sourceFilter
		wantErr      bool
	}{
		{name: "default is any", from: "any", want: filterAny},
		{name: "empty is any", from: "", want: filterAny},
		{name: "official", from: "official", want: filterOfficial},
		{name: "repo alias", from: "repo", want: filterOfficial},
		{name: "aur", from: "aur", want: filterAUR},
		{name: "case insensitive", from: "AUR", want: filterAUR},
		{name: "-O wins", from: "any", officialOnly: true, want: filterOfficial},
		{name: "-A wins", from: "any", aurOnly: true, want: filterAUR},

		// A typo must not silently widen the search to every source.
		{name: "typo rejected", from: "oficiial", wantErr: true},
		{name: "nonsense rejected", from: "mars", wantErr: true},
		{name: "conflicting flags rejected", officialOnly: true, aurOnly: true, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSourceFilter(tc.from, tc.officialOnly, tc.aurOnly)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for from=%q official=%v aur=%v", tc.from, tc.officialOnly, tc.aurOnly)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseSourceFilter(%q) = %v, want %v", tc.from, got, tc.want)
			}
		})
	}
}

func TestCollectValidArgs(t *testing.T) {
	valid, errs := collectValidArgs([]string{"neovim", "neovim", "NEOVIM", "gtk+3"})

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	want := []string{"neovim", "gtk+3"}
	if len(valid) != len(want) {
		t.Fatalf("valid = %v, want %v", valid, want)
	}
	for i := range want {
		if valid[i] != want[i] {
			t.Errorf("valid = %v, want %v", valid, want)
		}
	}
}

// An invalid name must be reported, not skipped in silence. The old behaviour
// installed the valid names and exited 0, so the user believed every argument
// had been handled.
func TestCollectValidArgs_ReportsInvalidNames(t *testing.T) {
	valid, errs := collectValidArgs([]string{"neovim", "../etc/passwd", "-Syu"})

	if len(valid) != 1 || valid[0] != "neovim" {
		t.Errorf("valid = %v, want [neovim]", valid)
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors for 2 invalid names, got %d: %v", len(errs), errs)
	}
}

func TestSourceFilterString(t *testing.T) {
	if got := filterOfficial.String(); got != "official" {
		t.Errorf("filterOfficial.String() = %q", got)
	}
	if got := filterAUR.String(); got != "AUR" {
		t.Errorf("filterAUR.String() = %q", got)
	}
	if got := filterAny.String(); got != "any" {
		t.Errorf("filterAny.String() = %q", got)
	}
}
