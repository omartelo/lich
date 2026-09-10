package terminal

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/omartelo/lich/internal/providers"
)

// todoEventName carries how far a session got through the task list its agent
// wrote for itself ({id, done, total}), emitted whenever that pair changes.
// Global like the other session events: the card that reads it is only mounted
// while its project is active, so a per-session name could not reach it.
//
// The pair is what the agent last wrote, not what is true: an agent that
// abandons a list without closing its items leaves the card reporting the
// count it was abandoned at, until it writes another one.
const todoEventName = "session-todo"

// todoEvent is the payload of todoEventName. Both counts are the whole list the
// agent last wrote, so a card can decide for itself whether a list is worth
// drawing (a finished one is not, and neither is a list of one).
type todoEvent struct {
	ID    string `json:"id"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// todoList is one task list as the sidebar reads it: how many of its items are
// finished, and how many there are. The items themselves stay in the terminal,
// where the agent wrote them.
type todoList struct {
	done  int
	total int
}

// todoReader reads one line of a provider's JSONL transcript, answering with
// the task list it writes. false for every line that writes none, which is
// almost all of them.
type todoReader func(line []byte) (todoList, bool)

// todoReaderFor pairs a conversation's transcript with the reader that finds a
// task list in it, and is the whole of what lich knows about reading one.
//
// Only Claude Code, and this is a limit of the harnesses rather than of lich.
// What each one was measured to do, so the next reader is not re-derived:
//
//   - Claude Code files `TodoWrite` as an ordinary tool call, with the list in
//     its input. Read here.
//   - Codex has `update_plan`, but its rollout never records it as a tool call
//     of its own: the plan arrives inside the JavaScript source of an `exec`
//     call (`await tools.update_plan({plan:[{step:"...",status:"..."}]})`),
//     where the object is not even JSON (measured 2026.09.09 against every
//     `update_plan` in ~/.codex/sessions). Reading it means parsing JS with a
//     regexp, which is a reader that breaks on a line break.
//   - oh-my-pi, Antigravity and Kiro write no task list at all, so there is
//     nothing to find in transcripts lich already walks.
//   - opencode and Crush keep their messages in SQLite (sessiondb.go) and
//     Cursor CLI in a blob store lich cannot order (docs/ceilings.md), so they
//     have no line to read either way.
//
// A session whose provider is absent here simply draws no progress, which is
// the same thing a session whose agent never wrote a list draws.
func todoReaderFor(src usageSource) (string, todoReader, bool) {
	if src.kind == providers.Claude {
		return src.path, claudeTodo, true
	}
	return "", nil, false
}

// claudeTodo reads a `TodoWrite` call out of a Claude transcript line.
//
// `todos` is taken both as the array it reads as and as a string holding that
// array, because Claude Code files it as a string (measured against 2.1.212 and
// 2.1.216); an array is what the tool's own schema describes, so a version that
// stops encoding it twice keeps working here.
//
// Sidechain lines are skipped for the reason claudeTurn skips them: a sub-agent
// keeps a list of its own, and its progress is not the session's.
func claudeTodo(line []byte) (todoList, bool) {
	var entry struct {
		Type        string `json:"type"`
		IsSidechain bool   `json:"isSidechain"`
		Message     struct {
			Content []struct {
				Type  string `json:"type"`
				Name  string `json:"name"`
				Input struct {
					Todos json.RawMessage `json:"todos"`
				} `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return todoList{}, false
	}
	if entry.IsSidechain || entry.Type != roleAssistant {
		return todoList{}, false
	}
	for _, block := range entry.Message.Content {
		if block.Type != "tool_use" || block.Name != "TodoWrite" {
			continue
		}
		if list, ok := countTodos(block.Input.Todos); ok {
			return list, true
		}
	}
	return todoList{}, false
}

// todoItem is the one field of a written task the count needs.
type todoItem struct {
	Status string `json:"status"`
}

