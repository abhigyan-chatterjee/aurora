package aur

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// aurRPCBaseURL is the AUR RPC v5 endpoint. It is a variable so tests can point
// it at a local server.
var aurRPCBaseURL = "https://aur.archlinux.org/rpc/v5"

// rpcPackage is the single shape of an AUR RPC package across info, search and
// batch queries. There used to be three identical anonymous structs.
type rpcPackage struct {
	Name         string `json:"Name"`
	Version      string `json:"Version"`
	Description  string `json:"Description"`
	URL          string `json:"URL"`
	LastModified int64  `json:"LastModified"`
	NumVotes     int    `json:"NumVotes"`
	Maintainer   string `json:"Maintainer"`
	OutOfDate    int64  `json:"OutOfDate"`
}

type rpcResponse struct {
	ResultCount int          `json:"resultcount"`
	Type        string       `json:"type"`
	Results     []rpcPackage `json:"results"`
}

func (p rpcPackage) toResult() resolve.AURResult {
	return resolve.AURResult{
		Name:         p.Name,
		Version:      p.Version,
		Description:  p.Description,
		URL:          p.URL,
		LastModified: p.LastModified,
		NumVotes:     p.NumVotes,
		Maintainer:   p.Maintainer,
		OutOfDate:    p.OutOfDate,
	}
}

var (
	versionMu sync.RWMutex
	version   = "dev"
)

// SetVersion records the running Aurora version for the User-Agent header.
// The AUR asks clients to identify themselves, and a header that drifts from
// the real version makes its logs useless.
func SetVersion(v string) {
	if v == "" {
		return
	}
	versionMu.Lock()
	version = v
	versionMu.Unlock()
}

func userAgent() string {
	versionMu.RLock()
	defer versionMu.RUnlock()
	return "aurora/" + version
}

func newAURRequest(endpoint string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	return req, nil
}

// getRPC performs one RPC request and decodes the response.
func getRPC(client *http.Client, endpoint string) (*rpcResponse, error) {
	req, err := newAURRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error contacting the AUR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	var raw rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("error decoding AUR response: %w", err)
	}
	return &raw, nil
}

// searchEndpoint builds the search URL. The path component is escaped so a
// query never alters the endpoint itself.
func searchEndpoint(query string) string {
	return aurRPCBaseURL + "/search/" + url.PathEscape(query)
}

// infoEndpoint builds a single-package info URL.
func infoEndpoint(pkg string) string {
	return aurRPCBaseURL + "/info/" + url.PathEscape(pkg)
}
