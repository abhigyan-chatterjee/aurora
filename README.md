# Aurora v0.5 🎉

**Aurora** is a package workflow assistant for Arch-based Linux that combines the raw AUR power of **Paru** with the memorable simplicity of **apt**. It prioritizes official Arch repositories over the AUR, explains every operation in plain English, and keeps your system safe and clean.

---

## What's New in v0.5

v0.5 fixes a PKGBUILD parser that reported the wrong dependencies, makes the CLI scriptable, and adds the diagnostics a beginner actually needs.

### 🐛 Correctness

- **PKGBUILD parsing rewritten** — `depends`, `makedepends`, `checkdepends` and `optdepends` are read by key rather than by substring search. The old parser reported a package's *build* dependencies as its runtime dependencies; `aurora inspect paru` claimed `cargo` was required at runtime and dropped `git`, `pacman` and `libalpm`. Split packages (`pkgname=(...)`), `depends+=(...)` appends, commented-out assignments and function bodies are handled correctly.
- **`--from official` means Arch** — a package provided by chaotic-aur or another third-party repo is no longer accepted as "official", and listings name the real repository instead of calling everything outside the AUR official.
- **`--inspect` cannot be bypassed** — the PKGBUILD is reviewed from the clone that is about to be built, and a failed fetch aborts that package instead of falling through to `makepkg`.
- **Arch-aware self-update** — the binary is selected for the machine's architecture and must match a SHA256 digest; the published `.sha256` file is used when GitHub omits the digest field. Releases now ship linux/amd64 and linux/arm64.
- **Real exit codes** — every command returns non-zero on failure, `search` returns non-zero when nothing matched, and `outdated` returns 1 when updates exist.

### ⚙️ Scriptable

- **`--json`** on `search`, `info`, `list`, `inspect`, `outdated` and `doctor`.
- **`aurora outdated`** — read-only update check, no sudo, exits 1 when work is pending.
- **`--quiet`** — suppresses informational output; warnings and errors still reach stderr.

### 🗣️ Beginner-first

- **`aurora doctor`** — checks pacman, vercmp, git, makepkg, fakeroot, sudo, checkupdates, database freshness, AUR reachability, cache writability, orphans and config validity. Exits 1 when something required is missing.
- **AUR dependency ordering** — `install` and `update` resolve `depends`/`makedepends`/`checkdepends` and build dependencies first. Virtual provides that pacman handles itself are left to pacman.
- **PKGBUILD diff on rebuild (`--diff`)** — opt-in, off by default. Beginners get a clean update; auditors can see exactly what changed since the last build.
- **Shell completion** — `aurora completion bash|zsh|fish|powershell` is enabled, and package arguments complete from installed packages.

### 🛡️ Reliability

- Every subprocess that can block has a deadline; a pacman call waiting on a database lock can no longer hang the tool. The two interactive calls (sudo asking for a password, `go install` compiling) are unbounded on purpose.
- Version checks run through a bounded worker pool instead of one goroutine (and one `vercmp` process) per package.
- `--needed` is passed to pacman, so re-running an install is a no-op instead of a reinstall.
- Errors are printed exactly once, to stderr.

---

## Commands

| Command | What it does |
|---|---|
| `aurora doctor [--json]` | Check that this machine can use every feature |
| `aurora outdated [--aur\|--repos] [--json]` | List available updates without changing anything |
| `aurora explain [topic]` | Learn Arch Linux concepts, pacman flags, and AUR workflows |
| `aurora inspect <package> [--from\|-O\|-A] [--raw] [--json]` | Inspect a PKGBUILD or official package metadata (alias: `pkgbuild`) |
| `aurora search <query...> [--from\|-O\|-A] [--json]` | Search official repos and the AUR concurrently |
| `aurora info <package...> [--from\|-O\|-A] [--json]` | Show detailed package information |
| `aurora install <package...> [--from\|-O\|-A] [--dry-run] [-i] [--diff] [--no-deps]` | Install in batch with preview, PKGBUILD review and dependency ordering |
| `aurora remove <package...>` | Remove packages with orphan cleanup (`-Rns`) |
| `aurora update [--dry-run] [--diff] [--no-deps]` | Full system upgrade, then rebuild outdated AUR packages |
| `aurora autoremove` | Remove unneeded orphan dependencies (apt-style) |
| `aurora list [query] [--official\|--foreign\|--orphans\|--upgradable] [--all] [--json]` | List packages by category |
| `aurora clean [--cache\|--pacman\|--all] [--dry-run]` | Reclaim disk space from build artifacts and package caches |
| `aurora selfupdate [--check] [--force]` | Update Aurora itself from GitHub Releases |
| `aurora completion <bash\|zsh\|fish\|powershell>` | Print a completion script |

### Global Flags

- **`-y, --yes`**: Skip Aurora's own confirmation prompts. pacman itself runs with `--noconfirm`.
- **`-c, --concise`**: Suppress teaching explanations and notes.
- **`-q, --quiet`**: Suppress informational output. Warnings, errors and the package list shown before a confirmation are always printed.
- **`--json`**: Emit machine-readable JSON instead of prose. Supported by `search`, `info`, `list`, `inspect`, `outdated` and `doctor`; other commands reject it rather than silently printing nothing.
- **`-v, --version`**: Show the Aurora version.

`--yes`, `--concise`, `--quiet`, `--from` and the other settings can also be set in the config file; an explicit flag always wins.

---

## Configuration

