package resolve

type PackageSource int

const (
	SourceUnknown PackageSource = iota
	SourceOfficial
	SourceAUR
)

func (s PackageSource) String() string {
	switch s {
	case SourceOfficial:
		return "official"
	case SourceAUR:
		return "AUR"
	default:
		return "unknown"
	}
}

type AURResult struct {
	Name         string
	Version      string
	Description  string
	URL          string
	LastModified int64
	NumVotes     int
}

type PacmanResult struct {
	Name        string
	Version     string
	Description string
	Repository  string
}

type ResolvedPackage struct {
	Query        string
	PacmanResult *PacmanResult
	AURResult    *AURResult
	ChosenSource PackageSource
	// Err holds a lookup failure (e.g. network or pacman error) so callers can
	// distinguish "could not check sources" from "package not found".
	Err error
}
