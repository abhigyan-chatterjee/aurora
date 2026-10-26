package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected missing config file to be tolerated, got: %v", err)
	}
	if cfg.Parallelism != 8 {
		t.Errorf("expected default parallelism 8, got %d", cfg.Parallelism)
	}
	if cfg.DefaultSource != "any" {
		t.Errorf("expected default source any, got %q", cfg.DefaultSource)
	}
	if !cfg.AURDeps {
		t.Errorf("expected AUR dependency ordering on by default")
	}
}

func TestLoad_FullFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "aurora"), 0755); err != nil {
		t.Fatal(err)
	}

	body := `# Aurora settings
concise = true
quiet = false
yes = true          # trailing comment
interactive_pacman = false
parallelism = 4
cache_dir = "/tmp/aurora-cache"
default_source = "official"
pkgbuild_diff = true
aur_deps = false
`
	if err := os.WriteFile(filepath.Join(dir, "aurora", "config.toml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if !cfg.Concise || !cfg.Yes || !cfg.PKGBUILDDiff {
		t.Errorf("expected concise, yes and pkgbuild_diff true, got %+v", cfg)
	}
	if cfg.AURDeps {
		t.Errorf("expected aur_deps false, got true")
	}
	if cfg.Parallelism != 4 {
		t.Errorf("expected parallelism 4, got %d", cfg.Parallelism)
	}
	if cfg.CacheDir != "/tmp/aurora-cache" {
		t.Errorf("expected cache_dir /tmp/aurora-cache, got %q", cfg.CacheDir)
	}
	if cfg.DefaultSource != "official" {
		t.Errorf("expected default_source official, got %q", cfg.DefaultSource)
	}
}

func TestLoad_RejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "aurora"), 0755); err != nil {
		t.Fatal(err)
	}
	body := "paralelism = 4\n"
	if err := os.WriteFile(filepath.Join(dir, "aurora", "config.toml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("expected an unknown key to be reported, got nil error")
	}
}

func TestLoad_RejectsBadValues(t *testing.T) {
	cases := map[string]string{
		"bad bool":      "quiet = maybe\n",
		"bad int":       "parallelism = lots\n",
		"bad source":    "default_source = \"mars\"\n",
		"out of range":  "parallelism = 500\n",
		"missing value": "quiet\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			if err := os.MkdirAll(filepath.Join(dir, "aurora"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "aurora", "config.toml"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(); err == nil {
				t.Fatalf("expected %q to be rejected", body)
			}
		})
	}
}

func TestStripComment(t *testing.T) {
	cases := []struct{ in, want string }{
		{`quiet = true # comment`, `quiet = true `},
		{`cache_dir = "/tmp/a#b"`, `cache_dir = "/tmp/a#b"`},
		{`# whole line`, ``},
		{`no comment`, `no comment`},
	}
	for _, tc := range cases {
		if got := stripComment(tc.in); got != tc.want {
			t.Errorf("stripComment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
