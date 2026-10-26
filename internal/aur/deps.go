package aur

import (
	"fmt"
	"sync"

	"github.com/abhigyan-chatterjee/aurora/internal/run"
)

// PackageFetcher returns the PKGBUILD text for a package name. It returns an
// error when the package is not in the AUR or cannot be read.
type PackageFetcher func(name string) (string, error)

// maxDependencyNodes bounds how many packages the resolver will walk, so a
// package with a pathological dependency tree cannot turn an install into
// hundreds of network requests.
const maxDependencyNodes = 120

// maxDependencyDepth bounds how many levels are followed.
const maxDependencyDepth = 5

// ResolveBuildOrder returns the requested AUR packages ordered so that every
// AUR dependency is built before the package that needs it.
//
// Dependencies already provided by the official repositories are left to
// pacman and are not part of the graph. A dependency that cannot be fetched
// from the AUR is skipped, which covers virtual dependencies such as
// java-runtime and packages a PKGBUILD only names conditionally.
//
// Ordering is best effort: resolution problems are returned as warnings and the
// caller still receives a usable order.
func ResolveBuildOrder(requested []string, isOfficial func(string) bool, workers int) ([]string, []error) {
	if len(requested) == 0 {
		return nil, nil
	}
	if isOfficial == nil {
		isOfficial = func(string) bool { return false }
	}
	return resolveBuildOrder(requested, FetchPKGBUILD, isOfficial, workers)
}

// resolveBuildOrder is ResolveBuildOrder with an injected fetcher, so the
// graph logic is testable without network access.
func resolveBuildOrder(requested []string, fetch PackageFetcher, isOfficial func(string) bool, workers int) ([]string, []error) {
	var (
		mu           sync.Mutex
		graph        = make(map[string][]string)
		warnings     []error
		seen         = make(map[string]bool)
		requestedSet = make(map[string]bool)
	)

	for _, name := range requested {
		requestedSet[name] = true
	}

	addWarning := func(err error) {
		mu.Lock()
		warnings = append(warnings, err)
		mu.Unlock()
	}

	// Breadth-first by level so each level can be fetched concurrently.
	level := dedupeNames(requested)
	for _, name := range level {
		seen[name] = true
	}

	for depth := 0; depth < maxDependencyDepth && len(level) > 0; depth++ {
		if len(seen) > maxDependencyNodes {
			addWarning(fmt.Errorf("dependency resolution stopped after %d packages", maxDependencyNodes))
			break
		}

		type fetchResult struct {
			name string
			deps []string
			err  error
		}
		results := make([]fetchResult, len(level))

		_ = run.Parallel(len(level), workers, func(i int) error {
			name := level[i]
			content, err := fetch(name)
			results[i] = fetchResult{name: name, err: err}
			if err != nil {
				return nil
			}
			info := ParsePKGBUILD(content)
			for _, dep := range append(append(append([]string{}, info.Depends...), info.MakeDepends...), info.CheckDepends...) {
				bare := StripVersionConstraint(dep)
				if bare == "" || isOfficial(bare) {
					continue
				}
				results[i].deps = append(results[i].deps, bare)
			}
			return nil
		})

		var next []string
		for _, r := range results {
			if r.err != nil {
				if !requestedSet[r.name] {
					// A dependency that is neither in an official repository nor
					// in the AUR is almost always a virtual provide
					// (java-runtime, libgl) that pacman resolves itself. Naming
					// it would suggest Aurora is about to do something with it.
					addWarning(fmt.Errorf("dependency %q is not an AUR package; leaving it to pacman", r.name))
				} else {
					addWarning(fmt.Errorf("could not read the PKGBUILD for %s: %w", r.name, r.err))
				}
				continue
			}
			graph[r.name] = dedupeNames(r.deps)
			for _, dep := range graph[r.name] {
				if !seen[dep] {
					seen[dep] = true
					next = append(next, dep)
				}
			}
		}
		level = dedupeNames(next)
	}

	// Drop edges pointing at packages that could not be fetched. Without this,
	// an unresolvable dependency lands in the build order and Aurora tries to
	// build a package that does not exist.
	for name, deps := range graph {
		kept := make([]string, 0, len(deps))
		for _, dep := range deps {
			if _, ok := graph[dep]; ok {
				kept = append(kept, dep)
			}
		}
		graph[name] = kept
	}

	// Post-order DFS emits dependencies before dependents.
	const (
		unvisited = 0
		visiting  = 1
		done      = 2
	)
	state := make(map[string]int, len(graph))
	order := make([]string, 0, len(graph))

	var visit func(string)
	visit = func(name string) {
		switch state[name] {
		case done:
			return
		case visiting:
			addWarning(fmt.Errorf("dependency cycle involving %s; build order may be imperfect", name))
			return
		}
		state[name] = visiting
		for _, dep := range graph[name] {
			visit(dep)
		}
		state[name] = done
		order = append(order, name)
	}

	for _, name := range dedupeNames(requested) {
		visit(name)
	}

	// A requested package whose PKGBUILD could not be fetched was never added
	// to the graph, so keep it in the list and let the build fail loudly.
	for _, name := range dedupeNames(requested) {
		if state[name] == unvisited {
			order = append(order, name)
		}
	}

	return order, dedupeErrors(warnings)
}

func dedupeNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func dedupeErrors(errs []error) []error {
	seen := make(map[string]bool, len(errs))
	out := make([]error, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		key := err.Error()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, err)
	}
	return out
}
