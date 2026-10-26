package aur

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrPackageNotFound reports that the AUR has no PKGBUILD for this name.
//
// It is a sentinel so callers can tell "this package does not exist" from
// "the AUR could not be reached", which need different messages and different
// fallbacks.
var ErrPackageNotFound = errors.New("package not found in the AUR")

// aurCgitBase is the PKGBUILD endpoint. It is a variable so tests can point it
// at a local server.
var aurCgitBase = "https://aur.archlinux.org/cgit/aur.git/plain/PKGBUILD"

// PKGBUILDInfo holds metadata extracted from a PKGBUILD.
//
// The parser reads assignments by key, line by line. It never searches the file
// for a substring, because `depends=` is a suffix of `makedepends=`: a search
// for the shorter key silently returns the longer key's values, which is how
// `aurora inspect paru` used to report cargo as a runtime dependency.
type PKGBUILDInfo struct {
	// PackageName is the first name declared by pkgname. For split packages it
	// is the base package; PackageNames holds the full list.
	PackageName  string
	PackageNames []string
	Version      string
	Release      string
	Description  string
	URL          string
	Maintainer   string
	Depends      []string
	MakeDepends  []string
	CheckDepends []string
	OptDepends   []string
	Provides     []string
	Conflicts    []string
	Replaces     []string
	Sources      []string
	RawContent   string
}

// FetchPKGBUILD retrieves the raw PKGBUILD file from the AUR cgit interface.
func FetchPKGBUILD(packageName string) (string, error) {
	if !isValidPackageName(packageName) {
		return "", fmt.Errorf("invalid package name: %s", packageName)
	}

	cgitURL := fmt.Sprintf("%s?h=%s", aurCgitBase, url.QueryEscape(packageName))

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := newAURRequest(cgitURL)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching PKGBUILD: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: %s", ErrPackageNotFound, packageName)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received HTTP status %s when fetching PKGBUILD", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading PKGBUILD response: %v", err)
	}

	content := string(bodyBytes)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("%w: %s has an empty PKGBUILD", ErrPackageNotFound, packageName)
	}

	return content, nil
}

// ParsePKGBUILD extracts metadata from raw PKGBUILD text.
//
// Only top-level `key=value`, `key=(...)` and `key+=(...)` assignments are
// interpreted. Shell code inside build(), package() and friends is not executed
// and not evaluated: a PKGBUILD that computes pkgver at build time reports the
// literal text, which is the safe failure mode.
func ParsePKGBUILD(content string) PKGBUILDInfo {
	info := PKGBUILDInfo{RawContent: content}

	scalars := make(map[string]string)
	arrays := make(map[string][]string)

	lines := strings.Split(content, "\n")
	braceDepth := 0
	for i := 0; i < len(lines); i++ {
		stripped := stripLineComment(lines[i])
		trimmed := strings.TrimSpace(stripped)

		// Assignments inside build(), package_*() and friends are shell code,
		// not metadata. Only the top-level declarations describe the package.
		atTopLevel := braceDepth == 0
		braceDepth += braceDelta(stripped)
		if braceDepth < 0 {
			braceDepth = 0
		}

		if trimmed == "" || !atTopLevel {
			continue
		}

		key, rest, appendOp, ok := splitAssignment(trimmed)
		if !ok {
			continue
		}

		if strings.HasPrefix(rest, "(") {
			body := rest[1:]
			for findArrayEnd(body) < 0 && i+1 < len(lines) {
				i++
				body += "\n" + stripLineComment(lines[i])
			}
			end := findArrayEnd(body)
			if end < 0 {
				end = len(body)
			}
			elements := arrayElements(body[:end])
			if appendOp {
				arrays[key] = append(arrays[key], elements...)
			} else {
				arrays[key] = elements
			}
			continue
		}

		if appendOp {
			if value := unquoteScalar(rest); value != "" {
				arrays[key] = append(arrays[key], value)
			}
			continue
		}

		if value := unquoteScalar(rest); value != "" {
			scalars[key] = value
		}
	}

	// The maintainer lives in a leading comment, not an assignment.
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# Maintainer:") {
			info.Maintainer = strings.TrimSpace(strings.TrimPrefix(trimmed, "# Maintainer:"))
			break
		}
	}

	if names := arrays["pkgname"]; len(names) > 0 {
		info.PackageNames = names
		info.PackageName = names[0]
	} else if name := scalars["pkgname"]; name != "" {
		info.PackageName = name
		info.PackageNames = []string{name}
	}

	info.Version = scalars["pkgver"]
	info.Release = scalars["pkgrel"]
	info.Description = scalars["pkgdesc"]
	info.URL = scalars["url"]

	info.Depends = arrays["depends"]
	info.MakeDepends = arrays["makedepends"]
	info.CheckDepends = arrays["checkdepends"]
	info.OptDepends = arrays["optdepends"]
	info.Provides = arrays["provides"]
	info.Conflicts = arrays["conflicts"]
	info.Replaces = arrays["replaces"]
	info.Sources = arrays["source"]

	return info
}

