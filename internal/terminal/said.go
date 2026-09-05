package terminal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/omartelo/lich/internal/providers"
)

// saidRuneCap bounds the closing words the panel is handed. A turn can end in a
// full report, and the panel is polled while it is open, so the whole of one is
// not worth pushing over the socket every two seconds. What is cut is the middle
// of a long answer, never the start — the first paragraph is the recap.
const saidRuneCap = 4000

// LastSaid is the prose the agent ended its last turn with, as the Review
// panel's "Last turn" shows it beside the diff. Text is empty for every absence
// there is — a provider whose conversation lich cannot read, a turn that ended
// in a tool call, a transcript still being written — because the panel draws
// nothing either way and a reason it cannot act on is a reason not worth wiring.
type LastSaid struct {
	Text string `json:"text,omitempty"`
}

// The two sides of a conversation, spelled the way Claude Code's transcript
// spells them. Every other provider's spelling is mapped onto these by its own
// reader, so a caller filters on one vocabulary rather than five.
const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// turn is one message of a conversation, read out of whatever shape its provider
// files: who said it, and what they said as plain text. It carries no timestamp
// and no id — a reader walks a transcript in order, so position is what dates a
// turn, and the two callers here (the last-turn recap, the palette's transcript
// search) both want the words and nothing else.
type turn struct {
	role string
	text string
}

// turnReader reads one line of a provider's JSONL transcript. false for every
// line that is not a message the user or the agent wrote — tool calls and their
// results, thinking, meta lines, and a line the tail cut in half.
type turnReader func(line []byte) (turn, bool)

// textBlock is the shape Claude Code, Codex and oh-my-pi share for one block of
// a message: a type naming what the block is, and the text when it is prose.
type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// LastTurnSaid returns the last thing the agent said in this session's
// conversation, as plain text.
//
// It answers the half of "what happened while I was away" that the diff beside
// it cannot: a turn that ran the suite and reported three failures changed
// nothing on disk, and that is exactly the turn worth catching up on. Nothing
// here summarises anything — the agent wrote this sentence itself, and lich only
// finds it.
//
// It is the last message on record, not the last message of the turn the diff
// brackets. The two agree whenever a turn has finished, which is the only time
// the panel offers a diff at all; mid-turn this is still the previous turn's
// words while the diff reads "unavailable", and the card's own spinner is what
// says why.
func (s *Service) LastTurnSaid(id string) (LastSaid, error) {
	providerSessionID, err := s.store.ProviderSession(id)
	if err != nil {
		return LastSaid{}, fmt.Errorf("read provider session: %w", err)
	}
	if providerSessionID == "" {
		return LastSaid{}, nil
	}
	src, ok := usageSourceFor(providerSessionID, s.spawnOf(id).cwd)
	if !ok {
		return LastSaid{}, nil
	}
	return LastSaid{Text: capRunes(saidFor(src), saidRuneCap)}, nil
}

// transcriptReaderFor pairs a conversation's JSONL transcript with the reader
// that parses its lines. It is the whole of what lich knows about reading a
// provider's conversation, and both readers of one go through it: the last-turn
// recap below, and the palette's transcript search (search.go).
//
// false for the two providers with no JSONL to walk. opencode keeps its messages
// in SQLite, which each caller queries for what it needs; Cursor CLI files a
// chat as a content-addressed blob store whose order lives in a protobuf index,
// so nothing here reaches it at all (docs/ceilings.md).
func transcriptReaderFor(src usageSource) (string, turnReader, bool) {
	switch src.kind {
	case providers.Claude:
		return src.path, claudeTurn, true
	case providers.Codex:
		return src.path, codexTurn, true
	case providers.OMP:
		return src.path, ompTurn, true
	case providers.Kiro:
		// usageSourceFor resolves the metadata `.json` the context readout
		// needs; the turns are in the `.jsonl` beside it.
		return kiroTranscriptPath(src.path), kiroTurn, true
	case providers.Antigravity:
		return src.path, antigravityTurn, true
	}
	return "", nil, false
}

