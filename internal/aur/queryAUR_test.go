package aur

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetAURInfoBatch_Empty(t *testing.T) {
	results, err := GetAURInfoBatch(nil)
	if err != nil {
		t.Fatalf("unexpected error on nil input: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results on nil input, got %+v", results)
	}

	results, err = GetAURInfoBatch([]string{})
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results on empty input, got %+v", results)
	}
}

func TestGetAURInfoBatch_Chunking(t *testing.T) {
	var requestCount int32
	var chunkSizes []int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		args := r.URL.Query()["arg[]"]
		chunkSizes = append(chunkSizes, len(args))

		var items []string
		for _, name := range args {
			items = append(items, fmt.Sprintf(`{"Name":%q,"Version":"1.0.0-1"}`, name))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"resultcount":%d,"results":[%s]}`, len(items), strings.Join(items, ","))
	}))
	defer srv.Close()

	origURL := aurRPCInfoBaseURL
	aurRPCInfoBaseURL = srv.URL + "/rpc/?v=5&type=info&"
	defer func() { aurRPCInfoBaseURL = origURL }()

	// 310 packages -> 3 chunks: 150, 150, 10
	names := make([]string, 310)
	for i := range names {
		names[i] = fmt.Sprintf("pkg-%d", i)
	}

	results, err := GetAURInfoBatch(names)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 310 {
		t.Fatalf("expected 310 results, got %d", len(results))
	}
	if got := atomic.LoadInt32(&requestCount); got != 3 {
		t.Fatalf("expected 3 chunked RPC requests, got %d", got)
	}
	wantSizes := []int{150, 150, 10}
	for i, want := range wantSizes {
		if chunkSizes[i] != want {
			t.Errorf("chunk %d size = %d, want %d", i, chunkSizes[i], want)
		}
	}
}

func TestGetAURInfoBatch_PartialFailurePreservesChunks(t *testing.T) {
	var call int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := atomic.AddInt32(&call, 1)
		args := r.URL.Query()["arg[]"]
		if c == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var items []string
		for _, name := range args {
			items = append(items, fmt.Sprintf(`{"Name":%q,"Version":"1.0.0-1"}`, name))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"resultcount":%d,"results":[%s]}`, len(items), strings.Join(items, ","))
	}))
	defer srv.Close()

	origURL := aurRPCInfoBaseURL
	aurRPCInfoBaseURL = srv.URL + "/rpc/?v=5&type=info&"
	defer func() { aurRPCInfoBaseURL = origURL }()

	names := make([]string, 160) // 150 in chunk 1 (succeeds), 10 in chunk 2 (fails)
	for i := range names {
		names[i] = fmt.Sprintf("pkg-%d", i)
	}

	results, err := GetAURInfoBatch(names)
	if err == nil {
		t.Fatalf("expected non-nil error when second chunk fails")
	}
	if len(results) != 150 {
		t.Fatalf("expected 150 partial results from first chunk, got %d", len(results))
	}
}
