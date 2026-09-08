package store

import (
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
// over the match: a snippet costs reading the conversation it is cut out of, and
// a term can match more sessions than one page carries.
//
// The window is cut here rather than by the index's own snippet(): that function
// walks every instance of the term in the document, which on a conversation that
// says "session" three thousand times is a tenth of a second per row. A row
// whose conversation cannot be read, or whose hit is a fold of the index's own
// (accents are folded, so "ción" is indexed as "cion" and found by either),
// keeps its place in the list with no snippet under it.
func (s *Service) attachSnippets(sessions []ClosedSession, ids []string, term string) {
	if len(ids) == 0 {
		return
	}
	bodies := s.conversationBodies(ids)
	// Split the way the index was asked (ftsQuery): "worktree/port" matched the
	// conversation as two words, and is two words here or no snippet is found.
	words := searchWords(strings.ToLower(term))
	for i := range sessions {
		body, ok := bodies[sessions[i].ID]
		if !ok {
			continue
		}
		for _, word := range words {
			if cut, ok := snippet.Around(around(body, word), word); ok {
				sessions[i].Snippet = cut
				break
			}
		}
	}
}

// windowBytes is how much of a conversation the snippet is cut out of, centred
// on a first, approximate reading of where the word is. Wide enough that the
// exact cut has the whole sentence around the hit to work with, and narrow
// enough that a two-megabyte conversation is not walked rune by rune once per
// row on screen.
const windowBytes = 4096

// around is that first reading: the stretch of body near the first mention of
// word, found by one case-folded scan and sliced out of the original. The offset
// is approximate, because folding is not length-preserving, and it does not need
// to be exact: it only has to land the mention inside a window the real cut then
// searches properly. Empty when the body does not mention the word at all, which
// is a hit the index found by a fold the plain text does not have.
func around(body, word string) string {
	// The plain scan first: word is already lowercased and a conversation is
	// mostly lowercase prose, so the copy the fold below needs is one this
	// almost always avoids. On a page of megabyte conversations that copy is the
	// larger half of the whole search.
	at := strings.Index(body, word)
	if at < 0 {
		at = strings.Index(strings.ToLower(body), word)
	}
	if at < 0 {
		return ""
	}
	start := max(at-windowBytes/2, 0)
	end := min(at+windowBytes/2, len(body))
	// Sliced off a byte offset, so either edge can fall inside a rune. A
	// half rune left at one would be drawn as a replacement character.
	for start < end && !utf8.RuneStart(body[start]) {
		start++
	}
	for end > start {
		if r, size := utf8.DecodeLastRuneInString(body[start:end]); r != utf8.RuneError || size > 1 {
			break
		}
		end--
	}
	return body[start:end]
}

// conversationBodies reads the indexed conversations of these sessions, keyed by
// session id. A body that is not there is simply absent from the answer, which
// is a row with no snippet rather than a search that failed.
func (s *Service) conversationBodies(ids []string) map[string]string {
	query := `SELECT session_id, body FROM session_texts WHERE session_id IN (?` +
		strings.Repeat(",?", len(ids)-1) + `)`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		slog.Warn("store: read indexed conversations", "err", err)
		return nil
	}
	defer rows.Close()
	bodies := make(map[string]string, len(ids))
	for rows.Next() {
		var id, body string
		if err := rows.Scan(&id, &body); err != nil {
			slog.Warn("store: scan indexed conversation", "err", err)
			return bodies
		}
		bodies[id] = body
	}
	if err := rows.Err(); err != nil {
		slog.Warn("store: iterate indexed conversations", "err", err)
	}
	return bodies
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