// saidFor reads one conversation's closing words out of whatever the provider
// files them in. Empty for every miss, and for the two providers that reach here
// with nothing to read — Cursor CLI files its chat as a content-addressed blob
// store, and neither it nor Crush reports a turn boundary, so neither is ever
// offered the panel this feeds (docs/ceilings.md).
func saidFor(src usageSource) string {
	if path, read, ok := transcriptReaderFor(src); ok {
		return lastSaidInTail(path, read)
	}
	if src.kind == providers.OpenCode {
		return sessionDBSaid(src.path, opencodeSaidQuery, src.id)
	}
	return ""
}

// lastSaidInTail walks a JSONL transcript's tail and keeps the last thing the
// agent said in it. The tail is bounded for the reason the transcript search
// bounds its own (searchTailBytes): a long conversation runs to tens of
// megabytes of tool output, and this is re-read while the panel is open.
//
// Ceiling: a turn whose closing words fall outside that tail reads as no words
// at all. It takes a single tool result larger than the bound to do it, and the
// honest answer then is the empty one — the alternative is showing an older
// turn's words under this turn's heading.
func lastSaidInTail(path string, read turnReader) string {
	if path == "" {
		return ""
	}
	tail, ok := readTail(path, searchTailBytes)
	if !ok {
		return ""
	}
	var last string
	for _, line := range strings.Split(string(tail), "\n") {
		// A tail cut mid-line fails to parse like any malformed one, which is
		// why nothing here distinguishes the two.
		if t, ok := read([]byte(line)); ok && t.role == roleAssistant {
			last = t.text
		}
	}
	return strings.TrimSpace(last)
}

// claudeTurn reads a Claude transcript line. Sidechain lines — a sub-agent's own
// conversation — are skipped because they are not the conversation the user had:
// their last word is not the session's, and a search hit in one points at a
// message the session never showed.
func claudeTurn(line []byte) (turn, bool) {
	var entry struct {
		Type        string `json:"type"`
		IsSidechain bool   `json:"isSidechain"`
		Message     struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return turn{}, false
	}
	if entry.IsSidechain || (entry.Type != roleUser && entry.Type != roleAssistant) {
		return turn{}, false
	}
	// A user turn is usually a bare string; an assistant turn is always a block
	// list, and so is a user turn carrying tool results.
	var plain string
	if err := json.Unmarshal(entry.Message.Content, &plain); err == nil {
		return turn{role: entry.Type, text: plain}, plain != ""
	}
	var blocks []textBlock
	if err := json.Unmarshal(entry.Message.Content, &blocks); err != nil {
		return turn{}, false
	}
	text, ok := joinText(blocks, "text")
	return turn{role: entry.Type, text: text}, ok
}

