# Aurora v0.4 🎉

**Aurora** is a modern package workflow assistant for Arch-based Linux that combines the raw AUR power of **Paru** with the memorable simplicity of **apt**. It prioritizes official repositories over the AUR, explains every operation in plain English, and keeps your system safe and clean.

---

## What's New in v0.4

v0.4 adds self-updating, library-safe system upgrade ordering, chunked AUR RPC queries, and stricter error propagation across all commands:

### 🔄 Self-Update & Reliability (v0.4)
- **Self-Update (`aurora selfupdate`)** — Checks GitHub Releases for newer versions, verifies the binary's SHA256 digest, and atomically updates the `aurora` executable (with `go install` fallback).
- **Library-Safe Upgrade Order (`aurora update`)** — Runs `sudo pacman -Syu` *first* before rebuilding outdated AUR packages so AUR builds link against the newest system libraries (and aborts AUR rebuilds if the system upgrade fails).
- **Large-System AUR Batching** — Automatically chunks AUR RPC `info` queries (150 packages per request) while preserving partial results and checking versions concurrently.
- **Explicit Error Propagation & Root Guard** — Blocks accidental `sudo` execution when building AUR packages (`makepkg` requirement) and returns non-zero exit codes on command failures for reliable scripting.

### ⚡ Paru-Style Power & Source Intelligence
- **PKGBUILD Inspection (`aurora inspect <pkg>`)** — Fetch and review PKGBUILD build recipes directly from AUR cgit before building or downloading.
- **Pre-Build Review (`aurora install --inspect <pkg>`)** — Inspect PKGBUILD scripts interactively prior to compiling AUR software with `makepkg`.
- **Source Filtering (`--from official|repo|aur`, `-O`, `-A`)** — Restrict operations cleanly to official repositories or AUR across `install`, `search`, `inspect`, and `info`.
- **Concurrent Lookups (Goroutines)** — Parallel queries across pacman and AUR cut search and resolution latency in half.

### 🐧 Apt-Style Simplicity & Lifecycle Management
- **`aurora autoremove`** — Discovers and removes leftover orphan dependencies (`pacman -Qtdq`) safely with `sudo pacman -Rns --noconfirm`.
- **`aurora list`** — Apt-style package query with filters: `--upgradable`, `--orphans`, `--official`, `--foreign` / `--aur`, and `--all`.
- **`aurora clean`** — Purges temporary AUR build artifacts in `~/.cache/aurora` (`--cache`) or cleans pacman's cache (`--pacman`, `--all`) with `--dry-run` inspection.

### 📚 Teaching Mode & Friction Reduction
- **`aurora explain [topic]`** — Plain-English concept explorer for Arch Linux package management (`-Syu`, `-Rns`, `aur`, `pkgbuild`, `makepkg`, `partial-upgrades`, `orphans`).
- **Global Automation (`-y, --yes`)** — Auto-confirm Aurora's prompt layer for unattended scripts.
- **Numbered Selection Picker** — Select packages from search and suggestion lists by typing their index (`1`, `2`) or name.
- **Did You Mean Suggestions** — Automatic typo tolerance and suggestions on missing packages.

---

## Commands

| Command | What it does |
|---|---|
| `aurora explain [topic]` | Learn Arch Linux concepts, pacman flags, and AUR workflows |
| `aurora inspect <package>` | Inspect PKGBUILD recipes or official package build metadata (alias: `pkgbuild`) |
| `aurora search <query...> [--from]` | Search official repos and AUR concurrently |
| `aurora info <package...> [--from]` | Show detailed package information and source resolution |
| `aurora install <package...> [--from] [-i]` | Install packages in batch with preview, confirmation, and optional PKGBUILD review |
| `aurora remove <package...>` | Remove packages safely with orphan cleanup (`-Rns`) |
| `aurora update` | Perform full system upgrade (`-Syu`), then rebuild outdated AUR packages |
| `aurora autoremove` | Scan and remove unneeded orphan dependencies (apt-style) |
| `aurora list [query] [--flags]` | List packages by category (`--upgradable`, `--orphans`, `--official`, `--foreign`, `--all`) |
| `aurora clean [--dry-run] [--pacman] [--all]` | Reclaim disk space by purging AUR build artifacts and/or pacman package caches |
| `aurora selfupdate [--check] [--force]` | Update Aurora itself from GitHub Releases (or `go install` fallback) |

