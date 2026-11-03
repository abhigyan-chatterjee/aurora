package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/config"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
)

func resetInstallGlobals(t *testing.T) {
	t.Cleanup(func() {
		yesFlag = false
		conciseFlag = false
		ui.Quiet = false
		appConfig = config.Default()
	})
	yesFlag = false
	conciseFlag = false
	ui.Quiet = false
	appConfig = config.Default()
}

func TestShouldCheckBin(t *testing.T) {
	resetInstallGlobals(t)
	cases := []struct {
		name   string
		pkg    string
		prefer string
		want   bool
	}{
		{"normal prompt", "foo", "prompt", true},
		{"bin suffix", "foo-bin", "prompt", false},
		{"never config", "foo", "never", false},
		{"always config", "foo", "always", true},
		{"bin suffix even always", "foo-bin", "always", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appConfig = config.Config{PreferBin: tc.prefer}
			got := shouldCheckBin(tc.pkg)
			if got != tc.want {
				t.Errorf("shouldCheckBin(%q, prefer=%q) = %v, want %v", tc.pkg, tc.prefer, got, tc.want)
			}
		})
	}
}

func TestQueuedBinName(t *testing.T) {
	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", AURResult: &resolve.AURResult{Name: "foo"}},
		{Query: "bar-bin", AURResult: &resolve.AURResult{Name: "bar-bin"}},
	}
	if !queuedBinName(resolved, "bar-bin") {
		t.Error("expected queuedBinName true for AURResult match")
	}
	if queuedBinName(resolved, "baz-bin") {
		t.Error("expected queuedBinName false for absent")
	}

	// query match
	if !queuedBinName([]*resolve.ResolvedPackage{{Query: "baz-bin"}}, "baz-bin") {
		t.Error("expected queuedBinName true for Query match")
	}

	// pacman result match
	if !queuedBinName([]*resolve.ResolvedPackage{{PacmanResult: &resolve.PacmanResult{Name: "qux-bin"}}}, "qux-bin") {
		t.Error("expected queuedBinName true for PacmanResult match")
	}
}

