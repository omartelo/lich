package terminal

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/store"
)

// conversationHeadBytes bounds how much of one transcript is read to describe
// it. Everything a listing needs (id, cwd, title, the first prompt) is written
// at the top: measured over 763 Claude Code transcripts, 88% carry their title
// record in the first 64 KB, and reading that much of each took 71 ms in all.
const conversationHeadBytes = 64 << 10

// conversationTitleRunes caps a title taken from a prompt, which can run to
// pages; the palette row shows one line of it.
const conversationTitleRunes = 120

// conversationReaders lists the conversations of each provider. Every reader
// fails toward listing less: a store that is missing, moved or unreadable, and
// a record that does not parse, drop out without an error, because a
// conversation lich cannot describe is not one it can offer back.
var conversationReaders = map[string]func() []store.Conversation{
	providers.Claude:      claudeConversations,
	providers.Codex:       codexConversations,
	providers.OMP:         ompConversations,
	providers.Kiro:        kiroConversations,
	providers.Cursor:      cursorConversations,
	providers.Antigravity: antigravityConversations,
	providers.OpenCode:    opencodeConversations,
	providers.Crush:       crushConversations,
}

// Conversations lists every conversation the providers keep on disk, whoever
// started it (store.SetConversationsOf). Headless runs and sub-agents are left
// out: neither is a conversation somebody resumes by hand.
func Conversations() []store.Conversation {
	var all []store.Conversation
	for _, read := range conversationReaders {
		all = append(all, read()...)
	}
	return all
}

// claudeConversations reads ~/.claude/projects/<slug>/<id>.jsonl. Sub-agents
// live a directory deeper and are not matched.
func claudeConversations() []store.Conversation {
	base, ok := harnessDir("CLAUDE_CONFIG_DIR", ".claude")
	if !ok {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(base, "projects", "*", "*.jsonl"))
	var out []store.Conversation
	for _, path := range paths {
		if c, ok := claudeConversation(path); ok {
			out = append(out, c)
		}
	}
	return out
}