// codexTurn reads a Codex rollout line. The two sides name their blocks
// differently — the agent writes `output_text` and the user's turn arrives as
// `input_text` — and both are taken here, because the reader that only knew one
// would be blind to half the conversation.
//
// The `developer` role is left out on purpose: Codex files the system prompt and
// the skills preamble under it, which is not something anybody said (measured
// against a real rollout).
func codexTurn(line []byte) (turn, bool) {
	var entry struct {
		Type    string `json:"type"`
		Payload struct {
			Type    string      `json:"type"`
			Role    string      `json:"role"`
			Content []textBlock `json:"content"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return turn{}, false
	}
	role := entry.Payload.Role
	if entry.Type != "response_item" || entry.Payload.Type != "message" ||
		(role != roleUser && role != roleAssistant) {
		return turn{}, false
	}
	text, ok := joinText(entry.Payload.Content, "output_text", "input_text")
	return turn{role: role, text: text}, ok
}

// ompTurn reads an oh-my-pi transcript line. Its assistant turns are mostly
// thinking plus a tool call — only the turn that ends the run carries a `text`
// block. The role is on the message rather than on the envelope, and a tool
// result arrives under a role of its own, so neither side is guessed here.
func ompTurn(line []byte) (turn, bool) {
	var entry struct {
		Type    string `json:"type"`
		Message struct {
			Role    string      `json:"role"`
			Content []textBlock `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return turn{}, false
	}
	role := entry.Message.Role
	if entry.Type != "message" || (role != roleUser && role != roleAssistant) {
		return turn{}, false
	}
	text, ok := joinText(entry.Message.Content, "text")
	return turn{role: role, text: text}, ok
}

// kiroTurn reads a Kiro CLI transcript line. Its envelope names the entry kind
// in PascalCase and the blocks inside it in lower case, and a block's payload
// sits under `data` rather than under a name of its own.
//
// That payload is typed per block kind — a `text` block's is a string, and a
// `thinking` block's is an object — so it is decoded per block rather than
// declared as a string on the struct. Declaring it fails the whole line the
// moment a turn thinks before it speaks, which is most of them: the text is
// parsed and then thrown away with the error (measured against a real 2.21.0
// transcript, where every assistant turn but the first carried both).
func kiroTurn(line []byte) (turn, bool) {
	var entry struct {
		Kind string `json:"kind"`
		Data struct {
			Content []struct {
				Kind string          `json:"kind"`
				Data json.RawMessage `json:"data"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return turn{}, false
	}
	var role string
	switch entry.Kind {
	case "AssistantMessage":
		role = roleAssistant
	case "Prompt":
		role = roleUser
	default:
		return turn{}, false
	}
	var parts []string
	for _, block := range entry.Data.Content {
		var text string
		if block.Kind != "text" || json.Unmarshal(block.Data, &text) != nil || text == "" {
			continue
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return turn{}, false
	}
	return turn{role: role, text: strings.Join(parts, "\n")}, true
}

// antigravityTurn reads an Antigravity transcript line. `PLANNER_RESPONSE` is
// the model's own prose and `USER_INPUT` the prompt it answered; both carry a
// bare string. The `GENERIC` entries that outnumber them carry tool output —
// spinner frames and command results — which is what a reader matching on
// `source: MODEL` alone would surface instead (measured against a real 1.1.19
// transcript).
func antigravityTurn(line []byte) (turn, bool) {
	var entry struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(line, &entry); err != nil || entry.Content == "" {
		return turn{}, false
	}
	switch entry.Type {
	case "PLANNER_RESPONSE":
		return turn{role: roleAssistant, text: entry.Content}, true
	case "USER_INPUT":
		return turn{role: roleUser, text: entry.Content}, true
	}
	return turn{}, false
}

// joinText joins the blocks of a message whose type is one of want. false when
// the message holds none of them — an assistant turn that is all thinking and
// tool calls, or a user turn that is a tool result.
func joinText(blocks []textBlock, want ...string) (string, bool) {
	var parts []string
	for _, block := range blocks {
		if block.Text != "" && slices.Contains(want, block.Type) {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n"), true
}

// opencodeSaidQuery is the newest text part of the newest assistant message in
// one opencode conversation. opencode splits a message across `part` rows and
// keeps the role on the `message` row, so the join is what tells an assistant's
// own words from a tool result written beside them.
//
// Sub-agents are left out on purpose, where the cost query walks down into them:
// opencode files each as a session of its own, and what a sub-agent said last is
// not what the session said last.
const opencodeSaidQuery = `SELECT p.data FROM part p JOIN message m ON m.id = p.message_id
	WHERE p.session_id = ? AND json_extract(m.data, '$.role') = 'assistant'
	AND json_extract(p.data, '$.type') = 'text'
	ORDER BY p.time_created DESC LIMIT 1`

// sessionDBSaid reads one conversation's closing words out of a provider's own
// SQLite database. Read-only and opened per call, for the reasons sessionDBCost
// gives: lich must never write into a database another tool owns.
func sessionDBSaid(path, query, id string) string {
	if id == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(%d)", path, sessionDBBusyMS)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return ""
	}
	defer func() { _ = db.Close() }()

	var data string
	if err := db.QueryRow(query, id).Scan(&data); err != nil {
		return ""
	}
	var part struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(data), &part); err != nil {
		return ""
	}
	return strings.TrimSpace(part.Text)
}

// capRunes trims the middle out of an over-long answer, keeping two thirds of
// the budget from the front and the rest from the back: the recap is the first
// paragraph, and what a report ends on — a question, a list of what is left — is
// worth keeping too. Counted in runes so a cut never lands inside a character.
func capRunes(text string, cap int) string {
	runes := []rune(text)
	if len(runes) <= cap {
		return text
	}
	head := cap * 2 / 3
	tail := cap - head
	return string(runes[:head]) + "\n\n[…]\n\n" + string(runes[len(runes)-tail:])
}
