package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// maxSearchMatches caps one search's answer. The list is read by a person in a
// dock column, and a query loose enough to pass this is one to narrow, not to
// scroll; SearchResult.Cut says the cap was hit.
const maxSearchMatches = 500

// maxExcerptBytes caps the line text a match carries, and excerptLeadBytes is
// how much of the line before the match survives a cut. A minified bundle is
// one line of megabytes, and the dock row has room for a sentence.
const (
	maxExcerptBytes  = 240
	excerptLeadBytes = 40
)

// SearchMatch is one line of one file that contains the query.
type SearchMatch struct {
	Path string `json:"path"`
	// Line is 1-based, the same numbering the preview's gutter shows.
	Line int `json:"line"`
	// Text is the line with its indentation dropped, cut to an excerpt around
	// the first occurrence when the line is long.
	Text string `json:"text"`
}

// SearchResult is one search over a checkout's files.
type SearchResult struct {
	Matches []SearchMatch `json:"matches"`
	// Cut reports a search that stopped at maxSearchMatches.
	Cut bool `json:"cut"`
	// TooLarge counts the listed files skipped for being above maxReadFileSize.
	// The tree shows them, so an answer that left them out says so; a binary
	// is skipped without a count, since nobody searches one for text.
	TooLarge int `json:"tooLarge"`
}

// errTooLarge marks a listed file readSearchable skipped for its size, the one
// skip SearchResult counts.
var errTooLarge = errors.New("file too large to search")

// Search finds the lines that contain query, case-insensitively and as a
// literal, across the files Tree lists for path. It reads Tree's listing rather
// than running git grep so that every hit is a file the tree beside it shows,
// in a repository and in a plain folder alike. A file the preview would refuse
// (binary, irregular, above maxReadFileSize) is skipped, so every hit opens.
// Matches come in Tree's order, file by file, line by line.
func (s *Service) Search(path, query string) (SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return SearchResult{}, errors.New("search query is empty")
	}
	// Lowercasing both sides instead of a (?i) regexp: measured over a 25MB
	// checkout, the regexp spent 660ms where this spends 60ms.
	needle := bytes.ToLower([]byte(query))
	listing, err := s.Tree(path)
	if err != nil {
		return SearchResult{}, err
	}
	result := SearchResult{Matches: []SearchMatch{}}
	for _, rel := range listing.Files {
		data, err := readSearchable(path, rel)
		if errors.Is(err, errTooLarge) {
			result.TooLarge++
			continue
		}
		if err != nil {
			return SearchResult{}, err
		}
		if !bytes.Contains(bytes.ToLower(data), needle) {
			continue
		}
		if result.Cut = appendMatches(&result.Matches, rel, data, string(needle)); result.Cut {
			break
		}
	}
	return result, nil
}

// readSearchable returns a listed file's bytes, or nil for a file the search
// passes over: one the preview refuses, and one that went away or locked up
// between the listing and the read, which an agent writing into the checkout
// makes routine. A file above maxReadFileSize is errTooLarge.
func readSearchable(root, rel string) ([]byte, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", rel, err)
	}
	// Lstat, so a symlink is irregular and skipped: ReadFile refuses one that
	// leaves the checkout, and following none is the simplest way to agree.
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	if info.Size() > maxReadFileSize {
		return nil, errTooLarge
	}
	data, err := os.ReadFile(full)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	if isBinary(data) {
		return nil, nil
	}
	return data, nil
}

// appendMatches adds rel's matching lines to matches and reports whether the
// list reached maxSearchMatches. needle is the query already lowercased.
func appendMatches(matches *[]SearchMatch, rel string, data []byte, needle string) bool {
	for i, line := range bytes.Split(data, []byte("\n")) {
		text := strings.TrimLeft(strings.TrimSuffix(string(line), "\r"), " \t")
		at := strings.Index(strings.ToLower(text), needle)
		if at < 0 {
			continue
		}
		*matches = append(*matches, SearchMatch{Path: rel, Line: i + 1, Text: excerpt(text, at)})
		if len(*matches) >= maxSearchMatches {
			return true
		}
	}
	return false
}

// excerpt cuts a long line down to maxExcerptBytes starting a little before
// the match at byte offset at, on rune boundaries, marking each cut side. at
// is an offset into the lowercased line, which a few runes lengthen, so it is
// clamped rather than trusted.
func excerpt(text string, at int) string {
	if len(text) <= maxExcerptBytes {
		return text
	}
	start := min(len(text)-1, max(0, at-excerptLeadBytes))
	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}
	end := min(len(text), start+maxExcerptBytes)
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end--
	}
	cut := text[start:end]
	if start > 0 {
		cut = "…" + cut
	}
	if end < len(text) {
		cut += "…"
	}
	return cut
}
