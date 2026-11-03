package aur

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckBinPackage_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"resultcount":1,"results":[{"Name":"foo-bin","Version":"1.2.3-1","Description":"a bin","URL":"https://example.com","NumVotes":5,"Maintainer":"m"}]}`)
	}))
	defer srv.Close()

	orig := aurRPCBaseURL
	aurRPCBaseURL = srv.URL
	defer func() { aurRPCBaseURL = orig }()

	res, err := CheckBinPackage("foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Name != "foo-bin" {
		t.Errorf("name = %q, want foo-bin", res.Name)
	}
	if res.Version != "1.2.3-1" {
		t.Errorf("version = %q", res.Version)
	}
}

func TestCheckBinPackage_Missing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"resultcount":0,"results":[]}`)
	}))
	defer srv.Close()

	orig := aurRPCBaseURL
	aurRPCBaseURL = srv.URL
	defer func() { aurRPCBaseURL = orig }()

	res, err := CheckBinPackage("ghost")
	if err != nil {
		t.Fatalf("unexpected error on missing: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result for missing, got %+v", res)
	}
}

func TestCheckBinPackage_NetworkError(t *testing.T) {
	orig := aurRPCBaseURL
	aurRPCBaseURL = "http://127.0.0.1:1" // closed port
	defer func() { aurRPCBaseURL = orig }()

	res, err := CheckBinPackage("foo")
	if err == nil {
		t.Fatal("expected network error")
	}
	if res != nil {
		t.Errorf("expected nil result on error, got %+v", res)
	}
}

func TestCheckBinPackage_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{not json`)
	}))
	defer srv.Close()

	orig := aurRPCBaseURL
	aurRPCBaseURL = srv.URL
	defer func() { aurRPCBaseURL = orig }()

	res, err := CheckBinPackage("foo")
	if err == nil {
		t.Fatal("expected decode error on malformed response")
	}
	if res != nil {
		t.Errorf("expected nil result on malformed, got %+v", res)
	}
}

func TestCheckBinPackage_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := aurRPCBaseURL
	aurRPCBaseURL = srv.URL
	defer func() { aurRPCBaseURL = orig }()

	res, err := CheckBinPackage("foo")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if res != nil {
		t.Errorf("expected nil result on 500, got %+v", res)
	}
}

func TestCheckBinPackage_DoesNotQueryOfficialRepos(t *testing.T) {
	// The function must only hit the AUR RPC endpoint. Verify the request path
	// contains /info/ and the -bin suffix.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"resultcount":0,"results":[]}`)
	}))
	defer srv.Close()

	orig := aurRPCBaseURL
	aurRPCBaseURL = srv.URL
	defer func() { aurRPCBaseURL = orig }()

	_, _ = CheckBinPackage("foo")
	if gotPath != "/info/foo-bin" {
		t.Errorf("request path = %q, want /info/foo-bin", gotPath)
	}
}
