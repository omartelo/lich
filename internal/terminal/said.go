package terminal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

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
// words while the diff reads "unavailable", which is why the band labels them
// as such while the card is busy (saidNote, frontend/src/lib/git/last-turn.ts).
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
	return LastSaid{Text: capRunes(s.recap.said(id, src), saidRuneCap)}, nil
}

// transcriptReaderFor pairs a conversation's JSONL transcript with the reader
// that parses its lines. It is the whole of what lich knows about reading a
// provider's conversation, and both readers of one go through it: the last-turn
// recap below, and the palette's transcript search (search.go).
//
// false for the three providers with no JSONL to walk. opencode and Crush keep
// their messages in SQLite, read by query instead (sessiondb.go); Cursor CLI
// files a chat as a content-addressed blob store whose order lives in a protobuf
// index, so neither mechanism reaches it at all (docs/ceilings.md).
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

// saidCursors is where each session's transcript was last read to. A transcript
// is append-only, so remembering the offset one read ended at turns the next
// into a walk of what has been written since, and a turn is then read whole
// however large it is, where a bounded tail re-read every poll (searchTailBytes,
// still what the palette's search does) loses the turn whose closing words fall
// behind more tool output than the bound holds.
//
// In memory, per run of lich: what the cursor saves is the reading, never the
// answer, so a session restored at launch simply seeds itself again.
type saidCursors struct {
	mu sync.Mutex
	at map[string]*saidCursor
}

// saidCursor is one session's place in one transcript: the file's identity, how
// far it has been read, and the last prose found there. The identity is what
// tells an ordinary append apart from a file this cursor no longer belongs to
// (a forked conversation, or a rewritten one), where the offset would otherwise
// count into bytes nobody read.
type saidCursor struct {
	info   os.FileInfo
	offset int64
	text   string
}

// said reads one conversation's closing words out of whatever the provider files
// them in: a walk of the transcript for the providers that write JSONL, and a
// query for the two that keep their messages in a database of their own
// (sessiondb.go), which have no tail to bound and so no cursor to keep. Empty
// for every miss, and for Cursor CLI, which reaches here with nothing either can
// read: a chat filed as a content-addressed blob store (docs/ceilings.md).
func (c *saidCursors) said(id string, src usageSource) string {
	if path, read, ok := transcriptReaderFor(src); ok {
		return c.walk(id, path, read)
	}
	if texts := sessionDBTexts(src.path, queriesFor(src.kind).said, src.id); len(texts) > 0 {
		return strings.TrimSpace(texts[0])
	}
	return ""
}

// walk reads whatever the transcript has grown since this session was last
// asked, and keeps the newest thing the agent said in it. The answer already on
// record stands when those new bytes hold no prose, which is what a poll
// landing in the middle of a turn's tool output reads, and what used to be an
// empty band indistinguishable from a turn that ended on a tool call.
func (c *saidCursors) walk(id, path string, read turnReader) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	// The lock spans the read: its only contenders are two polls of the same
	// panel, which would be reading the same bytes anyway.
	c.mu.Lock()
	defer c.mu.Unlock()
	cur, fresh := c.cursor(id, info)
	lines, end := readLines(f, cur.offset, info.Size())
	text, found := lastSaid(lines, read)
	// A tail holding no prose at all is the one case worth a full walk: it takes
	// a single tool result larger than the bound, and the words behind it are
	// exactly the turn the band exists to catch up on. Once per session: the
	// cursor lands at the end of the file either way.
	if !found && fresh && cur.offset > 0 {
		lines, end = readLines(f, 0, info.Size())
		text, found = lastSaid(lines, read)
	}
	cur.info, cur.offset = info, end
	if found {
		cur.text = text
	}
	return cur.text
}

// cursor answers with this session's place in info, seeding a new one at the
// file's tail: a conversation running for hours is tens of MB, and a first read
// is paying for a panel somebody just opened. fresh says the cursor was made
// here: nothing read before this, or a file the previous read's offset does
// not
// belong to (see saidCursor).
func (c *saidCursors) cursor(id string, info os.FileInfo) (*saidCursor, bool) {
	if cur, ok := c.at[id]; ok && os.SameFile(cur.info, info) && info.Size() >= cur.offset {
		return cur, false
	}
	if c.at == nil {
		c.at = make(map[string]*saidCursor)
	}
	cur := &saidCursor{offset: max(0, info.Size()-searchTailBytes)}
	c.at[id] = cur
	return cur, true
}

// forget drops a closed session's cursor. Nothing will ask about its transcript
// again, and a card is closed far more often than lich is.
func (c *saidCursors) forget(id string) {
	c.mu.Lock()
	delete(c.at, id)
	c.mu.Unlock()
}

// readLines returns the complete lines between start and the end of an open
// transcript, and the offset the last of them ended at. A transcript caught
// mid-write leaves its half line behind rather than consuming it: the next read
// starts at that line and reads it whole.
func readLines(f *os.File, start, size int64) ([]byte, int64) {
	if size <= start {
		return nil, start
	}
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil, start
	}
	cut := bytes.LastIndexByte(buf, '\n')
	if cut < 0 {
		return nil, start
	}
	return buf[:cut+1], start + int64(cut) + 1
}

// lastSaid keeps the last thing the agent said in a run of transcript lines.
// false when they hold none, which for a continuing read means nothing new was
// said rather than that nothing was.
func lastSaid(lines []byte, read turnReader) (string, bool) {
	var last string
	var found bool
	for _, line := range strings.Split(string(lines), "\n") {
		// A line the reader rejects is a tool call, a meta line, or one a seed
		// tail cut in half, and nothing here distinguishes them.
		if t, ok := read([]byte(line)); ok && t.role == roleAssistant {
			last, found = t.text, true
		}
	}
	return strings.TrimSpace(last), found
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
