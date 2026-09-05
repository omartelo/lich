package terminal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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

// saidFor reads one conversation's closing words out of whatever the provider
// files them in. Empty for every miss, and for the two providers that reach here
// with nothing to read — Cursor CLI files its chat as a content-addressed blob
// store, and neither it nor Crush reports a turn boundary, so neither is ever
// offered the panel this feeds (docs/ceilings.md).
func saidFor(src usageSource) string {
	switch src.kind {
	case providers.Claude:
		return lastSaidInTail(src.path, claudeSaid)
	case providers.Codex:
		return lastSaidInTail(src.path, codexSaid)
	case providers.OMP:
		return lastSaidInTail(src.path, ompSaid)
	case providers.Kiro:
		return lastSaidInTail(kiroTranscriptPath(src.path), kiroSaid)
	case providers.Antigravity:
		return lastSaidInTail(src.path, antigravitySaid)
	case providers.OpenCode:
		return sessionDBSaid(src.path, opencodeSaidQuery, src.id)
	}
	return ""
}

// lastSaidInTail walks a JSONL transcript's tail and keeps the last line say
// reads a message out of. The tail is bounded for the reason the transcript
// search bounds its own (searchTailBytes): a long conversation runs to tens of
// megabytes of tool output, and this is re-read while the panel is open.
//
// Ceiling: a turn whose closing words fall outside that tail reads as no words
// at all. It takes a single tool result larger than the bound to do it, and the
// honest answer then is the empty one — the alternative is showing an older
// turn's words under this turn's heading.
func lastSaidInTail(path string, say func([]byte) (string, bool)) string {
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
		if text, ok := say([]byte(line)); ok {
			last = text
		}
	}
	return strings.TrimSpace(last)
}

// claudeSaid reads a Claude transcript line. Sidechain lines — a sub-agent's own
// conversation — are skipped for the reason the transcript search skips them:
// they are not the conversation the user had, so their last word is not the
// session's.
func claudeSaid(line []byte) (string, bool) {
	var entry struct {
		Type        string `json:"type"`
		IsSidechain bool   `json:"isSidechain"`
		Message     struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return "", false
	}
	if entry.Type != "assistant" || entry.IsSidechain {
		return "", false
	}
	return joinText(entry.Message.Content)
}

// codexSaid reads a Codex rollout line: the assistant's own message items, whose
// blocks are `output_text` rather than `text` (measured against a real rollout —
// every assistant block in it was that one type).
func codexSaid(line []byte) (string, bool) {
	var entry struct {
		Type    string `json:"type"`
		Payload struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return "", false
	}
	if entry.Type != "response_item" || entry.Payload.Type != "message" ||
		entry.Payload.Role != "assistant" {
		return "", false
	}
	var parts []string
	for _, block := range entry.Payload.Content {
		if block.Type == "output_text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n"), true
}

// ompSaid reads an oh-my-pi transcript line. Its assistant turns are mostly
// thinking plus a tool call — only the turn that ends the run carries a `text`
// block, which is exactly the one wanted here.
func ompSaid(line []byte) (string, bool) {
	var entry struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return "", false
	}
	if entry.Type != "message" || entry.Message.Role != "assistant" {
		return "", false
	}
	return joinText(entry.Message.Content)
}

// kiroSaid reads a Kiro CLI transcript line. Its envelope names the entry kind
// in PascalCase and the blocks inside it in lower case, and a block's payload
// sits under `data` rather than under a name of its own.
func kiroSaid(line []byte) (string, bool) {
	var entry struct {
		Kind string `json:"kind"`
		Data struct {
			Content []struct {
				Kind string `json:"kind"`
				Data string `json:"data"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return "", false
	}
	if entry.Kind != "AssistantMessage" {
		return "", false
	}
	var parts []string
	for _, block := range entry.Data.Content {
		if block.Kind == "text" && block.Data != "" {
			parts = append(parts, block.Data)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n"), true
}

// antigravitySaid reads an Antigravity transcript line. `PLANNER_RESPONSE` is
// the model's own prose and its content is a bare string; the `GENERIC` entries
// that outnumber it carry tool output — spinner frames and command results —
// which is what a reader matching on `source: MODEL` alone would surface
// instead (measured against a real 1.1.19 transcript).
func antigravitySaid(line []byte) (string, bool) {
	var entry struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return "", false
	}
	if entry.Type != "PLANNER_RESPONSE" || entry.Content == "" {
		return "", false
	}
	return entry.Content, true
}

// joinText joins the text blocks of a message whose blocks name their type
// `type` and their text `text` — the shape Claude Code and oh-my-pi share.
// false when the message is all thinking and tool calls, which is every
// assistant turn but the last of a run.
func joinText(blocks []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) (string, bool) {
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
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
