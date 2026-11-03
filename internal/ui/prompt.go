package ui

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// SelectionItem represents a selectable package, topic, or candidate item in an interactive prompt.
type SelectionItem struct {
	Index       int
	Name        string
	FullName    string
	SourceLabel string
	Version     string
	Description string
}

// ConfirmAction prompts the user for a yes/no confirmation.
// If autoConfirm is true (e.g., from --yes / -y), it auto-confirms without blocking.
func ConfirmAction(reader *bufio.Reader, prompt string, autoConfirm bool) bool {
	if autoConfirm {
		fmt.Printf("%s (y/N): y [auto-confirmed]\n", prompt)
		return true
	}

	fmt.Printf("%s (y/N): ", prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// BinPromptChoice represents the user's decision between source and binary packages.
type BinPromptChoice int

const (
	BinChoiceBinary BinPromptChoice = iota // Selected -bin
	BinChoiceSource                        // Selected source
	BinChoiceSkip                          // User cancelled/skipped this package
)

// PromptBinSelection presents the -bin vs source choice and returns the chosen action.
func PromptBinSelection(reader *bufio.Reader, pkgName, binPkgName string, concise, autoConfirm bool) BinPromptChoice {
	if autoConfirm {
		Out("Found '%s' in AUR. Selecting binary package (--yes).\n", binPkgName)
		return BinChoiceBinary
	}

	quiet := Quiet

	if !quiet {
		if concise {
			Out("AUR binary package available for '%s':\n", pkgName)
		} else {
			Out("An AUR binary package is available for '%s': '%s'\n", pkgName, binPkgName)
			Out("\n")
			Out("Why choose -bin?\n")
			Out("  • %-*s : Downloads pre-compiled binaries. Installs in seconds with\n", len(binPkgName), binPkgName)
			Out("                        minimal CPU and memory usage (recommended).\n")
			Out("  • %-*s : Compiles locally from source with makepkg. Requires build\n", len(binPkgName), pkgName)
			Out("                        toolchains, high CPU usage, and can take much longer.\n")
			Out("\n")
			Out("Which package would you like to install?\n")
		}
		Out("  [1] %-*s (binary, recommended)\n", len(binPkgName), binPkgName)
		Out("  [2] %-*s (source)\n", len(binPkgName), pkgName)
		Out("\n")
	}

	attempts := 0
	for {
		if quiet {
			fmt.Printf("Install '%s' instead of source '%s'? [Y/n]: ", binPkgName, pkgName)
		} else if attempts == 0 {
			fmt.Print("Enter choice [1/2] (default: 1): ")
		}

		input, err := reader.ReadString('\n')
		trimmed := strings.TrimSpace(input)
		if err != nil && trimmed == "" {
			Warn("non-interactive session detected; defaulting to '%s'", binPkgName)
			return BinChoiceBinary
		}

		lower := strings.ToLower(trimmed)
		choice, ok, skip := parseBinChoice(lower, pkgName, binPkgName)
		if skip {
			return BinChoiceSkip
		}
		if ok {
			return choice
		}

		attempts++
		if attempts >= 3 {
			fmt.Printf("Defaulting to '%s'.\n", binPkgName)
			return BinChoiceBinary
		}
		fmt.Printf("Invalid choice '%s'. Please enter 1 for '%s' or 2 for '%s' (default: 1): ", trimmed, binPkgName, pkgName)
	}
}

func parseBinChoice(input, pkgName, binPkgName string) (BinPromptChoice, bool, bool) {
	switch {
	case input == "", input == "1", input == "b", input == "bin", input == "binary",
		input == "y", input == "yes":
		return BinChoiceBinary, true, false
	case input == "2", input == "s", input == "src", input == "source",
		input == "n", input == "no":
		return BinChoiceSource, true, false
	case input == "q", input == "quit", input == "abort", input == "cancel":
		return BinChoiceSkip, true, true
	}
	if strings.EqualFold(input, binPkgName) {
		return BinChoiceBinary, true, false
	}
	if strings.EqualFold(input, pkgName) {
		return BinChoiceSource, true, false
	}
	return BinChoiceBinary, false, false
}

// PromptSelection presents a numbered list and returns the selected item, or nil
// on skip or invalid input.
//
// The slice index is deliberately not returned: every caller acts on the item,
// and an index that nobody reads is an invitation to misuse it.
func PromptSelection(reader *bufio.Reader, prompt string, items []SelectionItem) *SelectionItem {
	if len(items) == 0 {
		return nil
	}

	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	// 1. Numeric choice, matched against the item's displayed Index.
	if num, err := strconv.Atoi(input); err == nil {
		for i := range items {
			if items[i].Index == num {
				return &items[i]
			}
		}
	}

	// 2. Exact, case-insensitive name or full-name match.
	for i := range items {
		if strings.EqualFold(items[i].Name, input) ||
			(items[i].FullName != "" && strings.EqualFold(items[i].FullName, input)) {
			return &items[i]
		}
	}

	return nil
}
