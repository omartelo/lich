package store

import (
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/omartelo/lich/internal/snippet"
)

// backfillRate is how long the lazy backfill waits between one parked session
// and the next. Each one is a bounded transcript read (terminal.TranscriptText)
// and a write into a database with a single connection, so the pause is what
// keeps a machine holding hundreds of parked sessions from spending its disk on
// a search somebody is still typing.
const backfillRate = 100 * time.Millisecond

// SetTranscriptOf registers how to read a conversation as searchable prose
// (terminal.TranscriptText), given the provider's own conversation id and the
// directory the session ran in. The second answer is whether the cap dropped the
// oldest of that conversation, which the row it is written to carries.
//
// It is startup wiring, called before anything serves. Without it a session
// parks with no index and the history search matches names alone, which is what
// every test that does not wire it gets, and what lich did before the index
// existed.
func (s *Service) SetTranscriptOf(fn func(providerSessionID, cwd string) (text string, cut bool)) {
	s.transcriptOf = fn
}

// indexTranscript records what one session said, under its own id, replacing
// whatever was there. It is called when a session is parked and by the backfill,
// and it writes a row either way: an empty one for a shell, for a provider lich
// can read by neither mechanism, for a transcript that is gone. That empty row
// is what marks the session indexed; without it the backfill would ask the disk
// about the same session on every search forever.
func (s *Service) indexTranscript(sessionID string) {
	if s.transcriptOf == nil {
		return
	}
	var text string
	var cut bool
	if providerSessionID, cwd := s.indexTarget(sessionID); providerSessionID != "" {
		text, cut = s.transcriptOf(providerSessionID, cwd)
	}
	if _, err := s.db.Exec(`DELETE FROM session_texts WHERE session_id = ?`, sessionID); err != nil {
		slog.Warn("store: clear transcript index", "session", sessionID, "err", err)
		return
	}
	if _, err := s.db.Exec(
		`INSERT INTO session_texts(session_id, truncated, body) VALUES (?, ?, ?)`,
		sessionID, cut, text,
	); err != nil {
		slog.Warn("store: write transcript index", "session", sessionID, "err", err)
	}
}

// attachSnippets fills in the words that made each of these rows a hit, for the
// ids the search matched on their conversation. It runs over the page and not
// over the match: a snippet costs reading part of the conversation it is cut out
// of, and a term can match more sessions than one page carries.
//
// One query per word of the term, over the rows still without a snippet: a term
// is almost always one word, and a session is windowed on the first of them its
// conversation mentions, which is the word the snippet is then centred on. A row
// whose conversation mentions none of them (a hit the index found by a fold of
// its own, since accents are folded and "ción" is indexed as "cion") keeps its
// place in the list with no snippet under it.
func (s *Service) attachSnippets(sessions []ClosedSession, ids []string, term string) {
	if len(ids) == 0 {
		return
	}
	row := make(map[string]int, len(sessions))
	for i := range sessions {
		row[sessions[i].ID] = i
	}
	// Split the way the index was asked (ftsQuery): "worktree/port" matched the
	// conversation as two words, and is two words here or no snippet is found.
	pending := ids
	for _, word := range searchWords(strings.ToLower(term)) {
		if len(pending) == 0 {
			return
		}
		windows := s.conversationWindows(pending, word)
		var missed []string
		for _, id := range pending {
			i, ok := row[id]
			if !ok {
				continue
			}
			cut, ok := snippet.Around(windows[id], word)
			if !ok {
				missed = append(missed, id)
				continue
			}
			sessions[i].Snippet = cut
		}
		pending = missed
	}
}

// windowChars is how much of a conversation the snippet is cut out of, centred
// on the first mention of the word. Wide enough that the exact cut has the whole
// sentence around the hit to work with even after collapsing the whitespace a
// transcript is written with, and narrow enough that a page of a hundred rows is
// hundreds of kilobytes rather than the hundreds of megabytes the conversations
// behind it hold.
const windowChars = 4096

// firstMention is where a body first mentions the bound word, as SQL: one
// position, or 0 for a body that does not mention it at all. Characters, not
// bytes: instr and substr both count in characters, so a window cut off this
// offset never opens or closes inside a rune.
//
// The plain scan first, because coalesce stops at the first argument that is not
// null: a conversation is mostly lowercase prose and the word is already
// lowercased, so the folded copy of a multi-megabyte body is one this almost
// always avoids. The fold itself is SQLite's, which is ASCII: a body that shouts
// an accented word in capitals is a hit with no snippet, the same answer the
// index's own accent fold already gives.
const firstMention = `coalesce(nullif(instr(body, ?), 0), instr(lower(body), ?))`

