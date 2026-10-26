// Package run wraps every external process Aurora spawns.
//
// Two invariants live here and nowhere else:
//
//  1. Every subprocess has a deadline. A pacman invocation blocked on a
//     database lock, or a git clone against a stalled mirror, must fail with
//     a clear error instead of hanging the tool forever.
//  2. "exit code 1 means no matches" is a pacman query convention, so the
//     distinction between "ran and found nothing" and "did not run" is made
//     in exactly one place.
package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// QueryTimeout bounds read-only commands: pacman queries, vercmp, AUR lookups.
const QueryTimeout = 60 * time.Second

// BuildTimeout bounds operations that legitimately run for a long time: a full
// system upgrade, a package build, a self-update download.
const BuildTimeout = 2 * time.Hour

// ErrNotFound reports that the executable is not on PATH.
var ErrNotFound = exec.ErrNotFound

// Exists reports whether name resolves on PATH.
func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// LookPath returns the absolute path to name, or an error naming it.
func LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH: %w", name, err)
	}
	return path, nil
}

// IsExitCode reports whether err is an *exec.ExitError carrying the given code.
func IsExitCode(err error, code int) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == code
	}
	return false
}

// Output runs the command with a deadline and returns its stdout.
//
// A nonzero exit is returned as an error, except that callers should use
// OutputOrEmpty for pacman queries where exit 1 is the documented "no matches".
func Output(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout.Bytes(), fmt.Errorf("`%s %v` timed out after %s", name, args, timeout)
		}
		msg := formatStderr(stderr.String())
		if msg != "" {
			return stdout.Bytes(), fmt.Errorf("`%s %v` failed: %w: %s", name, args, err, msg)
		}
		return stdout.Bytes(), fmt.Errorf("`%s %v` failed: %w", name, args, err)
	}
	return stdout.Bytes(), nil
}

// OutputOrEmpty runs a query where exit code 1 means "no results".
//
// It returns (output, nil) for a successful run and for exit 1, and a real
// error for anything else (missing binary, timeout, broken database).
func OutputOrEmpty(timeout time.Duration, name string, args ...string) ([]byte, error) {
	out, err := Output(timeout, name, args...)
	if err == nil {
		return out, nil
	}
	if IsExitCode(err, 1) {
		return out, nil
	}
	return out, err
}

// Interactive runs a command with stdin, stdout and stderr attached, so an
// upgrade or a build can prompt the user directly.
func Interactive(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("`%s %v` timed out after %s", name, args, timeout)
		}
		return fmt.Errorf("`%s %v` failed: %w", name, args, err)
	}
	return nil
}

// InteractiveDir is Interactive with a working directory, used for makepkg.
func InteractiveDir(timeout time.Duration, dir, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("`%s %v` timed out after %s", name, args, timeout)
		}
		return fmt.Errorf("`%s %v` failed: %w", name, args, err)
	}
	return nil
}

// Parallel runs fn for every index in [0,n) using at most workers goroutines
// at once and returns the first error encountered.
//
// This exists because a goroutine per package, each shelling out to a process,
// is a process storm on a machine with a few hundred packages.
func Parallel(n, workers int, fn func(i int) error) error {
	if n <= 0 {
		return nil
	}
	if workers < 1 {
		workers = 1
	}
	if workers > n {
		workers = n
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, workers)
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(idx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	return firstErr
}

// formatStderr keeps an error message readable when it is embedded in another
// error. The limit is in bytes, so a multi-byte character at the boundary may be
// split; that is cosmetic in a diagnostic and not worth a rune scan.
func formatStderr(s string) string {
	s = strings.TrimSpace(s)
	const max = 400
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
