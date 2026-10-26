package pacman

import "testing"

func TestParseNameVersionPairs(t *testing.T) {
	out := "bash 5.3.20-1\ncoreutils 9.7-1\n\nsingle\nno-version-line\n"
	got := parseNameVersionPairs(out)

	if len(got) != 2 {
		t.Fatalf("parsed %d pairs, want 2: %v", len(got), got)
	}
	if got["bash"] != "5.3.20-1" {
		t.Errorf("bash version = %q, want 5.3.20-1", got["bash"])
	}
	if got["coreutils"] != "9.7-1" {
		t.Errorf("coreutils version = %q, want 9.7-1", got["coreutils"])
	}
}

func TestIsOfficialRepo(t *testing.T) {
	official := []string{"core", "extra", "multilib", "core-testing", "extra-testing"}
	for _, repo := range official {
		if !IsOfficialRepo(repo) {
			t.Errorf("expected %q to be an official repository", repo)
		}
	}

	thirdParty := []string{"chaotic-aur", "endeavouros", "garuda", "archlinuxcn", ""}
	for _, repo := range thirdParty {
		if IsOfficialRepo(repo) {
			t.Errorf("expected %q to be treated as third-party", repo)
		}
	}
}
