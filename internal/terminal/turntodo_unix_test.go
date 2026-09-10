// The suite reuses stubBins and writeTranscript from the Unix-tagged files, so
// it carries the same tag and the package still builds on Windows. Nothing here
// is platform-specific: the reader itself is covered in turntodo_test.go, which
// runs everywhere.
//go:build !windows

package terminal

import (
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// TestSessionTodoReportsTheWrittenList proves the whole read the card's count
// rides on: session id, recorded provider session, transcript, the event.
func TestSessionTodoReportsTheWrittenList(t *testing.T) {
	writeTranscript(t, "uuid-abc", todoLine(
		todoItemJSON("completed"), todoItemJSON("in_progress"), todoItemJSON("pending"),
	)+"\n")
	svc := New(stubBins{providerSession: "uuid-abc"}, nil, events.New())
	src, ok := svc.transcriptSource("s1")
	if !ok {
		t.Fatal("transcriptSource: want ok, got false")
	}

	got, ok := svc.sessionTodo("s1", src)
	if !ok {
		t.Fatal("sessionTodo: want ok, got false")
	}
	if want := (todoEvent{ID: "s1", Done: 1, Total: 3}); got != want {
		t.Errorf("sessionTodo = %+v, want %+v", got, want)
	}

	// The second read of an unchanged transcript is the common case, once per
	// tool call: there is nothing to push.
	if _, ok := svc.sessionTodo("s1", src); ok {
		t.Error("sessionTodo: want no event for an unchanged list")
	}
}

// TestSessionTodoIsSilentWithoutAList pins the two misses that reach here on
// every report of most sessions: an agent that wrote no list, and a provider
// whose transcript holds none lich can read.
func TestSessionTodoIsSilentWithoutAList(t *testing.T) {
	t.Run("claude session with no list", func(t *testing.T) {
		writeTranscript(t, "uuid-abc", `{"type":"assistant","message":{"content":`+
			`[{"type":"text","text":"Done."}]}}`+"\n")
		svc := New(stubBins{providerSession: "uuid-abc"}, nil, events.New())
		src, ok := svc.transcriptSource("s1")
		if !ok {
			t.Fatal("transcriptSource: want ok, got false")
		}
		if _, ok := svc.sessionTodo("s1", src); ok {
			t.Error("sessionTodo: want no event for a session with no list")
		}
	})

	t.Run("provider that writes none", func(t *testing.T) {
		writeCodexTranscript(t, "uuid-codex", `{"type":"response_item","payload":`+
			`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done."}]}}`+"\n")
		svc := New(stubBins{providerSession: "uuid-codex"}, nil, events.New())
		src, ok := svc.transcriptSource("s1")
		if !ok {
			t.Fatal("transcriptSource: want ok, got false")
		}
		if _, ok := svc.sessionTodo("s1", src); ok {
			t.Error("sessionTodo: want no event for a provider with no list to read")
		}
	})
}

// TestTranscriptSourceIsSilentWithoutAProviderSession pins the miss both
// readouts share: a session whose provider has not reported its conversation
// yet, which is every session until its first hook lands.
func TestTranscriptSourceIsSilentWithoutAProviderSession(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if _, ok := svc.transcriptSource("s1"); ok {
		t.Error("transcriptSource: want false without a provider session")
	}
}
