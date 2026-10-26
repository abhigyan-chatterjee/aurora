package ui

import (
	"encoding/json"
	"fmt"
	"os"
)

// Output modes. Both are set once during startup from flags and config.
//
//   - Quiet suppresses informational output but never prompts, warnings or
//     errors: a user who asked for silence still has to be told when something
//     failed.
//   - JSON suppresses informational output as well, because the machine-readable
//     document is the entire result. Warnings go to stderr so they never
//     corrupt a piped JSON stream.
var (
	Quiet    bool
	JSONMode bool
)

// Out writes informational output to stdout unless output is suppressed.
func Out(format string, a ...any) {
	if Quiet || JSONMode {
		return
	}
	fmt.Printf(format, a...)
}

// Outln writes an informational line to stdout unless output is suppressed.
func Outln(a ...any) {
	if Quiet || JSONMode {
		return
	}
	fmt.Println(a...)
}

// Preview writes a transaction summary that the user is about to confirm.
//
// Quiet does not suppress it. Suppressing the plan while still showing the
// prompt asks the user to approve a destructive action they cannot see, which
// is worse than being noisy.
func Preview(format string, a ...any) {
	if JSONMode {
		return
	}
	fmt.Printf(format, a...)
}

// Warn writes a warning to stderr. Warnings survive --quiet: silently dropping
// a warning would leave the user with a failed operation and no explanation.
func Warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", a...)
}

// Errf writes an error line to stderr.
func Errf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// EmitJSON writes v as an indented JSON document to stdout.
//
// It writes to the real os.Stdout directly so that --json output is never
// swallowed by the suppression applied to Out/Outln.
func EmitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
