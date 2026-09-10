package terminal

import (
	"os"
	"sync"
)

// transcriptCursors is where each session's provider transcript was last read
// to, for one kind of answer. A transcript is append-only, so remembering the
// offset a read ended at turns the next one into a walk of what has been
// written since, where a bounded tail re-read every time loses whatever falls
// behind the bound.
//
// It holds the bookkeeping the readers share and none of their policy: what
// counts as an answer, and what to do when a walk finds none, belongs to the
// reader (said.go keeps the last thing the agent said, turntodo.go the last
// task list it wrote).
//
// In memory, per run of lich: what a cursor saves is the reading, never the
// answer, so a session restored at launch simply seeds itself again.
//
// The zero value is ready to use.
type transcriptCursors[T any] struct {
	mu sync.Mutex
	at map[string]*transcriptCursor[T]
}

// transcriptCursor is one session's place in one transcript: the file's
// identity, how far it has been read, and the last answer found there. The
// identity is what tells an ordinary append apart from a file this cursor no
// longer belongs to (a forked conversation, or a rewritten one), where the
// offset would otherwise count into bytes nobody read.
type transcriptCursor[T any] struct {
	info   os.FileInfo
	offset int64
	value  T
}

// under runs fn with this session's cursor, holding the lock for as long as it
// takes: a reader mutates the cursor it was handed, and the only contenders are
// two reads of the same session, which would be reading the same bytes anyway.
//
// fresh says the cursor was made here: nothing read before this, or a file the
// previous read's offset does not belong to. A new cursor is seeded at the
// file's tail, since a conversation running for hours is tens of MB and a first
// read is paying for something with a person or an agent waiting on it.
func (c *transcriptCursors[T]) under(id string, info os.FileInfo, fn func(cur *transcriptCursor[T], fresh bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur, ok := c.at[id]; ok && os.SameFile(cur.info, info) && info.Size() >= cur.offset {
		fn(cur, false)
		return
	}
	if c.at == nil {
		c.at = make(map[string]*transcriptCursor[T])
	}
	cur := &transcriptCursor[T]{offset: max(0, info.Size()-searchTailBytes)}
	c.at[id] = cur
	fn(cur, true)
}

// forget drops a closed session's cursor. Nothing will ask about its transcript
// again, and a card is closed far more often than lich is.
func (c *transcriptCursors[T]) forget(id string) {
	c.mu.Lock()
	delete(c.at, id)
	c.mu.Unlock()
}
