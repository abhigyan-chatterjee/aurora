package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/abhigyan-chatterjee/aurora/internal/aur"
	"github.com/abhigyan-chatterjee/aurora/internal/config"
	"github.com/abhigyan-chatterjee/aurora/internal/run"
	"github.com/abhigyan-chatterjee/aurora/internal/ui"
	"github.com/spf13/cobra"
)

var (
	yesFlag     bool
	conciseFlag bool
	quietFlag   bool
	jsonFlag    bool

	// Version is the application version, overridable at build time via:
	// -ldflags "-X github.com/abhigyan-chatterjee/aurora/cmd.Version=x.y.z"
	Version = "0.5.0"

	// appConfig is the merged config file and flag state for this run.
	appConfig = config.Default()
)

var rootCmd = &cobra.Command{
	Use:           "aurora",
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	Short:         "Aurora — A powerful, beginner-first package assistant for Arch Linux",
	Long: `Aurora is a package manager and AUR helper for Arch Linux that combines
the power of Paru with the simplicity of apt. It prioritizes official repositories,
explains commands in plain English, and provides transparent package lifecycle management.

Run 'aurora doctor' to check that this machine can use every feature.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Commands that are pure documentation or that update Aurora itself do
		// not need pacman, so they still work when the tool is misused on
		// another distribution or before Arch is installed.
		if needsPacman(cmd) {
			if _, err := run.LookPath("pacman"); err != nil {
				return fmt.Errorf("this machine does not appear to be Arch-based (pacman not found); Aurora only supports Arch-based systems")
			}
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		appConfig = cfg
		applyConfig(cmd, cfg)

		// --json is a persistent flag, but only the reporting commands implement
		// it. Accepting it elsewhere silenced the transaction plan and left the
		// user staring at a bare confirmation prompt.
		if jsonFlag && !jsonCapableCommands[cmd.Name()] {
			return fmt.Errorf("--json is not supported by %q; it works with search, info, list, inspect, outdated and doctor", cmd.Name())
		}
		return nil
	},
}

// jsonCapableCommands implement --json. Every other command prints prose only,
// so accepting the flag there would suppress output without replacing it.
var jsonCapableCommands = map[string]bool{
	"search":   true,
	"info":     true,
	"list":     true,
	"inspect":  true,
	"outdated": true,
	"doctor":   true,
}

// needsPacman reports whether a command requires an Arch system.
func needsPacman(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "explain", "selfupdate", "self-update", "completion", "help":
		// Documentation and self-update work anywhere. Every other command
		// drives pacman and needs an Arch system.
		return false
	}
	return true
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "skip Aurora's confirmation prompts (pacman runs with --noconfirm)")
	rootCmd.PersistentFlags().BoolVarP(&conciseFlag, "concise", "c", false, "enable concise output mode (suppress teaching explanations and notes)")
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "suppress informational output; warnings and errors are still printed")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "emit machine-readable JSON instead of prose (search, info, list, inspect, outdated, doctor)")

	// Shell completion. Cobra registers a `completion` command automatically
	// once it is not disabled, which is what makes `aurora <tab>` work for
	// bash, zsh, fish and PowerShell.
	rootCmd.CompletionOptions.DisableDefaultCmd = false
}

// applyConfig layers the config file under explicitly passed flags.
func applyConfig(cmd *cobra.Command, cfg config.Config) {
	if !flagChanged(cmd, "yes") {
		yesFlag = cfg.Yes
	}
	if !flagChanged(cmd, "concise") {
		conciseFlag = cfg.Concise
	}
	if !flagChanged(cmd, "quiet") {
		quietFlag = cfg.Quiet
	}

	if !flagChanged(cmd, "from") {
		switch cmd.Name() {
		case "install":
			installFromFlag = cfg.DefaultSource
		case "search":
			searchFromFlag = cfg.DefaultSource
		case "info":
			infoFromFlag = cfg.DefaultSource
		case "inspect":
			inspectFromFlag = cfg.DefaultSource
		}
	}

	aur.CacheDirOverride = cfg.CacheDir

	ui.Quiet = quietFlag
	ui.JSONMode = jsonFlag
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if f := cmd.Flags().Lookup(name); f != nil {
		return f.Changed
	}
	if f := cmd.PersistentFlags().Lookup(name); f != nil {
		return f.Changed
	}
	return false
}

// Execute runs the CLI and returns the process exit code.
func Execute() {
	aur.SetVersion(Version)
	rootCmd.Version = Version

	if err := rootCmd.Execute(); err != nil {
		// A command may ask for a specific status: `outdated` exits 1 when
		// updates exist, which is a result rather than a failure.
		var coded interface{ ExitCode() int }
		if errors.As(err, &coded) {
			os.Exit(coded.ExitCode())
		}
		// Cobra is silenced so a failure is reported exactly once, here, in a
		// consistent format. With SilenceErrors alone the message was simply
		// dropped and the user saw a bare exit code.
		fmt.Fprintf(os.Stderr, "aurora: %v\n", err)
		os.Exit(1)
	}
}

// completePackageNames offers installed package names for shell completion.
// It is used by commands whose argument is a package already on the system.
func completePackageNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names, err := installedPackageNames()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeAnyPackageName accepts free-form package names without offering
// filesystem paths, which are never valid arguments here.
func completeAnyPackageName(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
