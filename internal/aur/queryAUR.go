package aur

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// aurInfoBatchSize caps how many packages go into one RPC request.
//
// The AUR RPC interface encodes every name as its own arg[]= query parameter,
// so an oversized batch exceeds URL length limits on a machine with many
// foreign packages installed.
const aurInfoBatchSize = 150

// GetAURInfoBatch fetches metadata for many packages, chunked to stay inside
// URL limits. Chunks are independent: one failing chunk does not discard the
// results of the others.
func GetAURInfoBatch(packageNames []string) ([]resolve.AURResult, error) {
	if len(packageNames) == 0 {
		return nil, nil
	}

	var (
		all       []resolve.AURResult
		chunkErrs []error
	)
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
	args := make([]string, 0, len(packageNames))
	for _, name := range packageNames {
		args = append(args, "arg[]="+url.QueryEscape(name))
	}
	endpoint := aurRPCBaseURL + "/info?" + strings.Join(args, "&")

	client := &http.Client{Timeout: 30 * time.Second}

	raw, err := getRPC(client, endpoint)
	if err != nil {
		return nil, err
	}

	results := make([]resolve.AURResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		results = append(results, r.toResult())
	}
	return results, nil
}
