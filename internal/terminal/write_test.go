package terminal

import (
	"bytes"
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// shortWritePTY accepts at most chunk bytes per Write call, the way a Windows
// anonymous pipe can when its buffer is fuller than the write is long (see
// writeBytes). Everything past Write is unused by this test.
type shortWritePTY struct {
	chunk    int
	received bytes.Buffer
}

func (p *shortWritePTY) Write(b []byte) (int, error) {
	n := min(len(b), p.chunk)
	p.received.Write(b[:n])
	return n, nil
}

func (p *shortWritePTY) Read([]byte) (int, error) { return 0, nil }
func (p *shortWritePTY) Resize(int, int) error    { return nil }
func (p *shortWritePTY) Pid() int                 { return 0 }
func (p *shortWritePTY) Wait() error              { return nil }
func (p *shortWritePTY) Close() error             { return nil }

// TestWriteBytesSurvivesShortWrites proves a PTY that only ever accepts part
// of a write still receives every byte: writeBytes has to loop rather than
// trust the first call, which is what a large paste needs on Windows.
func TestWriteBytesSurvivesShortWrites(t *testing.T) {
	fake := &shortWritePTY{chunk: 3}
	s := New(nil, nil, events.New())
	s.sessions["s1"] = &session{pty: fake}

	payload := "\x1b[200~a message longer than the fake pipe's chunk size\x1b[201~\r"
	if err := s.Write("s1", payload); err != nil {
		t.Fatalf("Write = %v, want nil", err)
	}
	if got := fake.received.String(); got != payload {
		t.Errorf("PTY received %q, want %q", got, payload)
	}
}
