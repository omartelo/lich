package project

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/omartelo/lich/internal/relpath"
)

// maxExpandLines caps one FileLines answer. The diff's expander asks for a
// whole gap at once and a gap can be the rest of the file, so the cap is what
// keeps one click bounded; the affordance stays over whatever is left, which
// turns a larger gap into several clicks rather than one unbounded response.
const maxExpandLines = 500

// ghRawAccept asks the contents API for the file itself rather than the JSON
// envelope, whose base64 body is refused above 1MB.
const ghRawAccept = "Accept: application/vnd.github.raw"

// FileLines returns lines from..to of one repo-relative file — 1-based and
// inclusive, the numbering a diff's gutter shows — for the review panel's
// context expander: the unchanged text around a hunk, which git never printed.
//
// ref names the revision the diff's new side stands at, and the panels differ
// in which one they have: "" is the working tree, an oid is a git object this
// checkout holds (a turn's snapshot tree), and a pull request's head is an oid
// this clone need not hold at all — measured, the head oids `gh pr list`
// reports are absent from a clone that never checked those branches out, so a
// local read alone would leave the Pulls screen's expander dead. The object is
// tried first because it costs nothing; GitHub answers for the rest.
//
// rel is validated against traversal like every other file-by-path surface, and
// binaries and oversize files are refused the way ReadFile refuses them: this
// is a source viewer, and a blob has no lines to show.
func (s *Service) FileLines(path, rel, ref string, from, to int) ([]string, error) {
	if err := relpath.Validate(rel); err != nil {
		return nil, err
	}
	if from < 1 || to < from {
		return nil, fmt.Errorf("invalid line range %d..%d", from, to)
	}
	text, err := s.fileAt(path, rel, ref)
	if err != nil {
		return nil, err
	}
	return sliceLines(text, from, to), nil
}

// fileAt reads one file whole at ref. See FileLines for what a ref can be.
func (s *Service) fileAt(path, rel, ref string) (string, error) {
	if ref == "" {
		return s.ReadFile(path, rel)
	}
	if !isOID(ref) {
		return "", fmt.Errorf("invalid revision %q", ref)
	}
	// gitQuiet and not runGit: an object this clone does not have is the
	// expected answer for a pull request's head, not a failure to report.
	if out, ok := gitQuiet(path, "show", ref+":"+rel); ok {
		return checkedText(rel, out)
	}
	out, err := s.gh(prReadTimeout, path, "api", "-H", ghRawAccept, contentsPath(rel, ref))
	if err != nil {
		return "", err
	}
	return checkedText(rel, string(out))
}

// checkedText applies ReadFile's two refusals to text that never passed through
// it — a git object, or GitHub's answer.
func checkedText(rel, text string) (string, error) {
	if len(text) > maxReadFileSize {
		return "", fmt.Errorf("%s is too large to expand (%d bytes)", rel, len(text))
	}
	if isBinary([]byte(text)) {
		return "", fmt.Errorf("%s is a binary file", rel)
	}
	return text, nil
}

// contentsPath addresses one file of the checkout's repository at a commit.
// {owner} and {repo} are gh's own placeholders, filled from the remote, and a
// fork's commit is reachable there too: GitHub keeps every pull request's head
// in the base repository.
func contentsPath(rel, ref string) string {
	segments := strings.Split(rel, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return fmt.Sprintf(
		"repos/{owner}/{repo}/contents/%s?ref=%s",
		strings.Join(segments, "/"), url.QueryEscape(ref),
	)
}

// isOID reports whether ref is a git object name — the only revision the page
// ever names, and the guard that keeps a value from the page out of git's
// option parsing and out of GitHub's URL.
func isOID(ref string) bool {
	if len(ref) < 7 || len(ref) > 64 {
		return false
	}
	for _, char := range ref {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// sliceLines takes lines from..to (1-based, inclusive) out of a file's text,
// clamped to maxExpandLines and to the end of the file. It is never nil: an
// empty answer means the range starts past the last line, and it has to cross
// the wire as [] rather than as JSON null.
func sliceLines(text string, from, to int) []string {
	lines := strings.Split(text, "\n")
	// A trailing newline ends the last line; it does not open another.
	if last := len(lines) - 1; last >= 0 && lines[last] == "" {
		lines = lines[:last]
	}
	if from > len(lines) {
		return []string{}
	}
	if to > len(lines) {
		to = len(lines)
	}
	if to-from+1 > maxExpandLines {
		to = from + maxExpandLines - 1
	}
	return lines[from-1 : to]
}
