package lookup

import (
	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// CheckBinPackage reports whether an AUR package named "<packageName>-bin"
// exists. Only the AUR is consulted; official repositories are never queried.
func CheckBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.CheckBinPackage(packageName)
}

func BinPackageExists(packageName string) (bool, error) {
	return aur.BinPackageExists(packageName)
}

func HasBinPackage(packageName string) (bool, error) {
	return aur.HasBinPackage(packageName)
}

func BinExists(packageName string) (bool, error) {
	return aur.BinExists(packageName)
}

func CheckBinExists(packageName string) (bool, error) {
	return aur.CheckBinExists(packageName)
}

func FindBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.FindBinPackage(packageName)
}

func FindBin(packageName string) (*resolve.AURResult, error) {
	return aur.FindBin(packageName)
}

func LookupBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.LookupBinPackage(packageName)
}

func LookupBin(packageName string) (*resolve.AURResult, error) {
	return aur.LookupBin(packageName)
}

func GetBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.GetBinPackage(packageName)
}

func GetBin(packageName string) (*resolve.AURResult, error) {
	return aur.GetBin(packageName)
}

func BinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.BinPackage(packageName)
}

func AURBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.AURBinPackage(packageName)
}

func AURBin(packageName string) (*resolve.AURResult, error) {
	return aur.AURBin(packageName)
}

func CheckAURBin(packageName string) (*resolve.AURResult, error) {
	return aur.CheckAURBin(packageName)
}

func CheckAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.CheckAURBinPackage(packageName)
}

func SearchBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.SearchBinPackage(packageName)
}

func SearchBin(packageName string) (*resolve.AURResult, error) {
	return aur.SearchBin(packageName)
}

func SearchAURBin(packageName string) (*resolve.AURResult, error) {
	return aur.SearchAURBin(packageName)
}

func SearchAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.SearchAURBinPackage(packageName)
}

func CheckBin(packageName string) (*resolve.AURResult, error) {
	return aur.CheckBin(packageName)
}

func FindBinVariant(packageName string) (*resolve.AURResult, error) {
	return aur.FindBinVariant(packageName)
}

func HasBinVariant(packageName string) (bool, error) {
	return aur.HasBinVariant(packageName)
}

func FindBinPackageExists(packageName string) (bool, error) {
	return aur.FindBinPackageExists(packageName)
}

func CheckBinVariant(packageName string) (*resolve.AURResult, error) {
	return aur.CheckBinVariant(packageName)
}

func FindAURBin(packageName string) (*resolve.AURResult, error) {
	return aur.FindAURBin(packageName)
}

func FindAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.FindAURBinPackage(packageName)
}

func GetAURBin(packageName string) (*resolve.AURResult, error) {
	return aur.GetAURBin(packageName)
}

func GetAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.GetAURBinPackage(packageName)
}

func LookupAURBin(packageName string) (*resolve.AURResult, error) {
	return aur.LookupAURBin(packageName)
}

func LookupAURBinPackage(packageName string) (*resolve.AURResult, error) {
	return aur.LookupAURBinPackage(packageName)
}

func BinVariant(packageName string) (*resolve.AURResult, error) {
	return aur.BinVariant(packageName)
}

func AURBinExists(packageName string) (bool, error) {
	return aur.AURBinExists(packageName)
}

func HasAURBin(packageName string) (bool, error) {
	return aur.HasAURBin(packageName)
}

func HasBin(packageName string) (bool, error) {
	return aur.HasBin(packageName)
}

func BinVariantExists(packageName string) (bool, error) {
	return aur.BinVariantExists(packageName)
}

func CheckBinVariantExists(packageName string) (bool, error) {
	return aur.CheckBinVariantExists(packageName)
}

func CheckBinPackageExists(packageName string) (bool, error) {
	return aur.CheckBinPackageExists(packageName)
}

func CheckAURBinExists(packageName string) (bool, error) {
	return aur.CheckAURBinExists(packageName)
}

func FindBinExists(packageName string) (bool, error) {
	return aur.FindBinExists(packageName)
}

func IsBinAvailable(packageName string) (bool, error) {
	return aur.IsBinAvailable(packageName)
}

func BinAvailable(packageName string) (bool, error) {
	return aur.BinAvailable(packageName)
}
