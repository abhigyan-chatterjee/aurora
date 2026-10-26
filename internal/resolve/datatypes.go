package resolve

// PackageSource identifies where a package was found.
type PackageSource int

const (
	SourceUnknown PackageSource = iota
	SourceOfficial
	SourceAUR
)

// AURResult is one package as reported by the AUR RPC.
type AURResult struct {
	Name         string
	Version      string
	Description  string
	URL          string
	LastModified int64
	NumVotes     int
	Maintainer   string
	// OutOfDate is a Unix timestamp when the AUR maintainer flagged the package
	// as out of date, or 0 when it is current.
	OutOfDate int64
}

// IsOutOfDate reports whether the AUR has flagged this package as stale.
func (r AURResult) IsOutOfDate() bool { return r.OutOfDate > 0 }

// PacmanResult is one package as reported by a sync database.
type PacmanResult struct {
	Name        string
	Version     string
	Description string
	Repository  string
}

// ResolvedPackage is the outcome of looking one query up across both sources.
type ResolvedPackage struct {
	Query        string
	PacmanResult *PacmanResult
	AURResult    *AURResult
	ChosenSource PackageSource

	// Err means resolution failed: nothing usable was found and at least one
	// source errored. Callers must not act on a result carrying Err.
	Err error

	// Warnings are non-fatal source failures. The canonical case is the
	// official lookup erroring while the AUR answered: the AUR result is usable
	// but the caller should tell the user that official repositories were not
	// fully checked before installing from the AUR.
	Warnings []error
}
