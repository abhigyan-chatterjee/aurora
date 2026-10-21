package lookup

import (
	"errors"
	"sync"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/pacman"
	"github.com/abhigyan-chatterjee/aurora/internal/resolve"
)

// SearchBoth performs parallel searching in pacman and AUR repositories.
func SearchBoth(query string) ([]resolve.PacmanResult, []resolve.AURResult, error, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var pacmanResults []resolve.PacmanResult
	var pacmanErr error

	var aurResults []resolve.AURResult
	var aurErr error

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

// ResolveOne performs parallel exact-match queries against pacman and AUR.
// Official repositories are preferred over AUR.
func ResolveOne(query string) (*resolve.ResolvedPackage, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var pacmanPkg *resolve.PacmanResult
	var pacmanErr error

	var aurPkg *resolve.AURResult
	var aurErr error

	go func() {
		defer wg.Done()
		pacmanPkg, pacmanErr = pacman.SearchPacmanExact(query)
	}()

	go func() {
		defer wg.Done()
		aurPkg, aurErr = aur.SearchAURExact(query)
	}()

	wg.Wait()

	res := &resolve.ResolvedPackage{
		Query: query,
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

	if err := errors.Join(pacmanErr, aurErr); err != nil {
		return nil, err
	}

	res.ChosenSource = resolve.SourceUnknown
	return res, nil
}

// ResolveBatch resolves multiple package queries concurrently in input order.
func ResolveBatch(queries []string) []*resolve.ResolvedPackage {
	results := make([]*resolve.ResolvedPackage, len(queries))
	var wg sync.WaitGroup

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, query string) {
			defer wg.Done()
			res, err := ResolveOne(query)
			if res != nil {
				results[idx] = res
			} else {
				results[idx] = &resolve.ResolvedPackage{
					Query:        query,
					ChosenSource: resolve.SourceUnknown,
					Err:          err,
				}
			}
		}(i, q)
	}

	wg.Wait()
	return results
}
