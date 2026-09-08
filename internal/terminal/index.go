package terminal

import (
	"bufio"
	"os"
	"strings"
	"unicode/utf8"
)

// indexTextBytes caps the prose one session's index holds, newest kept. It is
// what keeps the workspace database bounded by the number of sessions rather
// than by the size of the conversations behind them.
//
// It is the only bound on this read: the transcript itself is walked whole,
// because a park happens once and off the hot path, and a tail bound on it would
// be the ceiling that actually decided how far back a parked session is
// searchable. Prose is a little over 1% of a Claude transcript's bytes, so the
// largest conversation on record here (169 MB) indexes to 427 KB and this cap
// never comes near it. What it is really there for is the two providers whose
// messages come out of a database as prose with no tool output to dilute them,
// and the conversation long enough to be pathological either way.
//
// A session the cap did cut says so on its own row, because the words it dropped
// are the oldest ones and a search that misses them has to be explicable.
const indexTextBytes = 8 << 20

// proseReadBuffer is how much of a transcript one read pulls in. The file is
// walked a line at a time rather than held whole, so peak memory is this plus
// the longest single line rather than the size of the conversation.
const proseReadBuffer = 1 << 20

// TranscriptText is one conversation as a single block of searchable prose:
// every message either side of it said, oldest first, with the tool calls and
// results between them left out. It is what the history index stores for a
// parked session, and it is read through exactly the two mechanisms the live
// search is (search.go): a walk by the provider's own line reader, or a query
// for the two providers that keep their messages in a database of their own.
//
// cut reports that indexTextBytes dropped the oldest of it, which is what a
// history row shows under its snippet: an index that does not go all the way
// back has to say so where the search that missed is read.
//
// cwd is the directory the session ran in, needed by Crush and Cursor alone.
// Empty for every miss: a provider lich can read by neither mechanism, a
// transcript that is gone, a conversation id nothing on disk matches. A caller
// records the empty answer the way it records a full one, and that is what marks
// the session indexed and keeps a backfill from asking again every time.
func TranscriptText(providerSessionID, cwd string) (text string, cut bool) {
	src, ok := usageSourceFor(providerSessionID, cwd)
	if !ok {
		return "", false
	}
	if path, read, ok := transcriptReaderFor(src); ok {
		return newestBytes(transcriptProse(path, read), indexTextBytes)
	}
	texts := sessionDBTexts(src.path, queriesFor(src.kind).search, src.id)
	return newestBytes(strings.Join(texts, "\n"), indexTextBytes)
}

// transcriptProse walks a JSONL transcript whole and joins what was said in it,
// one message per line. Streamed rather than read into memory: the conversations
// this is pointed at run to hundreds of megabytes, of which the prose is a low
// single-digit percentage, and only the prose is kept.
//
// A malformed line contributes nothing, which is also what a line still being
// written looks like.
func transcriptProse(path string, read turnReader) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	var prose strings.Builder
	lines := bufio.NewReaderSize(f, proseReadBuffer)
	for {
		// ReadBytes rather than a Scanner: a single tool result can be tens of
		// megabytes, and a Scanner stops the whole walk at the first line past
		// its buffer rather than skipping it.
		line, err := lines.ReadBytes('\n')
		if t, ok := read(line); ok && t.text != "" {
			prose.WriteString(t.text)
			prose.WriteByte('\n')
		}
		if err != nil {
			return prose.String()
		}
	}
}

// newestBytes keeps at most limit bytes from the end of text, cut at a line break
// so the index never holds half a sentence, and never inside a rune. The newest
// end is the one kept: a search is asked about work somebody remembers doing,
// and the closer to the close of a session it happened the likelier that is.
// cut is whether anything was dropped at all.
func newestBytes(text string, limit int) (string, bool) {
	if len(text) <= limit {
		return text, false
	}
	cut := text[len(text)-limit:]
	if at := strings.IndexByte(cut, '\n'); at >= 0 {
		cut = cut[at+1:]
	}
	// A cut between lines is already rune-aligned; one that found no line break
	// at all (a single message longer than the cap) is not.
	for len(cut) > 0 && !utf8.RuneStart(cut[0]) {
		cut = cut[1:]
	}
	return cut, true
}
