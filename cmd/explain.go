package cmd

import (
	"bufio"
	"os"
	"strings"

	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/spf13/cobra"
)

type conceptExplanation struct {
	Title       string
	Summary     string
	Description string
	Aliases     []string
}

var explanations = []conceptExplanation{
	{
		Title:   "-Syu (Full System Upgrade)",
		Summary: "Refreshes database indexes and upgrades all installed packages.",
		Description: `Command: sudo pacman -Syu

What each flag means:
  -S : synchronize (operation mode)
  -y : refresh package databases from official mirrors
  -u : upgrade all installed packages to their newest versions

Why it matters:
  Arch Linux is a rolling-release distribution. Performing full system upgrades
  regularly keeps dependencies synchronised and safe. Partial upgrades (installing
  new software without updating the rest of the system) are discouraged.`,
		Aliases: []string{"-Syu", "syu", "system-upgrade", "upgrade"},
	},
	{
		Title:   "-Rns (Safe Package Removal)",
		Summary: "Removes a package, its unneeded dependencies, and configuration files.",
		Description: `Command: sudo pacman -Rns <package>

What each flag means:
  -R : remove the specified package
  -n : nosave (remove global configuration files)
  -s : recursive (remove dependencies nothing else needs)

Why it matters:
  Plain 'pacman -R' leaves orphan dependencies behind. 'pacman -Rns' removes the
  application and the dependencies it pulled in, without touching anything another
  package still needs.`,
		Aliases: []string{"-Rns", "rns", "remove-flags"},
	},
	{
		Title:   "AUR (Arch User Repository)",
		Summary: "Community repository of build scripts (PKGBUILDs), not binaries.",
		Description: `The Arch User Repository (AUR) is a community-driven repository.

Key points:
  1. The AUR does NOT host prebuilt binary packages.
  2. It hosts build scripts called PKGBUILDs.
  3. Aurora clones the repository, shows you the PKGBUILD if you ask, and runs
     'makepkg' to build the package on your machine.

Safety advice:
  Prefer official repository packages when both exist. Official packages are
  prebuilt, signed, and maintained by Arch developers.`,
		Aliases: []string{"aur", "arch-user-repository"},
	},
	{
		Title:   "Official Repositories",
		Summary: "Core, Extra, and Multilib repositories maintained by Arch developers.",
		Description: `Official repositories (core, extra, multilib) are the primary software
sources on Arch Linux.

Key advantages:
  - Packages are prebuilt binaries, so there is no build time.
  - Maintained and digitally signed by Arch Linux developers.
  - Updated as part of every system upgrade.

Aurora's rule:
  Official repositories are always preferred over the AUR.`,
		Aliases: []string{"official", "official-repos", "repos"},
	},
	{
		Title:   "PKGBUILD & makepkg",
		Summary: "The build recipe and build tool used to create Arch packages.",
		Description: `A PKGBUILD is a script holding metadata (version, dependencies, source URLs)
and build instructions.

How makepkg works:
  1. Downloads source files from upstream.
  2. Verifies integrity using the checksums in the PKGBUILD.
  3. Compiles the software locally.
  4. Packages the result into a '.pkg.tar.zst' archive.
  5. Installs it with pacman.

Because it runs shell code from the internet, review the PKGBUILD before building
anything you do not trust: 'aurora inspect <package>'.`,
		Aliases: []string{"pkgbuild", "makepkg"},
	},
	{
		Title:   "Partial Upgrades",
		Summary: "Installing new software without upgrading the rest of the system.",
		Description: `A partial upgrade happens when you refresh the database ('pacman -Sy') and
install a package without upgrading what is already installed ('pacman -u').

Why it causes problems:
  Shared libraries (openssl, glibc) move constantly on Arch. An application built
  against a newer library than the one installed can crash in confusing ways.

Best practice:
  Always run a full upgrade: 'aurora update' or 'sudo pacman -Syu'.`,
		Aliases: []string{"partial-upgrades", "partial-upgrade"},
	},
	{
		Title:   "Foreign Packages & Orphans",
		Summary: "Packages outside the official repos, and dependencies nothing needs.",
		Description: `Foreign packages:
  Installed packages that no sync database provides. Usually built from the AUR
  or installed from a local file. View them with: 'pacman -Qm'

Orphans:
  Packages installed as dependencies whose parent has since been removed.
  View them with: 'aurora list --orphans'
  Remove them with: 'aurora autoremove'`,
		Aliases: []string{"foreign", "foreign-packages", "orphans"},
	},
}

func explain(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// The command sets DisableFlagParsing so topics like -Syu are not eaten by
	// the flag parser. Standard flags are therefore handled here by hand.
	var cleanArgs []string
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return cmd.Help()
		case "--concise", "-c":
			conciseFlag = true
			ui.Quiet = quietFlag
		case "--yes", "-y":
			yesFlag = true
		case "--quiet", "-q":
			quietFlag = true
			ui.Quiet = true
		default:
			cleanArgs = append(cleanArgs, arg)
		}
	}

	if len(cleanArgs) > 0 {
		query := strings.ToLower(strings.TrimSpace(cleanArgs[0]))
		for _, exp := range explanations {
			for _, alias := range exp.Aliases {
				if strings.EqualFold(alias, query) {
					printExplanation(exp)
					return nil
				}
			}
		}
		ui.Out("No explanation topic found for %q.\n\n", cleanArgs[0])
	}

	ui.Outln("=== Aurora Teaching Mode — Concepts & Walkthroughs ===")
	ui.Outln("Select a topic to learn more about Arch Linux package management:")
	ui.Outln("")

	items := make([]ui.SelectionItem, 0, len(explanations))
	for i, exp := range explanations {
		items = append(items, ui.SelectionItem{
			Index:       i + 1,
			Name:        exp.Title,
			SourceLabel: exp.Summary,
		})
		ui.Out("  [%d] %-30s - %s\n", i+1, exp.Title, exp.Summary)
	}

	selected := ui.PromptSelection(reader, "\nEnter topic number or name (or press Enter to exit): ", items)
	if selected == nil {
		return nil
	}

	for _, exp := range explanations {
		if exp.Title == selected.Name {
			printExplanation(exp)
			return nil
		}
	}
	return nil
}

func printExplanation(exp conceptExplanation) {
	ui.Outln("\n=======================================================")
	ui.Out(" Topic: %s\n", exp.Title)
	ui.Outln("=======================================================")
	ui.Outln("")
	if conciseFlag {
		ui.Outln(exp.Summary)
	} else {
		ui.Outln(exp.Description)
	}
	ui.Outln("")
}

var explainCmd = &cobra.Command{
	Use:                "explain [topic]",
	Short:              "Explains Arch Linux package concepts, flags, and workflows in plain English",
	DisableFlagParsing: true,
	Long: `Teaching mode command that provides plain-English explanations of Arch Linux
concepts, pacman flags (-Syu, -Rns), the AUR build process, and safety best practices.

Available topics:
  - -Syu / upgrade
  - -Rns / remove-flags
  - aur / arch-user-repository
  - official / official-repos
  - pkgbuild / makepkg
  - partial-upgrades
  - foreign / orphans`,
	RunE:              explain,
	ValidArgsFunction: completeExplainTopics,
}

// completeExplainTopics offers the built-in topics for shell completion.
func completeExplainTopics(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	topics := make([]string, 0, len(explanations))
	for _, exp := range explanations {
		topics = append(topics, exp.Aliases...)
	}
	return topics, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	rootCmd.AddCommand(explainCmd)
}
