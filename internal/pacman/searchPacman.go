package pacman

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
	"github.com/abhigyan-chatterjee/aurora/internal/run"
)

var (
	pacmanSearchHeaderRe = regexp.MustCompile(`^([^/]+)/([^\s]+)\s+(.+)$`)
	pacmanExactFieldRe   = regexp.MustCompile(`^([^:]+)\s*:\s*(.+)$`)
)

// ParsePacmanSearchOutput parses the standard output of 'pacman -Ss' into
// structured results, keeping only packages whose name contains the query.
//
// Package descriptions appear on indented lines after their header, so the
// current result index is cleared when an unmatched header turns up. Otherwise
// a filtered-out package's description would attach to the previous match.
func ParsePacmanSearchOutput(output string, packageName string) []resolve.PacmanResult {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil
	}

	var results []resolve.PacmanResult
	currentIdx := -1

	for _, line := range lines {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if currentIdx >= 0 && results[currentIdx].Description == "" {
				results[currentIdx].Description = strings.TrimSpace(line)
			}
			continue
		}

		matches := pacmanSearchHeaderRe.FindStringSubmatch(line)
		if matches == nil {
			currentIdx = -1
			continue
		}

		pkgName := matches[2]

		if !strings.Contains(strings.ToLower(pkgName), strings.ToLower(packageName)) {
			currentIdx = -1
			continue
		}

		results = append(results, resolve.PacmanResult{
			Repository: matches[1],
			Name:       pkgName,
			Version:    matches[3],
		})
		currentIdx = len(results) - 1
	}

	if len(results) == 0 {
		return nil
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Name != results[j].Name {
			return results[i].Name < results[j].Name
		}
		return results[i].Repository < results[j].Repository
	})

	return results
}

// SearchPacman runs a repository search.
func SearchPacman(packageName string) ([]resolve.PacmanResult, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Ss", packageName)
	if err != nil {
		return nil, fmt.Errorf("error running pacman -Ss: %w", err)
	}
	return ParsePacmanSearchOutput(string(out), packageName), nil
}

// ParsePacmanExactOutput parses the output of 'pacman -Si' into one result.
func ParsePacmanExactOutput(output string, packageName string) *resolve.PacmanResult {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil
	}

	var pkg resolve.PacmanResult
	for _, line := range lines {
		if line == "" && pkg.Name != "" {
			break
		}
		matches := pacmanExactFieldRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		field := strings.TrimSpace(matches[1])
		value := strings.TrimSpace(matches[2])

		switch field {
		case "Name":
			pkg.Name = value
		case "Version":
			pkg.Version = value
		case "Description":
			pkg.Description = value
		case "Repository":
			pkg.Repository = value
		}
	}

	if pkg.Name == "" || !strings.EqualFold(pkg.Name, packageName) {
		return nil
	}

	return &pkg
}

// SearchPacmanExact looks a package up by exact name in the sync databases.
func SearchPacmanExact(packageName string) (*resolve.PacmanResult, error) {
	out, err := run.OutputOrEmpty(run.QueryTimeout, "pacman", "-Si", packageName)
	if err != nil {
		return nil, fmt.Errorf("error running pacman -Si: %w", err)
	}
	return ParsePacmanExactOutput(string(out), packageName), nil
}
