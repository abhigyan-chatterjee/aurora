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

// PromptSelection presents a numbered picker and parses either a numerical index
// or an item name. It returns the selected item, or nil on skip or invalid input.
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