func mockAURBinServer(t *testing.T, binName, binVersion string, present bool) (*httptest.Server, *int32) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, binName) && present {
			fmt.Fprintf(w, `{"resultcount":1,"results":[{"Name":%q,"Version":%q}]}`, binName, binVersion)
		} else {
			fmt.Fprint(w, `{"resultcount":0,"results":[]}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestBuildInstallPlan_OfficialSkip(t *testing.T) {
	resetInstallGlobals(t)
	srv, calls := mockAURBinServer(t, "git-bin", "1.0", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "git", ChosenSource: resolve.SourceOfficial, PacmanResult: &resolve.PacmanResult{Name: "git", Repository: "core", Version: "2.4"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	official, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(official) != 1 || official[0].Name != "git" {
		t.Errorf("official = %+v", official)
	}
	if len(aurPlan) != 0 {
		t.Errorf("aurPlan = %+v", aurPlan)
	}
	if *calls != 0 {
		t.Errorf("AUR checked for official package: calls=%d", *calls)
	}
}

func TestBuildInstallPlan_BinSuffixSkip(t *testing.T) {
	resetInstallGlobals(t)
	srv, calls := mockAURBinServer(t, "yay-bin-bin", "1.0", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "yay-bin", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "yay-bin", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "yay-bin" {
		t.Errorf("aurPlan = %+v", aurPlan)
	}
	if *calls != 0 {
		t.Errorf("AUR checked for -bin-suffixed package: calls=%d", *calls)
	}
}

func TestBuildInstallPlan_BinHitDefaultEnter(t *testing.T) {
	resetInstallGlobals(t)
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader("\n"))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo-bin" {
		t.Errorf("aurPlan = %+v, want foo-bin", aurPlan)
	}
	if aurPlan[0].Version != "9.9" {
		t.Errorf("aurPlan[0].Version = %q, want 9.9", aurPlan[0].Version)
	}
}

func TestBuildInstallPlan_BinHitExplicitSource(t *testing.T) {
	resetInstallGlobals(t)
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	for _, in := range []string{"2\n", "s\n"} {
		resolved := []*resolve.ResolvedPackage{
			{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
		}
		reader := bufio.NewReader(strings.NewReader(in))
		_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
		if len(errs) != 0 {
			t.Fatalf("errs: %v", errs)
		}
		if len(aurPlan) != 1 || aurPlan[0].Name != "foo" {
			t.Errorf("input %q: aurPlan = %+v, want foo", in, aurPlan)
		}
	}
}

func TestBuildInstallPlan_BinHitSkip(t *testing.T) {
	resetInstallGlobals(t)
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader("q\n"))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 0 {
		t.Errorf("aurPlan = %+v, want empty (skipped)", aurPlan)
	}
}

func TestBuildInstallPlan_ConfigNever(t *testing.T) {
	resetInstallGlobals(t)
	appConfig = config.Config{PreferBin: "never", AURDeps: true}
	srv, calls := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo" {
		t.Errorf("aurPlan = %+v, want foo", aurPlan)
	}
	if *calls != 0 {
		t.Errorf("AUR checked despite prefer_bin=never: calls=%d", *calls)
	}
}

func TestBuildInstallPlan_RPCFailureFallback(t *testing.T) {
	resetInstallGlobals(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo" {
		t.Errorf("aurPlan = %+v, want foo (fallback)", aurPlan)
	}
}

func TestBuildInstallPlan_NoBinFallback(t *testing.T) {
	resetInstallGlobals(t)
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", false)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo" {
		t.Errorf("aurPlan = %+v, want foo", aurPlan)
	}
}

func TestBuildInstallPlan_QueuedBin(t *testing.T) {
	resetInstallGlobals(t)
	srv, calls := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
		{Query: "foo-bin", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo-bin", Version: "9.9"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	// foo should remain foo (not replaced) because foo-bin already queued
	foundFoo := false
	for _, it := range aurPlan {
		if it.Name == "foo" {
			foundFoo = true
		}
	}
	if !foundFoo {
		t.Errorf("aurPlan = %+v, expected foo retained", aurPlan)
	}
	if *calls != 0 {
		t.Errorf("AUR checked despite queued bin: calls=%d", *calls)
	}
}

func TestBuildInstallPlan_YesFlag(t *testing.T) {
	resetInstallGlobals(t)
	yesFlag = true
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo-bin" {
		t.Errorf("aurPlan = %+v, want foo-bin", aurPlan)
	}
}

func TestBuildInstallPlan_ConfigAlways(t *testing.T) {
	resetInstallGlobals(t)
	appConfig = config.Config{PreferBin: "always", AURDeps: true}
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader(""))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo-bin" {
		t.Errorf("aurPlan = %+v, want foo-bin", aurPlan)
	}
}

func TestBuildInstallPlan_InvalidInputRecovery(t *testing.T) {
	resetInstallGlobals(t)
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader("xyz\n1\n"))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo-bin" {
		t.Errorf("aurPlan = %+v, want foo-bin", aurPlan)
	}
}

func TestBuildInstallPlan_DidYouMeanAURBin(t *testing.T) {
	resetInstallGlobals(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/search/") {
			fmt.Fprint(w, `{"resultcount":1,"results":[{"Name":"foo","Version":"1.0","Description":"d","NumVotes":1,"Maintainer":"m"}]}`)
			return
		}
		if strings.Contains(r.URL.Path, "foo-bin") {
			fmt.Fprintf(w, `{"resultcount":1,"results":[{"Name":"foo-bin","Version":"9.9"}]}`)
			return
		}
		fmt.Fprint(w, `{"resultcount":0,"results":[]}`)
	}))
	defer srv.Close()
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "fooo", ChosenSource: resolve.SourceUnknown},
	}
	reader := bufio.NewReader(strings.NewReader("1\n\n"))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAUR)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo-bin" {
		t.Errorf("aurPlan = %+v, want foo-bin", aurPlan)
	}
	if aurPlan[0].Version != "9.9" {
		t.Errorf("aurPlan[0].Version = %q, want 9.9", aurPlan[0].Version)
	}
}
func TestBuildInstallPlan_Concise(t *testing.T) {
	resetInstallGlobals(t)
	conciseFlag = true
	srv, _ := mockAURBinServer(t, "foo-bin", "9.9", true)
	orig := aur.SetRPCBaseURL(srv.URL)
	defer func() { aur.SetRPCBaseURL(orig) }()

	resolved := []*resolve.ResolvedPackage{
		{Query: "foo", ChosenSource: resolve.SourceAUR, AURResult: &resolve.AURResult{Name: "foo", Version: "1.0"}},
	}
	reader := bufio.NewReader(strings.NewReader("\n"))
	_, aurPlan, errs := buildInstallPlan(resolved, reader, filterAny)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(aurPlan) != 1 || aurPlan[0].Name != "foo-bin" {
		t.Errorf("aurPlan = %+v, want foo-bin", aurPlan)
	}
}
