// Spawns a real PTY like terminal_test.go, whose stubBins it uses, so it is
// Unix-only for the same reason.
//go:build !windows

package terminal

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/events"
)

// TestStartPassesUltracodeToTheProcess proves both sources of ultracode reach
// argv ahead of --mcp-config, and that a resume carries it too: Claude Code
// forgets ultracode on --resume, unlike the model and effort.
func TestStartPassesUltracodeToTheProcess(t *testing.T) {
	tests := []struct {
		name   string
		store  stubBins
		resume string
		before []string
	}{
		{name: "provider setting", store: stubBins{ultracode: true}},
		{name: "opened with ultracode", store: stubBins{sessionUltracode: true}},
		{name: "resuming", store: stubBins{sessionUltracode: true}, resume: "conv-1",
			before: []string{"--resume", "conv-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := stayAliveBin(t)
			tt.store.bin = bin
			svc := New(tt.store, nil, events.New())
			t.Cleanup(func() { _ = svc.Close("s1") })

			if err := svc.Start("s1", "p1", t.TempDir(), "claude", tt.resume, "", false, false, 80, 24); err != nil {
				t.Fatalf("Start = %v, want nil", err)
			}

			svc.mu.Lock()
			got := spawnedArgs(t, svc, "s1")
			svc.mu.Unlock()

			want := append([]string{bin}, tt.before...)
			want = append(want, "--settings", `{"ultracode":true}`, "--mcp-config")
			spawnPins(t, got, want...)
		})
	}
}

// TestStartCarriesUltracodeOntoAFork proves a fork of an ultracode conversation
// spawns with it: the fork's row is a fresh insert, and Claude Code drops
// ultracode on --resume --fork-session as on a plain resume.
func TestStartCarriesUltracodeOntoAFork(t *testing.T) {
	bin := stayAliveBin(t)
	store := stubBins{
		bin:              bin,
		ultracodeParents: map[string]bool{"conv-1": true},
		inherited:        map[string]bool{},
	}
	svc := New(store, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s2") })

	if err := svc.Start("s2", "p1", t.TempDir(), "claude", "conv-1", "", true, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s2")
	svc.mu.Unlock()

	if !slices.Contains(got, `{"ultracode":true}`) {
		t.Errorf("fork args = %v, want ultracode carried from its parent", got)
	}
	if !store.inherited["s2"] {
		t.Error("the fork's row was not given ultracode, so its next restart loses it")
	}
}

func TestStartLeavesUltracodeOffByDefault(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	svc.mu.Lock()
	got := spawnedArgs(t, svc, "s1")
	svc.mu.Unlock()

	if slices.Contains(got, "--settings") {
		t.Errorf("args = %v, want no --settings without ultracode asked for", got)
	}
}