// stripLineComment removes a trailing # comment that is not inside a quoted
// string. A line that is entirely a comment returns "".
func stripLineComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '#':
			return line[:i]
		}
	}
	return line
}

// splitAssignment parses `key=value` or `key+=value`, returning the key, the
// raw value text, and whether the append operator was used.
func splitAssignment(line string) (key, rest string, appendOp bool, ok bool) {
	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false, false
	}

	name := strings.TrimSpace(line[:eq])
	rest = strings.TrimSpace(line[eq+1:])

	if strings.HasSuffix(name, "+") {
		appendOp = true
		name = strings.TrimSpace(strings.TrimSuffix(name, "+"))
	}
	if !isIdentifier(name) {
		return "", "", false, false
	}
	return name, rest, appendOp, true
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch == '_':
		case ch >= '0' && ch <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// braceDelta counts net brace nesting on a line, ignoring braces inside quotes.
// It is how function bodies are skipped: anything after an unbalanced `{` is
// shell code rather than a package declaration.
//
// Known limitation: braces inside an unquoted here-document are counted, so a
// here-document whose body has unbalanced braces can end the body early and let
// a following assignment be read as metadata. Real PKGBUILDs keep braces
// balanced in here-documents (JSON, C, shell all balance), so this is left as a
// documented edge rather than a tokenizer that parses shell.
func braceDelta(line string) int {
	delta := 0
	var quote byte
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

// findArrayEnd returns the index of the parenthesis that closes an array whose
// opening parenthesis has already been consumed, or -1 if the array continues.
// Parentheses inside quotes do not count, so a source URL containing '(' does
// not truncate the array.
func findArrayEnd(body string) int {
	depth := 0
	var quote byte
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// arrayElements splits array contents on whitespace outside quotes and strips
// the quoting from each element.
func arrayElements(content string) []string {
	var (
		out   []string
		quote byte
		cur   strings.Builder
		flush = func() {
			if s := strings.TrimSpace(cur.String()); s != "" {
				out = append(out, s)
			}
			cur.Reset()
		}
	)

	for i := 0; i < len(content); i++ {
		ch := content[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
				continue
			}
			cur.WriteByte(ch)
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			cur.WriteByte(ch)
		}
	}
	flush()
	return out
}

// unquoteScalar strips quoting from a single value.
func unquoteScalar(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if s[0] == '\'' || s[0] == '"' {
		if idx := strings.IndexByte(s[1:], s[0]); idx >= 0 {
			return s[1 : 1+idx]
		}
		return s[1:]
	}
	if idx := strings.IndexAny(s, " \t"); idx >= 0 {
		return s[:idx]
	}
	return s
}

// StripVersionConstraint reduces a dependency spec to a bare package name so it
// can be looked up: `libalpm.so>=14` becomes `libalpm.so`, `openssl<3` becomes
// `openssl`. A virtual dependency such as `java-runtime` is returned unchanged.
func StripVersionConstraint(dep string) string {
	if dep == "" {
		return ""
	}
	if idx := strings.IndexAny(dep, "<>="); idx > 0 {
		return strings.TrimSpace(dep[:idx])
	}
	return strings.TrimSpace(dep)
}
