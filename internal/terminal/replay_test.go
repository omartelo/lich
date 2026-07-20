package terminal

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestReplayBufferKeepsOutputInOrder(t *testing.T) {
	b := newReplayBuffer(1024)
	b.append([]byte("hello "))
	b.append([]byte("world"))
	if got := string(b.snapshot()); got != "hello world" {
		t.Fatalf("snapshot = %q, want %q", got, "hello world")
	}
}

func TestReplayBufferIgnoresEmptyAppend(t *testing.T) {
	b := newReplayBuffer(1024)
	b.append(nil)
	b.append([]byte{})
	if got := b.snapshot(); len(got) != 0 {
		t.Fatalf("snapshot = %q, want empty", got)
	}
}

func TestReplayBufferCopiesInput(t *testing.T) {
	b := newReplayBuffer(1024)
	buf := []byte("abc")
	b.append(buf)
	copy(buf, "XYZ") // the caller reuses its read buffer for the next PTY read
	if got := string(b.snapshot()); got != "abc" {
		t.Fatalf("snapshot = %q, want %q — append must copy, not alias", got, "abc")
	}
}

func TestReplayBufferDropsOldestOnOverflow(t *testing.T) {
	b := newReplayBuffer(10)
	b.append([]byte("aaaaa")) // 5
	b.append([]byte("bbbbb")) // 10, still within cap
	b.append([]byte("ccccc")) // 15 → drops the first chunk
	got := string(b.snapshot())
	if strings.Contains(got, "a") {
		t.Fatalf("snapshot = %q, still holds the dropped chunk", got)
	}
	if got != "bbbbbccccc" {
		t.Fatalf("snapshot = %q, want %q", got, "bbbbbccccc")
	}
}

func TestReplayBufferKeepsOneChunkLargerThanCap(t *testing.T) {
	b := newReplayBuffer(4)
	big := bytes.Repeat([]byte("x"), 100)
	b.append(big)
	if got := b.snapshot(); len(got) != 100 {
		t.Fatalf("snapshot len = %d, want 100 — a lone oversized chunk must survive", len(got))
	}
}

// A bare Service with a hand-built session exercises Replay without a PTY, so
// this stays OS-agnostic (no !windows build tag): Replay only reads the tail.
func TestServiceReplayEncodesTheTail(t *testing.T) {
	b := newReplayBuffer(replayCapBytes)
	b.append([]byte("scrollback"))
	svc := &Service{sessions: map[string]*session{"s1": {replay: b}}}

	encoded, err := svc.Replay("s1")
	if err != nil {
		t.Fatalf("Replay = %v, want nil", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != "scrollback" {
		t.Fatalf("Replay tail = %q, want %q", decoded, "scrollback")
	}
}

func TestServiceReplayUnknownSessionIsEmpty(t *testing.T) {
	svc := &Service{sessions: map[string]*session{}}
	got, err := svc.Replay("nope")
	if err != nil {
		t.Fatalf("Replay = %v, want nil", err)
	}
	if got != "" {
		t.Fatalf("Replay unknown = %q, want empty", got)
	}
}
