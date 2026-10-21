package aur

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// aurInfoBatchSize caps how many packages are queried per RPC request.
// The AUR RPC interface encodes every name as an "arg[]=" query parameter in
// the URL, so oversized batches can exceed URL length limits on systems with
// many foreign packages installed.
const aurInfoBatchSize = 150

var aurRPCInfoBaseURL = "https://aur.archlinux.org/rpc/?v=5&type=info&"

func GetAURInfoBatch(packageNames []string) ([]resolve.AURResult, error) {
	if len(packageNames) == 0 {
		return nil, nil
	}

	var all []resolve.AURResult
	var chunkErrs []error
	for start := 0; start < len(packageNames); start += aurInfoBatchSize {
		end := start + aurInfoBatchSize
		if end > len(packageNames) {
			end = len(packageNames)
		}
		chunk, err := getAURInfoChunk(packageNames[start:end])
		if err != nil {
			chunkErrs = append(chunkErrs, err)
			continue
		}
		all = append(all, chunk...)
	}
	return all, errors.Join(chunkErrs...)
}

func getAURInfoChunk(packageNames []string) ([]resolve.AURResult, error) {
	var argsQuery []string
	for _, name := range packageNames {
		argsQuery = append(argsQuery, "arg[]="+url.QueryEscape(name))
	}

	reqURL := aurRPCInfoBaseURL + strings.Join(argsQuery, "&")

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := newAURRequest(reqURL)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error checking AUR: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	var raw struct {
		ResultCount int `json:"resultcount"`
		Results     []struct {
			Name         string `json:"Name"`
			Version      string `json:"Version"`
			Description  string `json:"Description"`
			URL          string `json:"URL"`
			LastModified int64  `json:"LastModified"`
			NumVotes     int    `json:"NumVotes"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("error decoding AUR response: %v", err)
	}

	var results []resolve.AURResult
	for _, r := range raw.Results {
		results = append(results, resolve.AURResult{
			Name:         r.Name,
			Version:      r.Version,
			Description:  r.Description,
			URL:          r.URL,
			LastModified: r.LastModified,
			NumVotes:     r.NumVotes,
		})
	}

	return results, nil
}
