package aur

import (
	"net/http"
	"strings"
	"time"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// CheckBinPackage reports whether an AUR package named "<packageName>-bin"
// exists. The lookup goes to the AUR RPC only; official repositories are never
// consulted, so a package that exists only in an official repo is not treated
// as a -bin candidate.
//
// A nil result with a nil error means the -bin package does not exist. A
// non-nil error means the AUR could not be queried or returned a malformed
// response. Install-flow callers treat such errors as "no -bin available"
// and proceed with the originally requested package (fail-open); they do not
// surface the error to the user.
func CheckBinPackage(packageName string) (*resolve.AURResult, error) {
	binName := packageName + "-bin"

	client := &http.Client{Timeout: 2500 * time.Millisecond}

	raw, err := getRPC(client, infoEndpoint(binName))
	if err != nil {
		return nil, err
	}
	if raw == nil || raw.ResultCount == 0 || len(raw.Results) == 0 {
		return nil, nil
	}

	for _, r := range raw.Results {
		if !isValidPackageName(r.Name) {
			continue
		}
		if strings.EqualFold(r.Name, binName) {
			result := r.toResult()
			return &result, nil
		}
	}
	// The RPC matched loosely; no exact (case-insensitive) hit means the
	// -bin package is not actually present.
	return nil, nil
}

// SetRPCBaseURL overrides the AUR RPC base URL and returns the previous value.
// It exists so tests in other packages can point the client at a local server.
func SetRPCBaseURL(u string) string {
	old := aurRPCBaseURL
	aurRPCBaseURL = u
	return old
}