### Global Flags

- **`-y, --yes`**: Skip Aurora's confirmation prompts (pacman/sudo still prompt).
- **`-c, --concise`**: Enable concise output mode (suppress extra teaching explanations and notes).
- **`-v, --version`**: Show Aurora version (`0.4.0`).

---

## Philosophy

- **Pacman is King** — Aurora delegates to `pacman` and `makepkg`. It orchestrates, never replaces.
- **Transparency First** — Every destructive command is previewed with a plain-English explanation before it runs.
- **No Jargon Without Explanation** — When Aurora uses flags like `-Syu` or `-Rns`, `aurora explain` helps you understand what they mean.
- **Official Wins** — Official repositories are always preferred. AUR is an explicit choice, not a hidden default.

---

## Installation

### 1. Download Pre-built Binary (Recommended)

Download the latest binary from the [releases page](https://github.com/abhigyan-chatterjee/aurora/releases), then:

```bash
sudo cp aurora /usr/local/bin/
```

### 2. Install with Go

Requires Go 1.25+:

```bash
go install github.com/abhigyan-chatterjee/aurora@latest
```

### 3. Build from Source

```bash
git clone https://github.com/abhigyan-chatterjee/aurora.git
cd aurora
go build -o build/aurora main.go
sudo cp build/aurora /usr/local/bin/
```

---

## Quick Start

```bash
# Teaching & Exploration
aurora explain -Syu         # Learn what -Syu means and why it's important
aurora inspect yay          # View PKGBUILD and audit build script

# Search & Info
aurora search neovim git    # Search official repos + AUR in parallel
aurora search yay --from aur # Search only the AUR
aurora info neovim          # Detailed package details

# Installation & Upgrades
aurora install neovim git   # Install multiple packages with 1 confirmation
aurora install yay -i       # Review PKGBUILD before building from AUR
aurora update               # Full system upgrade (-Syu), then rebuild outdated AUR packages
aurora selfupdate           # Update Aurora itself to the latest GitHub Release

# System Maintenance (Apt-style)
aurora autoremove           # Remove unused orphan dependencies
aurora list --upgradable    # List packages with available updates
aurora list --foreign       # List installed AUR packages
aurora clean --dry-run      # Check how much cache space can be reclaimed
aurora clean                # Purge ~/.cache/aurora build artifacts
aurora clean --pacman       # Prune uninstalled packages from pacman cache
aurora remove neovim        # Safe removal with dependency cleanup
```

---

## Roadmap

- **v0.4 — Self-Update, Safe Upgrade Ordering & Hardened CLI** (Complete!)
  - Self-updater (`aurora selfupdate [--check] [--force]`) with SHA256 digest verification and `go install` fallback
  - Paru-style `update` ordering (`pacman -Syu` runs first before AUR rebuilds)
  - 150-package chunked AUR RPC queries and concurrent `vercmp` checks
  - Isolated `clean` scope flags (`--cache`, `--pacman`, `--all`), non-root AUR build guard, and `RunE` exit codes
  - Automated Arch Linux CI and semantic release pipeline (`staging` -> `main`)
- **v0.3 — Paru Power + Apt Simplicity** (Complete!)
  - Teaching mode (`explain` command, concise mode)
  - Concurrency (parallel lookups with goroutines)
  - Multi-argument batch operations across commands
  - Source Intelligence (`--from official|aur`, PKGBUILD `inspect` command, pre-build review)
  - System Lifecycle (`autoremove` for orphans, `list` queries, `clean` cache maintenance)
  - Unit test suite and clean internal architecture (`internal/ui`)
- **Future Explorations**:
  - Optional pacman mirror freshness check
  - Tab completion scripts for bash/zsh/fish

---

## Contributing

We value clarity and transparency. If you have an idea, open an issue first — we prioritize changes that keep Aurora transparent and predictable.