// conversationWindows cuts, in the query, the stretch of each of these
// conversations around its first mention of word, keyed by session id. A body
// that is not indexed or does not mention the word is simply absent from the
// answer, which is a row with no snippet rather than a search that failed.
//
// The window is what keeps a search off the size of the conversations it
// searches: the rows are matched on bodies of up to eight megabytes each and the
// answer carries windowChars characters of each, so the page a settled keystroke
// reads is bounded by the page's own length.
func (s *Service) conversationWindows(ids []string, word string) map[string]string {
	query := `SELECT session_id, substr(body, max(` + firstMention + ` - ?, 1), ?)
	            FROM session_texts
	           WHERE ` + firstMention + ` > 0
	             AND session_id IN (?` + strings.Repeat(",?", len(ids)-1) + `)`
	// Bound in the order the query names them: the window's own mention, its
	// two bounds, the filter's mention, then the ids.
	args := []any{word, word, windowChars / 2, windowChars, word, word}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		slog.Warn("store: window indexed conversations", "err", err)
		return nil
	}
	defer rows.Close()
	windows := make(map[string]string, len(ids))
	for rows.Next() {
		var id, window string
		if err := rows.Scan(&id, &window); err != nil {
			slog.Warn("store: scan conversation window", "err", err)
			return windows
		}
		windows[id] = window
	}
	if err := rows.Err(); err != nil {
		slog.Warn("store: iterate conversation windows", "err", err)
	}
	return windows
}

// indexTarget is what reading one session's conversation takes: the id its
// provider filed the conversation under, and the directory the session ran in:
// its own worktree, else its project's. Two empty strings for a session with no
// conversation to read, which is every shell and every row a provider never
// reported an id for.
func (s *Service) indexTarget(sessionID string) (providerSessionID, cwd string) {
	if err := s.db.QueryRow(
		`SELECT s.provider_session_id,
		        CASE WHEN s.path <> '' THEN s.path ELSE p.path END
		   FROM sessions s JOIN projects p ON p.id = s.project_id
		  WHERE s.id = ?`,
		sessionID,
	).Scan(&providerSessionID, &cwd); err != nil {
		return "", ""
	}
	return providerSessionID, cwd
}

// unindexed counts the parked sessions with no index row: the ones parked before
// lich wrote one, which is every row in a workspace meeting this version for the
// first time. It is what the History tab's header reports while the
// backfill runs, so a search that is still missing rows says so instead of
// answering short in silence.
func (s *Service) unindexed() int {
	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sessions
		  WHERE is_open = 0 AND id NOT IN (SELECT session_id FROM session_texts)`,
	).Scan(&n); err != nil {
		slog.Warn("store: count unindexed sessions", "err", err)
		return 0
	}
	return n
}

// backfill indexes the parked sessions that predate the index, newest close
// first, one at a time with a pause between them. It is started by the first
// history search that carries a term and runs until every row has one, never at
// launch, where it would spend a machine's disk on a list nobody has asked for.
//
// Only one runs at a time, and it stops when the store closes: every search
// while it is working starts it again and the guard turns that into nothing.
//
// The start is taken under the same lock Close closes the store under, so a
// backfill never begins after Close has started waiting: a WaitGroup added to
// while it is being waited on at zero is the one thing the type forbids, and
// the goroutine it would let through writes into a closed database.
func (s *Service) backfill() {
	if s.transcriptOf == nil {
		return
	}
	s.backfillMu.Lock()
	defer s.backfillMu.Unlock()
	select {
	case <-s.stopped:
		return
	default:
	}
	if !s.backfilling.CompareAndSwap(false, true) {
		return
	}
	s.backfillDone.Add(1)
	go func() {
		defer s.backfillDone.Done()
		defer s.backfilling.Store(false)
		for {
			sessionID, ok := s.nextUnindexed()
			if !ok {
				return
			}
			s.indexTranscript(sessionID)
			select {
			case <-s.stopped:
				return
			case <-time.After(backfillRate):
			}
		}
	}()
}

// nextUnindexed names the parked session the backfill should read next: the most
// recently closed one with no index row, because that is the one a search is
// likeliest to be looking for. False when there are none left, which is what ends
// the backfill.
func (s *Service) nextUnindexed() (string, bool) {
	var sessionID string
	if err := s.db.QueryRow(
		`SELECT id FROM sessions
		  WHERE is_open = 0 AND id NOT IN (SELECT session_id FROM session_texts)
		  ORDER BY closed_at DESC, rowid DESC LIMIT 1`,
	).Scan(&sessionID); err != nil {
		return "", false
	}
	return sessionID, true
}

// ftsQuery turns a user's search term into an FTS5 MATCH expression: every word
// as a quoted prefix phrase, ANDed, so "work tree" finds a conversation that
// mentions both somewhere. Empty when the term holds nothing to match on, which
// is the caller's signal to search names alone.
//
// The term is split on everything that is not a letter or a digit (the same cut
// FTS5's own tokenizer makes) rather than on whitespace: "use-history-search"
// is three tokens in the index, and one quoted phrase of the whole hyphenated
// string would match nothing. Splitting it here is also what makes the quoting
// safe: no character that means anything to the MATCH grammar can survive it, so
// a term full of quotes and parentheses is a search rather than a syntax error.
//
// Prefix rather than whole-word because the name search beside it matches
// substrings: a query that found "worktree" by name and not by conversation
// would be answering two questions.
func ftsQuery(term string) string {
	words := searchWords(term)
	phrases := make([]string, 0, len(words))
	for _, word := range words {
		phrases = append(phrases, `"`+word+`"*`)
	}
	return strings.Join(phrases, " ")
}

// searchWords is a term cut the way FTS5's own tokenizer cuts the text it
// indexes: on everything that is not a letter or a digit. Both halves of the
// conversation search read the term through it, the query that asks the index
// and the snippet that shows the hit, so they agree on what a word is.
func searchWords(term string) []string {
	return strings.FieldsFunc(term, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