// claudeRecord is the union of the fields a Claude Code transcript's records
// carry that describe the conversation.
type claudeRecord struct {
	Type        string `json:"type"`
	Cwd         string `json:"cwd"`
	Entrypoint  string `json:"entrypoint"`
	CustomTitle string `json:"customTitle"`
	AITitle     string `json:"aiTitle"`
	IsMeta      bool   `json:"isMeta"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// claudeHeadlessEntrypoint marks a `claude -p` run.
const claudeHeadlessEntrypoint = "sdk-cli"

func claudeConversation(path string) (store.Conversation, bool) {
	c := store.Conversation{Kind: providers.Claude, ID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}
	var birthName, titled, prompt string
	for _, line := range headLines(path) {
		var r claudeRecord
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		if r.Entrypoint == claudeHeadlessEntrypoint {
			return store.Conversation{}, false
		}
		if c.Cwd == "" {
			c.Cwd = r.Cwd
		}
		birthName = firstNonEmpty(birthName, r.CustomTitle)
		titled = firstNonEmpty(titled, r.CustomTitle, r.AITitle)
		if prompt == "" && r.Type == "user" && !r.IsMeta {
			prompt = promptTitle(stringContent(r.Message.Content))
		}
	}
	// The name lich spawns Claude Code with is the first title on record; a
	// conversation carrying one was born in lich, and lich has let go of it.
	if c.Cwd == "" || relay.IsRosterName(birthName, c.Cwd) {
		return store.Conversation{}, false
	}
	c.Title = firstNonEmpty(titled, prompt)
	c.UpdatedAt = modTime(path)
	return c, true
}

// codexConversations reads ~/.codex/sessions/<y>/<m>/<d>/rollout-*.jsonl. The
// first record is the session's own metadata; the title is the name the user
// gave it in session_index.jsonl, else its first prompt.
func codexConversations() []store.Conversation {
	base, ok := harnessDir("CODEX_HOME", ".codex")
	if !ok {
		return nil
	}
	names := codexThreadNames(filepath.Join(base, "session_index.jsonl"))
	paths, _ := filepath.Glob(filepath.Join(base, "sessions", "*", "*", "*", "rollout-*.jsonl"))
	var out []store.Conversation
	for _, path := range paths {
		if c, ok := codexConversation(path); ok {
			c.Title = firstNonEmpty(names[c.ID], c.Title)
			out = append(out, c)
		}
	}
	return out
}

type codexRecord struct {
	Type    string `json:"type"`
	Payload struct {
		Type         string          `json:"type"`
		ID           string          `json:"id"`
		Cwd          string          `json:"cwd"`
		Source       json.RawMessage `json:"source"`
		ThreadSource string          `json:"thread_source"`
		Message      string          `json:"message"`
	} `json:"payload"`
}

// codexInteractiveSource is the session_meta source of the TUI; `codex exec`
// writes "exec", and a sub-agent thread carries a source object instead.
const codexInteractiveSource = `"cli"`

func codexConversation(path string) (store.Conversation, bool) {
	lines := headLines(path)
	if len(lines) == 0 {
		return store.Conversation{}, false
	}
	var meta codexRecord
	if json.Unmarshal(lines[0], &meta) != nil || meta.Type != "session_meta" {
		return store.Conversation{}, false
	}
	interactive := string(meta.Payload.Source) == codexInteractiveSource
	spawned := meta.Payload.ThreadSource != "" && meta.Payload.ThreadSource != "user"
	if !interactive || spawned || meta.Payload.ID == "" {
		return store.Conversation{}, false
	}
	c := store.Conversation{Kind: providers.Codex, ID: meta.Payload.ID, Cwd: meta.Payload.Cwd, UpdatedAt: modTime(path)}
	for _, line := range lines[1:] {
		var r codexRecord
		if json.Unmarshal(line, &r) == nil && r.Payload.Type == "user_message" {
			c.Title = promptTitle(r.Payload.Message)
			break
		}
	}
	return c, c.Cwd != ""
}

func codexThreadNames(path string) map[string]string {
	names := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return names
	}
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		var entry struct {
			ID   string `json:"id"`
			Name string `json:"thread_name"`
		}
		if json.Unmarshal(line, &entry) == nil && entry.ID != "" {
			names[entry.ID] = entry.Name
		}
	}
	return names
}

// ompConversations reads omp's sessions/<encoded-cwd>/<timestamp>_<id>.jsonl:
// a title record, then the session header with the id and cwd. Sub-agents live
// a directory deeper and are not matched.
func ompConversations() []store.Conversation {
	base, ok := ompAgentDir()
	if !ok {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(base, "sessions", "*", "*_*.jsonl"))
	var out []store.Conversation
	for _, path := range paths {
		if c, ok := ompConversation(path); ok {
			out = append(out, c)
		}
	}
	return out
}

func ompConversation(path string) (store.Conversation, bool) {
	c := store.Conversation{Kind: providers.OMP, UpdatedAt: modTime(path)}
	for _, line := range headLines(path) {
		var r struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			ID    string `json:"id"`
			Cwd   string `json:"cwd"`
		}
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		switch r.Type {
		case "title":
			c.Title = r.Title
		case "session":
			c.ID, c.Cwd = r.ID, r.Cwd
		}
		if c.ID != "" && c.Title != "" {
			break
		}
	}
	return c, c.ID != "" && c.Cwd != ""
}

// headLines is the complete lines in the first conversationHeadBytes of path. A
// line cut by the bound is dropped rather than half-parsed.
func headLines(path string) [][]byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	head, err := io.ReadAll(io.LimitReader(f, conversationHeadBytes))
	if err != nil {
		return nil
	}
	if len(head) == conversationHeadBytes {
		if cut := bytes.LastIndexByte(head, '\n'); cut >= 0 {
			head = head[:cut]
		}
	}
	return bytes.Split(bytes.TrimSpace(head), []byte("\n"))
}

// stringContent is a message body written as a plain string. A body of blocks
// is a tool result or an attachment, never a prompt somebody typed.
func stringContent(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return ""
	}
	return text
}

// promptTitle turns a prompt into a one-line title. A prompt that opens with a
// tag is one the harness wrote (a slash command, a caveat), not the user.
func promptTitle(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || strings.HasPrefix(prompt, "<") {
		return ""
	}
	line, _, _ := strings.Cut(prompt, "\n")
	if utf8.RuneCountInString(line) <= conversationTitleRunes {
		return line
	}
	return string([]rune(line)[:conversationTitleRunes]) + "…"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func modTime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}
