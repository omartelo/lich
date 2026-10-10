package relay

import (
	"strings"
	"testing"
)

func TestFocusOpensTheCardAndRaisesTheWindow(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s1", "s2", "s3"), events)
	var order []string
	svc.SetRaiseWindow(func() {
		if len(events.focus) == 1 {
			order = append(order, "card", "window")
		}
	})

	got, err := svc.Focus(FocusOptions{Target: "feature"})
	if err != nil {
		t.Fatalf("Focus = %v, want nil", err)
	}
	if got != (Focused{ID: "s2", Label: "feature", Project: "lich"}) {
		t.Errorf("Focus = %+v, want the session it named", got)
	}
	if len(events.focus) != 1 || events.focus[0].ID != "s2" {
		t.Errorf("focus events = %+v, want one for s2", events.focus)
	}
	if strings.Join(order, ",") != "card,window" {
		t.Errorf("order = %v, want the card chosen before the window comes up", order)
	}
}

func TestFocusNamesTheLiveSessionsWhenTheTargetIsUnknown(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s1", "s3"), events)
	raised := false
	svc.SetRaiseWindow(func() { raised = true })

	_, err := svc.Focus(FocusOptions{Target: "feature"})
	if err == nil || !strings.Contains(err.Error(), "main") || !strings.Contains(err.Error(), "solo") {
		t.Fatalf("Focus = %v, want an error listing the live sessions", err)
	}
	if raised || len(events.focus) != 0 {
		t.Errorf("raised = %v, focus events = %+v, want nothing touched", raised, events.focus)
	}
}

func TestFocusWorksWithoutAWindow(t *testing.T) {
	svc := newRelay(editorWorkspace("/src/lich", "/src/lich-wt"), newFakeTerminal("s3"), nil)

	if got, err := svc.Focus(FocusOptions{Target: "solo", Project: "revu"}); err != nil || got.ID != "s3" {
		t.Errorf("Focus = %+v, %v, want s3", got, err)
	}
}
