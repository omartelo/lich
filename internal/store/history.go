package store

import (
	"fmt"
	"strings"
)

// ClosedSession is one parked session offered for resuming — what identifies it
// in a list somebody is browsing rather than one they are already looking at.
// The project rides along because history spans every project at once, closed
// ones included, and the project name is what tells two sessions of the same
// name apart.
//
// Two answers about the branch, and they are different questions: ParkedBranch
// is the snapshot this row was closed on, which is what the search matches, and
// what the window draws is read live off the checkout, because a worktree's
// directory stops agreeing with its branch the moment an agent branches inside
// it (frontend/src/lib/git/checkout-label.ts). A row whose checkout is gone has
// nothing live to show — which is the same row that cannot be resumed anyway.
type ClosedSession struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	// The project's own directory, so resuming a session of a closed project can
	// reopen that project first without a second lookup — and can ask where it
	// went when the directory has moved.
	ProjectPath string `json:"projectPath"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	// The branch the checkout was on at the close, empty for a row parked before
	// lich recorded one. Searchable, not drawn.
	ParkedBranch string `json:"parkedBranch"`
	// Unix seconds, 0 for a row parked before closed_at existed — which sorts
	// last and is drawn as no date rather than as 1970.
	ClosedAt int64 `json:"closedAt"`
	// The stretch of conversation that matched the search, empty for a row the
	// search matched by name, and for every row of an empty query, which
	// matched nothing in particular.
	Snippet string `json:"snippet"`
	// Whether the cap on one session's index dropped the oldest of this
	// conversation. The row says so on screen: a search that reaches only part
	// of a session has to be readable as that rather than as an absence.
	Truncated bool `json:"truncated"`
}

// ClosedHistory is one page of the parked-session history and the size of the
// match it was cut from. Total is what lets the list say it was cut instead of
// presenting closedSessionLimit rows as the whole answer.
type ClosedHistory struct {
	Sessions []ClosedSession `json:"sessions"`
	Total    int             `json:"total"`
	// Parked sessions whose conversation has not been indexed yet, which is what
	// the list reports while the backfill catches up on a workspace that predates
	// the index. 0 once every row has one, which is the steady state.
	Indexing int `json:"indexing"`
}

// closedSessionLimit caps the history handed to the window — how many rows one
// call answers with, not how far back it looks: the search runs in the query,
// so a match older than the newest hundred is still found. The alternative is
// handing over every session ever closed to draw a list nobody scrolls to the
// end of.
const closedSessionLimit = 100

// likeEscape is the ESCAPE character the searches declare, so a name containing
// % or _ searches for those characters instead of matching anything.
const likeEscape = `\`

// escapeLike neutralises the LIKE wildcards in a user's search term. The escape
// character goes first, or escaping the wildcards would re-escape it.
func escapeLike(term string) string {
	term = strings.ReplaceAll(term, likeEscape, likeEscape+likeEscape)
	term = strings.ReplaceAll(term, "%", likeEscape+"%")
	return strings.ReplaceAll(term, "_", likeEscape+"_")
}

// ClosedSessions returns the parked sessions (is_open = 0) matching term, the
// last one closed first, up to closedSessionLimit of them. An empty term is the
// plain history: the most recently closed, whatever they are named. Sessions of
// closed projects answer too: a project hidden from the tab strip still owns the
// work done in it, and resuming one of its sessions is what reopens it.
//
// A row answers on either of two matches, and the OR between them is the whole
// point: every whitespace-separated word appearing in the name, the project's
// name, the path or the branch it was parked on (the same reading the palette's
// own filter gives a query), or the conversation itself mentioning them, out of
// the FTS index the close wrote (transcripts.go). A row matched on its
// conversation carries the Snippet that made it one, and one matched by name
// carries none.
//
// Both are matched here rather than in the window because the window only ever
// sees one page of rows, and a session parked further back than that page would
// be unfindable. LIKE is case-insensitive for ASCII in SQLite, which is what
// makes the name half a search and not a prefix test.
//
// Total counts the whole match, not the page: without it a term matching more
// parked sessions than fit would answer with closedSessionLimit rows and no way
// for the list to say so. Indexing counts the parked sessions the backfill has
// not reached yet, which is the other way this answer can be short; asking for
// one with a term is what starts that backfill.
//
// rowid is the tiebreak, not the order, for RecentProjects' reason twice over:
// it dates the insert, and a resumed session is reinserted — so rows parked
// before closed_at existed all carry 0 and fall back to it together.
func (s *Service) ClosedSessions(term string) (ClosedHistory, error) {
	where := "WHERE s.is_open = 0"
	with, join, args := "", "", []any{}

	// The match is MATERIALIZED, and not a subquery inlined into the join:
	// without it SQLite runs the full-text search again for every parked row it
	// walks, which on a workspace of two hundred sessions costs tens of seconds
	// instead of tens of milliseconds.
	//
	// It is also written before the WHERE it feeds, so its argument is bound
	// first: the one place the order of these two halves matters.
	if strings.TrimSpace(term) != "" {
		// Started on any term, not only one the index can be asked about: the
		// count of what is still unindexed goes back with every answer, and a
		// term made of punctuation alone would otherwise leave the list
		// reporting a backfill nothing had started.
		s.backfill()
	}
	if match := ftsQuery(term); match != "" {
		with = `WITH said AS MATERIALIZED (
		          SELECT t.session_id FROM session_texts t
		            JOIN session_texts_fts f ON f.rowid = t.rowid
		           WHERE session_texts_fts MATCH ?)`
		join = "LEFT JOIN said m ON m.session_id = s.id"
		args = append(args, match)
	}
	if names := strings.Fields(term); len(names) > 0 {
		clauses := make([]string, 0, len(names))
		for _, word := range names {
			clauses = append(clauses,
				"(s.label || ' ' || p.name || ' ' || s.path || ' ' || s.parked_branch)"+
					" LIKE ? ESCAPE '"+likeEscape+"'")
			args = append(args, "%"+escapeLike(word)+"%")
		}
		matched := "(" + strings.Join(clauses, " AND ") + ")"
		if join != "" {
			matched += " OR m.session_id IS NOT NULL"
		}
		where += " AND (" + matched + ")"
	}
	args = append(args, closedSessionLimit)

	// Counted before the rows are asked for, never while they are being walked:
	// the store holds a single connection, and a second query opened inside the
	// first one's iteration waits for a connection the first will not give back.
	indexing := s.unindexed()

	// COUNT(*) OVER () rides along on every row rather than costing a second
	// query: a window function is computed before LIMIT, so it counts the match
	// and not the page.
	rows, err := s.db.Query(
		with+`
		 SELECT s.id, s.project_id, p.name, p.path, s.label, s.kind, s.path,
		        s.parked_branch, s.closed_at, `+saidColumn(join)+`,
		        COALESCE(x.truncated, 0), COUNT(*) OVER ()
		   FROM sessions s JOIN projects p ON p.id = s.project_id
		   LEFT JOIN session_texts x ON x.session_id = s.id
		   `+join+`
		   `+where+`
		  ORDER BY s.closed_at DESC, s.rowid DESC
		  LIMIT ?`,
		args...,
	)
	if err != nil {
		return ClosedHistory{}, fmt.Errorf("query closed sessions: %w", err)
	}
	defer rows.Close()

	history := ClosedHistory{Sessions: []ClosedSession{}, Indexing: indexing}
	var said []string
	for rows.Next() {
		var c ClosedSession
		var matchedConversation bool
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.ProjectName, &c.ProjectPath,
			&c.Label, &c.Kind, &c.Path, &c.ParkedBranch, &c.ClosedAt,
			&matchedConversation, &c.Truncated, &history.Total,
		); err != nil {
			return ClosedHistory{}, fmt.Errorf("scan closed session: %w", err)
		}
		if matchedConversation {
			said = append(said, c.ID)
		}
		history.Sessions = append(history.Sessions, c)
	}
	if err := rows.Err(); err != nil {
		return ClosedHistory{}, fmt.Errorf("iterate closed sessions: %w", err)
	}
	// After the rows are drained, for the single connection's reason above, and
	// only for the page: cutting a snippet costs the conversation it is cut out
	// of, and the match can be wider than the list.
	s.attachSnippets(history.Sessions, said, term)
	return history, nil
}

// saidColumn is what the query reports into the snippet pass: whether this row
// was matched on its conversation. A query with no match has no m to read, and a
// term nothing could be searched for never gets one, so every row answers false
// and no conversation is read at all.
func saidColumn(join string) string {
	if join == "" {
		return "0"
	}
	return "m.session_id IS NOT NULL"
}