Aurora reads `~/.config/aurora/config.toml` (or `$XDG_CONFIG_HOME/aurora/config.toml`). It is a small TOML subset: top-level `key = value` pairs with `#` comments. Unknown keys and unusable values are reported with a line number rather than ignored.

```toml
# Suppress teaching notes
concise = false

# Suppress informational output entirely
quiet = false

# Skip Aurora's prompts
yes = false

# Let pacman ask its own questions instead of passing --noconfirm
interactive_pacman = false

# Concurrent version/metadata checks (1-64)
parallelism = 8

# Move the AUR build cache off the default ~/.cache/aurora
cache_dir = "/home/you/.cache/aurora"

# Default for --from: any, official, repo, or aur
default_source = "any"

# Show PKGBUILD changes before rebuilding an AUR package
pkgbuild_diff = false

# Resolve and order AUR dependencies before building
aur_deps = true
```

Run `aurora doctor` to confirm the file was picked up.

---

## Philosophy

- **Pacman is King** — Aurora delegates to `pacman` and `makepkg`. It orchestrates, never replaces.
- **Transparency First** — Every destructive command is previewed before it runs, and `--dry-run` exists on every command where it makes sense.
- **No Jargon Without Explanation** — When Aurora uses flags like `-Syu` or `-Rns`, `aurora explain` says what they mean.
- **Official Wins** — Official repositories are preferred. AUR is an explicit choice, not a hidden default.
- **Silence Is Not Safety** — A check that cannot run says so. An unverified download is refused, not installed.

---

## Installation

### 1. Download Pre-built Binary (Recommended)

Download the binary for your architecture from the [releases page](https://github.com/abhigyan-chatterjee/aurora/releases):

- `aurora-linux-amd64` for x86_64
- `aurora-linux-arm64` for aarch64 / ARM

Verify it against the matching `.sha256` file, then:

```bash
sudo install -m 755 aurora-linux-amd64 /usr/local/bin/aurora
```

### 2. Install with Go

Requires Go 1.25.6 or newer:

```bash
go install github.com/abhigyan-chatterjee/aurora@latest
```

### 3. Build from Source

```bash
git clone https://github.com/abhigyan-chatterjee/aurora.git
cd aurora
go build -o build/aurora .
sudo install -m 755 build/aurora /usr/local/bin/aurora
```

---

## Quick Start

```bash
# Check the machine first
aurora doctor

# See what needs upgrading (read-only)
aurora outdated

# Teaching & Exploration
aurora explain -Syu         # What -Syu means and why it matters
aurora inspect paru         # Read the PKGBUILD before trusting it

# Search & Info
aurora search neovim git    # Official repos + AUR, in parallel
aurora search yay --from aur
aurora info neovim --json   # Feed it to fzf, jq, or a script

# Installation & Upgrades
aurora install neovim git
aurora install paru --dry-run   # See the plan, including AUR dependencies
aurora install paru -i          # Review the PKGBUILD before building
aurora update                   # -Syu first, then AUR rebuilds
aurora update --diff            # ...and show what changed in each PKGBUILD

# System Maintenance
aurora autoremove
aurora list --upgradable
aurora list --foreign
aurora clean --dry-run
aurora remove neovim
```

---

## Known Limitations

- **AUR packages are not signed.** The AUR hosts build scripts, not binaries, so there is no signature to check. Aurora can show you the PKGBUILD (`inspect`, `--inspect`) and what changed since your last build (`--diff`); judging it is still your call.
- **The update plan is computed before the system upgrade.** If `pacman -Syu` changes something that makes an AUR rebuild pointless, the rebuild still runs. Re-running `aurora update` after a large upgrade gets a fresh plan.
- **PKGBUILD parsing is not a shell.** It reads declarations, not expressions: a `pkgver` computed at build time shows as the literal text. Braces inside an unquoted here-document whose body is unbalanced can end a function body early.
- **`list --upgradable` depends on `checkupdates`** (pacman-contrib) for repository upgrades. Without it Aurora says so instead of reporting a false all-clear.

---

## Roadmap

- **v0.5 — Correctness, Scriptability & Diagnostics** (this release)
  - PKGBUILD parser rewritten: dependencies, split packages, appends, comments, function bodies
  - `--json`, `--quiet`, config file, `aurora outdated`, `aurora doctor`, shell completion
  - AUR dependency ordering, opt-in PKGBUILD diff, architecture-aware self-update
  - Subprocess deadlines, bounded workers, honest exit codes, third-party repository attribution
- **v0.4 — Self-Update & Safe Upgrade Ordering** (complete)
  - Self-updater with SHA256 verification and `go install` fallback
  - `pacman -Syu` before AUR rebuilds
  - Chunked AUR RPC queries, non-root AUR build guard, `RunE` error propagation
  - Arch Linux CI and semantic release pipeline
- **v0.3 — Paru Power + Apt Simplicity** (complete)
  - Teaching mode, concurrent lookups, multi-argument batch operations
  - Source intelligence (`--from`, `inspect`, pre-build review)
  - Lifecycle commands (`autoremove`, `list`, `clean`)
- **Future Explorations**
  - Optional mirror freshness check
  - Cached package metadata to cut repeated AUR round-trips

---

## Contributing

We value clarity and transparency. If you have an idea, open an issue first — we prioritize changes that keep Aurora transparent and predictable.

Every change is expected to keep `gofmt`, `go vet`, `staticcheck` and `go test -race ./...` clean; CI enforces all four, plus `govulncheck`.

## License

GPL-3.0. See [LICENSE](LICENSE).
