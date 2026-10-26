// Package config loads Aurora's user configuration.
//
// The file is a TOML subset: top-level `key = value` pairs, one per line, with
// `#` comments. That is deliberately small. Aurora needs nine settings, and a
// full TOML dependency for nine settings is not a trade worth making.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the merged user configuration. Zero values are never used directly;
// call Default and layer the file on top.
type Config struct {
	// Concise suppresses teaching explanations and notes.
	Concise bool
	// Quiet suppresses informational output entirely.
	Quiet bool
	// Yes skips Aurora's confirmation prompts.
	Yes bool
	// InteractivePacman drops --noconfirm from pacman invocations so pacman
	// asks its own questions. Off by default: Aurora has already asked.
	InteractivePacman bool
	// Parallelism caps concurrent subprocesses for version and metadata checks.
	Parallelism int
	// CacheDir overrides ~/.cache/aurora.
	CacheDir string
	// DefaultSource is the default for --from: any, official, or aur.
	DefaultSource string
	// PKGBUILDDiff shows a PKGBUILD diff before rebuilding an AUR package.
	PKGBUILDDiff bool
	// AURDeps resolves and orders AUR dependencies before building.
	AURDeps bool
}

// Default returns the configuration used when no file exists.
func Default() Config {
	return Config{
		Concise:           false,
		Quiet:             false,
		Yes:               false,
		InteractivePacman: false,
		Parallelism:       8,
		CacheDir:          "",
		DefaultSource:     "any",
		PKGBUILDDiff:      false,
		AURDeps:           true,
	}
}

// Path returns the location of the config file, honouring XDG_CONFIG_HOME.
func Path() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "aurora", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "aurora", "config.toml"), nil
}

// Load reads the config file if it exists and returns the merged result.
// A missing file is not an error; it yields Default().
func Load() (Config, error) {
	cfg := Default()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("could not read %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}

		key, rawValue, found := strings.Cut(line, "=")
		if !found {
			return cfg, fmt.Errorf("%s:%d: expected `key = value`, got %q", path, lineNo, line)
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)

		if err := cfg.set(key, rawValue); err != nil {
			return cfg, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, fmt.Errorf("could not read %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Validate checks that every value is usable.
func (c Config) Validate() error {
	if c.Parallelism < 1 || c.Parallelism > 64 {
		return fmt.Errorf("parallelism must be between 1 and 64, got %d", c.Parallelism)
	}
	switch strings.ToLower(c.DefaultSource) {
	case "any", "official", "repo", "aur":
	default:
		return fmt.Errorf("default_source must be one of any, official, repo, aur; got %q", c.DefaultSource)
	}
	return nil
}

func (c *Config) set(key, rawValue string) error {
	switch key {
	case "concise":
		return setBool(&c.Concise, key, rawValue)
	case "quiet":
		return setBool(&c.Quiet, key, rawValue)
	case "yes":
		return setBool(&c.Yes, key, rawValue)
	case "interactive_pacman":
		return setBool(&c.InteractivePacman, key, rawValue)
	case "pkgbuild_diff":
		return setBool(&c.PKGBUILDDiff, key, rawValue)
	case "aur_deps":
		return setBool(&c.AURDeps, key, rawValue)
	case "parallelism":
		n, err := strconv.Atoi(rawValue)
		if err != nil {
			return fmt.Errorf("parallelism expects an integer, got %q", rawValue)
		}
		c.Parallelism = n
		return nil
	case "cache_dir":
		c.CacheDir = unquote(rawValue)
		return nil
	case "default_source":
		c.DefaultSource = unquote(rawValue)
		return nil
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
}

func setBool(dst *bool, key, raw string) error {
	switch strings.ToLower(unquote(raw)) {
	case "true", "yes", "on", "1":
		*dst = true
		return nil
	case "false", "no", "off", "0":
		*dst = false
		return nil
	default:
		return fmt.Errorf("%s expects true or false, got %q", key, raw)
	}
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// stripComment removes a trailing # comment that is not inside a quoted string.
func stripComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		switch ch := line[i]; {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '"' || ch == '\'':
			quote = ch
		case ch == '#':
			return line[:i]
		}
	}
	return line
}