// countTodos counts a written list, whether it arrives as an array or as a
// string holding one. An empty list is not a list: it is what a `TodoWrite`
// clearing the board writes, and a card drawing "0 of 0" would be reporting
// work nobody planned.
func countTodos(raw json.RawMessage) (todoList, bool) {
	var items []todoItem
	if err := json.Unmarshal(raw, &items); err != nil {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return todoList{}, false
		}
		if err := json.Unmarshal([]byte(encoded), &items); err != nil {
			return todoList{}, false
		}
	}
	if len(items) == 0 {
		return todoList{}, false
	}
	list := todoList{total: len(items)}
	for _, item := range items {
		if item.Status == todoStatusDone {
			list.done++
		}
	}
	return list, true
}

// The one status worth counting. The other two Claude Code writes (`pending`
// and `in_progress`) are both work left to do, and a status this reader has
// never seen counts as work left too.
const todoStatusDone = "completed"

// todoCursors is where each session's transcript was last read to for a task
// list (see transcriptCursors).
//
// Kept apart from the recap's cursor because the two are read on different
// clocks: this one runs off the state hook on every report, the recap only
// while a panel is open, and a shared offset would let either consume the lines
// the other has not read yet.
type todoCursors struct {
	cursors transcriptCursors[todoList]
}

// read answers with the task list this session's agent last wrote, and whether
// that pair just changed. Only a change is worth an event: a report lands on
// every tool call, and re-emitting the same two numbers would re-render the
// card through a whole turn to say nothing.
//
// A list further back than the tail a new cursor is seeded at is not looked
// for. The recap pays a walk of the whole file in that case, but this read
// happens on the first report of every session rather than on a panel somebody
// opened, and a session that never writes a list is exactly the one that would
// never find anything: transcripts here run to 168MB, and the tail is 4MB of
// conversation. What it costs is a resumed session drawing no progress until
// its agent writes the list again, which the per-run cursor gives up on anyway.
func (c *todoCursors) read(id string, src usageSource) (todoList, bool) {
	path, reader, ok := todoReaderFor(src)
	if !ok {
		return todoList{}, false
	}
	f, err := os.Open(path)
	if err != nil {
		return todoList{}, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return todoList{}, false
	}
	var list todoList
	var changed bool
	c.cursors.under(id, info, func(cur *transcriptCursor[todoList], _ bool) {
		lines, end := readLines(f, cur.offset, info.Size())
		found, ok := lastTodo(lines, reader)
		cur.info, cur.offset = info, end
		if ok {
			changed = found != cur.value
			cur.value = found
		}
		list = cur.value
	})
	return list, changed
}

// lastTodo keeps the last task list written in a run of transcript lines. false
// when they hold none, which for a continuing read means no list was written
// since the last one rather than that none ever was.
func lastTodo(lines []byte, read todoReader) (todoList, bool) {
	var last todoList
	var found bool
	for _, line := range strings.Split(string(lines), "\n") {
		if list, ok := read([]byte(line)); ok {
			last, found = list, true
		}
	}
	return last, found
}

// sessionTodo resolves the event emitTodo would push for session id, and
// whether the pair moved since the last read. Split from the emit so the
// reading is testable without a connected window, the way sessionUsage is.
func (s *Service) sessionTodo(id string, src usageSource) (todoEvent, bool) {
	list, changed := s.todos.read(id, src)
	if !changed {
		return todoEvent{}, false
	}
	return todoEvent{ID: id, Done: list.done, Total: list.total}, true
}

// emitTodo reads the task list of the conversation running in session id and
// pushes it to the frontend when it has moved. Silent on every miss (a provider
// that writes no list, an unreadable transcript, a list that has not changed),
// so the card keeps the count it has.
//
// The lock spans the read and the push, which is what keeps two reports of the
// same session from landing out of order: this event is emitted only when the
// pair moves, so an older count arriving last would sit on the card until the
// list changes again. The contended window is one Emit, and every read here is
// already serialised by the cursor behind it.
func (s *Service) emitTodo(id string, src usageSource) {
	s.todoEmit.Lock()
	defer s.todoEmit.Unlock()
	if event, ok := s.sessionTodo(id, src); ok {
		s.hub.Emit(todoEventName, event)
	}
}
