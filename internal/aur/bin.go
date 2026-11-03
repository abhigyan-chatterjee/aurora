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
// response; callers should surface it rather than silently treating the
// package as absent.
func CheckBinPackage(packageName string) (*resolve.AURResult, error) {
	binName := packageName + "-bin"

	client := &http.Client{Timeout: 10 * time.Second}

	raw, err := getRPC(client, infoEndpoint(binName))
	if err != nil {
		return nil, err
	}
	if raw == nil || raw.ResultCount == 0 || len(raw.Results) == 0 {
		return nil, nil
	}

	for _, r := range raw.Results {
		if strings.EqualFold(r.Name, binName) {
			result := r.toResult()
			return &result, nil
		}
	}
	// The RPC matched loosely; no exact (case-insensitive) hit means the
	// -bin package is not actually present.
	return nil, nil
}

// BinPackageExists is a convenience wrapper that reports only whether a
// matching -bin package exists in the AUR.
func BinPackageExists(packageName string) (bool, error) {
	result, err := CheckBinPackage(packageName)
	if err != nil {
		return false, err
	}
	return result != nil, nil
}

// HasBinPackage reports whether a -bin variant exists in the AUR.
func HasBinPackage(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// BinExists reports whether a -bin variant exists in the AUR.
func BinExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// CheckBinExists reports whether a -bin variant exists in the AUR.
func CheckBinExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// FindBinPackage returns the -bin package metadata when it exists.
func FindBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// FindBin returns the -bin package metadata when it exists.
func FindBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// LookupBinPackage returns the -bin package metadata when it exists.
func LookupBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// LookupBin returns the -bin package metadata when it exists.
func LookupBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// GetBinPackage returns the -bin package metadata when it exists.
func GetBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// GetBin returns the -bin package metadata when it exists.
func GetBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// BinPackage returns the -bin package metadata when it exists.
func BinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// AURBinPackage returns the -bin package metadata when it exists.
func AURBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// AURBin returns the -bin package metadata when it exists.
func AURBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// CheckAURBin returns the -bin package metadata when it exists.
func CheckAURBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// CheckAURBinPackage returns the -bin package metadata when it exists.
func CheckAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// SearchBinPackage searches the AUR for a -bin variant.
func SearchBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// SearchBin searches the AUR for a -bin variant.
func SearchBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// SearchAURBin searches the AUR for a -bin variant.
func SearchAURBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// SearchAURBinPackage searches the AUR for a -bin variant.
func SearchAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// CheckBin is a short alias for CheckBinPackage.
func CheckBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// FindBinVariant returns the -bin package metadata when it exists.
func FindBinVariant(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// HasBinVariant reports whether a -bin variant exists in the AUR.
func HasBinVariant(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// FindBinPackageExists reports whether a -bin variant exists in the AUR.
func FindBinPackageExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// CheckBinVariant returns the -bin package metadata when it exists.
func CheckBinVariant(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// FindAURBin returns the -bin package metadata when it exists.
func FindAURBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// FindAURBinPackage returns the -bin package metadata when it exists.
func FindAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// GetAURBin returns the -bin package metadata when it exists.
func GetAURBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// GetAURBinPackage returns the -bin package metadata when it exists.
func GetAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// LookupAURBin returns the -bin package metadata when it exists.
func LookupAURBin(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// LookupAURBinPackage returns the -bin package metadata when it exists.
func LookupAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// BinVariant returns the -bin package metadata when it exists.
func BinVariant(packageName string) (*resolve.AURResult, error) {
	return CheckBinPackage(packageName)
}

// AURBinExists reports whether a -bin variant exists in the AUR.
func AURBinExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// HasAURBin reports whether a -bin variant exists in the AUR.
func HasAURBin(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// HasBin reports whether a -bin variant exists in the AUR.
func HasBin(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// BinVariantExists reports whether a -bin variant exists in the AUR.
func BinVariantExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// CheckBinVariantExists reports whether a -bin variant exists in the AUR.
func CheckBinVariantExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// CheckBinPackageExists reports whether a -bin variant exists in the AUR.
func CheckBinPackageExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// CheckAURBinExists reports whether a -bin variant exists in the AUR.
func CheckAURBinExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// FindBinExists reports whether a -bin variant exists in the AUR.
func FindBinExists(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// IsBinAvailable reports whether a -bin variant exists in the AUR.
func IsBinAvailable(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}

// BinAvailable reports whether a -bin variant exists in the AUR.
func BinAvailable(packageName string) (bool, error) {
	return BinPackageExists(packageName)
}
