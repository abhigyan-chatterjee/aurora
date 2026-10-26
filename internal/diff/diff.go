// Package diff produces unified diffs for text with line granularity.
//
// It exists so Aurora can show what changed in an AUR PKGBUILD between two
// builds without pulling in a dependency. PKGBUILDs are a few hundred lines, so
// the quadratic LCS table costs nothing.
package diff

import (
	"fmt"
	"strings"
)

// contextLines is how many unchanged lines surround each hunk.
const contextLines = 3

type opKind int

const (
	opEqual opKind = iota
	opDelete
	opInsert
)

type op struct {
	kind    opKind
	content string
}

// UnifiedText returns a unified diff of two documents, or "" when they match.
func UnifiedText(nameA, nameB, a, b string) string {
	if a == b {
		return ""
	}
	return Unified(nameA, nameB, splitLines(a), splitLines(b))
}

// Unified returns a unified diff of two line slices, or "" when they match.
func Unified(nameA, nameB string, a, b []string) string {
	ops := editScript(a, b)

	var hunks []string
	for _, h := range groupHunks(ops) {
		hunks = append(hunks, renderHunk(h))
	}
	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n", nameA)
	fmt.Fprintf(&sb, "+++ %s\n", nameB)
	for _, h := range hunks {
		sb.WriteString(h)
	}
	return sb.String()
}

// SplitLines splits a document for Unified. A trailing newline does not
// produce a spurious empty final line.
func SplitLines(s string) []string { return splitLines(s) }

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// editScript computes a shortest edit script with an LCS table.
func editScript(a, b []string) []op {
	n, m := len(a), len(b)

	// lcs[i][j] is the LCS length of a[i:] and b[j:].
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	ops := make([]op, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, op{opEqual, a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, op{opDelete, a[i]})
			i++
		default:
			ops = append(ops, op{opInsert, b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{opDelete, a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, op{opInsert, b[j]})
	}
	return ops
}

// hunk is a contiguous run of changed lines plus surrounding context.
type hunk struct {
	aStart, bStart int // 1-based line number of the first line in the hunk
	lines          []string
	aCount, bCount int
}

// groupHunks turns an edit script into hunks with limited context.
func groupHunks(ops []op) []hunk {
	// Mark which operations are within contextLines of a change.
	changed := make([]bool, len(ops))
	for idx, o := range ops {
		if o.kind != opEqual {
			for k := idx - contextLines; k <= idx+contextLines; k++ {
				if k >= 0 && k < len(ops) {
					changed[k] = true
				}
			}
		}
	}

	var (
		hunks   []hunk
		aLine   = 1
		bLine   = 1
		current *hunk
	)

	for idx, o := range ops {
		if !changed[idx] {
			if o.kind == opEqual {
				aLine++
				bLine++
			} else if o.kind == opDelete {
				aLine++
			} else {
				bLine++
			}
			current = nil
			continue
		}

		if current == nil {
			current = &hunk{aStart: aLine, bStart: bLine}
			hunks = append(hunks, hunk{})
		}

		switch o.kind {
		case opEqual:
			current.lines = append(current.lines, " "+o.content)
			current.aCount++
			current.bCount++
			aLine++
			bLine++
		case opDelete:
			current.lines = append(current.lines, "-"+o.content)
			current.aCount++
			aLine++
		case opInsert:
			current.lines = append(current.lines, "+"+o.content)
			current.bCount++
			bLine++
		}

		// Write the accumulated hunk back through the pointer.
		hunks[len(hunks)-1] = *current
	}

	return hunks
}

func renderHunk(h hunk) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "@@ -%s +%s @@\n", formatRange(h.aStart, h.aCount), formatRange(h.bStart, h.bCount))
	for _, line := range h.lines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatRange(start, count int) string {
	if count == 0 {
		// Unified diff convention: an empty range points just before start.
		return fmt.Sprintf("%d,0", start-1)
	}
	return fmt.Sprintf("%d,%d", start, count)
}
