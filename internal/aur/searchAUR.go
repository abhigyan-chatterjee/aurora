package aur

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// SearchAUR performs a keyword search, most-voted first.
func SearchAUR(query string) ([]resolve.AURResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	raw, err := getRPC(client, searchEndpoint(query))
	if err != nil {
		return nil, err
	}
	if raw.ResultCount == 0 || len(raw.Results) == 0 {
		return nil, nil
	}

	results := make([]resolve.AURResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		results = append(results, r.toResult())
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].NumVotes != results[j].NumVotes {
			return results[i].NumVotes > results[j].NumVotes
		}
		return results[i].Name < results[j].Name
	})

	return results, nil
}

// SearchAURExact looks up one package by exact name. The RPC matches loosely,
// so the returned name is compared case-insensitively before it is accepted.
func SearchAURExact(packageName string) (*resolve.AURResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	raw, err := getRPC(client, infoEndpoint(packageName))
	if err != nil {
		return nil, err
	}
	if raw.ResultCount == 0 || len(raw.Results) == 0 {
		return nil, nil
	}

	for _, r := range raw.Results {
		if strings.EqualFold(r.Name, packageName) {
			result := r.toResult()
			return &result, nil
		}
	}
	return nil, nil
}
