package lookup

import (
	"errors"
	"sync"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// SearchBoth searches the official repositories and the AUR concurrently.
func SearchBoth(query string) ([]resolve.PacmanResult, []resolve.AURResult, error, error) {
	var (
		wg            sync.WaitGroup
		pacmanResults []resolve.PacmanResult
		pacmanErr     error
		aurResults    []resolve.AURResult
		aurErr        error
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		pacmanResults, pacmanErr = pacman.SearchPacman(query)
	}()
	go func() {
		defer wg.Done()
		aurResults, aurErr = aur.SearchAUR(query)
	}()

	wg.Wait()
	return pacmanResults, aurResults, pacmanErr, aurErr
}

// ResolveOne resolves one exact package name, preferring official repositories.
//
// The returned error is reserved for "nothing usable was found and a source
// failed". When one source answers and the other errors, the answer is
// returned with the failure recorded in Warnings. The old behaviour of
// discarding the official error meant `aurora install foo` could build from the
// AUR while an official `foo` existed, with no indication that the official
// lookup had not worked.
func ResolveOne(query string) (*resolve.ResolvedPackage, error) {
	var (
		wg        sync.WaitGroup
		pacmanPkg *resolve.PacmanResult
		pacmanErr error
		aurPkg    *resolve.AURResult
		aurErr    error
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		pacmanPkg, pacmanErr = pacman.SearchPacmanExact(query)
	}()
	go func() {
		defer wg.Done()
		aurPkg, aurErr = aur.SearchAURExact(query)
	}()

	wg.Wait()

	res := &resolve.ResolvedPackage{Query: query, ChosenSource: resolve.SourceUnknown}

	if pacmanErr != nil {
		res.Warnings = append(res.Warnings, pacmanErr)
	}
	if aurErr != nil {
		res.Warnings = append(res.Warnings, aurErr)
	}

	if pacmanPkg != nil {
		res.PacmanResult = pacmanPkg
		res.ChosenSource = resolve.SourceOfficial
		return res, nil
	}

	if aurPkg != nil {
		res.AURResult = aurPkg
		res.ChosenSource = resolve.SourceAUR
		return res, nil
	}

	// Nothing found. If every source errored this is a resolution failure
	// rather than a genuine "package does not exist".
	if pacmanErr != nil && aurErr != nil {
		res.Err = errors.Join(pacmanErr, aurErr)
		return res, res.Err
	}

	return res, nil
}

// ResolveBatch resolves many queries concurrently and returns results in input
// order. A resolution failure is recorded on the result rather than aborting
// the batch, so one bad name cannot hide the others.
func ResolveBatch(queries []string) []*resolve.ResolvedPackage {
	results := make([]*resolve.ResolvedPackage, len(queries))
	var wg sync.WaitGroup

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, query string) {
			defer wg.Done()
			res, err := ResolveOne(query)
			if res == nil {
				res = &resolve.ResolvedPackage{Query: query, ChosenSource: resolve.SourceUnknown}
			}
			if err != nil && res.Err == nil {
				res.Err = err
			}
			results[idx] = res
		}(i, q)
	}

	wg.Wait()
	return results
}
