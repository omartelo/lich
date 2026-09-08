package terminal

import (
	"bytes"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"
)

// minSearchQuery is the shortest query worth scanning transcripts for. One or
// two characters match nearly every conversation, so the result would be a list
// of every session — paid for by reading every transcript on the machine.
const minSearchQuery = 3

// searchTailBytes bounds how much of one transcript is searched, counted from
// its end. A long-running conversation runs to tens of MB of tool output, and
// the search is re-run as the user types; the bound is what keeps a keystroke
// from turning into a full disk read per session.
//
// Ceiling: a mention old enough to have fallen out of the tail is not found.
// Indexing is the fix if that ever bites — not a bigger number.
const searchTailBytes = 4 << 20

// snippetWidth is how many runes of the matched message the palette row shows.
// Wide enough to recognise the sentence, short enough for one line at palette
// width.
const snippetWidth = 160

// TranscriptMatch is one session whose conversation mentions the query: the
// session id to jump to, the newest matching message trimmed to a line, and how
// many of its messages matched.
type TranscriptMatch struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet"`
	Count   int    `json:"count"`
}

// SearchTranscripts finds which of the given sessions have talked about query,
// and what they said. The caller passes the sessions it can route to (the
// palette's own list), so nothing is searched that the user could not be sent
// to.
//
// Only the conversation a session is running now is searched: that is the one
// transcript lich can name from a session id. Whatever was said before a
// `/clear` is in a transcript this does not reach.
//
// Every miss is silent and simply contributes no match — a session with no
// provider id yet (a shell, or a provider that reports none), a provider whose
// conversation lich can read by neither mechanism (searchSource), a transcript
// that is gone, a half-written line.
func (s *Service) SearchTranscripts(ids []string, query string) []TranscriptMatch {
	q := strings.ToLower(strings.TrimSpace(query))
	if utf8.RuneCountInString(q) < minSearchQuery {
		return nil
	}
	matches := make([]TranscriptMatch, 0, len(ids))
	for _, id := range ids {
		providerSessionID, err := s.store.ProviderSession(id)
		if err != nil {
			slog.Warn("terminal: read provider session", "session", id, "err", err)
			continue
		}
		if providerSessionID == "" {
			continue
		}
		src, ok := usageSourceFor(providerSessionID, s.spawnOf(id).cwd)
		if !ok {
			continue
		}
		match, ok := searchSource(src, q)
		if !ok {
			continue
		}
		match.ID = id
		matches = append(matches, match)
	}
	return matches
}

// searchSource asks one conversation about q, by whichever of the two mechanisms
// its provider is read through: a bounded tail walked by the provider's own line
// reader, or — for the two that keep their messages in a database of their own —
// a query for the prose of that conversation (sessiondb.go). Both count the
// messages that mention q and keep the newest as the snippet; false when neither
// finds one, which is also what a provider read by neither answers.
func searchSource(src usageSource, q string) (TranscriptMatch, bool) {
	if path, read, ok := transcriptReaderFor(src); ok {
		tail, ok := readTail(path, searchTailBytes)
		if !ok {
			return TranscriptMatch{}, false
		}
		return searchTranscript(tail, q, read)
	}
	return searchSessionRows(sessionDBTexts(src.path, queriesFor(src.kind).search, src.id), q)
}

// searchSessionRows is searchTranscript's other half: the same count and the
// same newest-wins snippet, over messages a query already separated from the
// tool calls around them, so there is no line to parse and nothing to reject.
//
// The rows are matched here rather than by a `LIKE` in the query: SQLite's is
// case-insensitive over ASCII alone, and a palette that found "worktree" in one
// provider and not "Wörter" in another would be answering two questions.
func searchSessionRows(texts []string, q string) (TranscriptMatch, bool) {
	var match TranscriptMatch
	for _, text := range texts {
		snippet, ok := snippetAround(text, q)
		if !ok {
			continue
		}
		match.Snippet = snippet
		match.Count++
	}
	return match, match.Count > 0
}

// searchTranscript counts the messages in a transcript tail that mention q
// (already lowercased and non-empty) and keeps the newest one as the snippet —
// what a session is saying about the query now is what identifies it. read is
// the provider's own line reader (said.go), which is the only part of this that
// differs from one provider to the next.
//
// The cheap bytes check first is what makes the scan affordable: only a line
// that could match is parsed, so a transcript full of tool output is walked
// without unmarshalling any of it.
func searchTranscript(tail []byte, q string, read turnReader) (TranscriptMatch, bool) {
	needle := []byte(q)
	var match TranscriptMatch
	for line := range bytes.SplitSeq(tail, []byte("\n")) {
		if !bytes.Contains(bytes.ToLower(line), needle) {
			continue
		}
		t, ok := read(line)
		if !ok {
			continue
		}
		snippet, ok := snippetAround(t.text, q)
		if !ok {
			// The query is somewhere in the JSON but not in what was said — a
			// tool's arguments, a file path, an id. Not a message about it.
			continue
		}
		match.Snippet = snippet
		match.Count++
	}
	return match, match.Count > 0
}

// snippetAround renders the stretch of text around the first mention of q as
// one palette line: whitespace collapsed (a message is prose with newlines and
// code in it), a window centred a little ahead of the match so the words
// leading into it stay visible, and an ellipsis on each side that was cut.
// false when the text does not mention q at all.
func snippetAround(text, q string) (string, bool) {
	flat := strings.Join(strings.Fields(text), " ")
	at := matchRuneIndex(flat, q)
	if at < 0 {
		return "", false
	}
	runes := []rune(flat)
	if len(runes) <= snippetWidth {
		return flat, true
	}
	start := max(at-snippetWidth/3, 0)
	end := start + snippetWidth
	if end > len(runes) {
		end = len(runes)
		start = end - snippetWidth
	}
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}
	return snippet, true
}

// matchRuneIndex is where text first mentions q (already lowercased), counted in
// runes of text itself; -1 when it does not.
//
// Lowercasing is not length-preserving — "Ⱥ" lowers to a rune one byte longer,
// "İ" to two runes — so an offset read off a lowered copy can point past the end
// of the original. The lowered text is built here alongside the rune of text
// each of its bytes came from, which is what keeps the offset addressing text.
func matchRuneIndex(text, q string) int {
	var lowered strings.Builder
	lowered.Grow(len(text))
	origin := make([]int, 0, len(text))
	for index, r := range []rune(text) {
		before := lowered.Len()
		lowered.WriteRune(unicode.ToLower(r))
		for range lowered.Len() - before {
			origin = append(origin, index)
		}
	}
	at := strings.Index(lowered.String(), q)
	if at < 0 {
		return -1
	}
	return origin[at]
}
