package store

import "testing"

func seedConversation(t *testing.T, svc *Service, sessionID, path, conversation string) {
	t.Helper()
	if err := svc.AddSession("p1", sessionID, sessionID, "claude", path, 2, ""); err != nil {
		t.Fatalf("AddSession %q: %v", sessionID, err)
	}
	if err := svc.SetProviderSession(sessionID, conversation); err != nil {
		t.Fatalf("SetProviderSession %q: %v", sessionID, err)
	}
}

func forgotten(t *testing.T, svc *Service) map[string]bool {
	t.Helper()
	got, err := svc.ForgottenProviderSessions()
	if err != nil {
		t.Fatalf("ForgottenProviderSessions: %v", err)
	}
	return got
}

func newTombstoneStore(t *testing.T) *Service {
	t.Helper()
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	return svc
}

// TestDeletingASessionRemembersItsConversation pins every way a row leaves for
// good: each must leave its conversation id behind, or a listing of the
// conversations on disk offers the user back what they threw away.
func TestDeletingASessionRemembersItsConversation(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		delete func(*Service) error
	}{
		{"DeleteSession", "", func(s *Service) error { return s.DeleteSession("p1", "s1", "") }},
		{"ForgetSession", "", func(s *Service) error {
			if err := s.CloseSession("p1", "s1", ""); err != nil {
				return err
			}
			return s.ForgetSession("s1")
		}},
		{"PurgeWorktreeSessions", "/wt/a", func(s *Service) error { return s.PurgeWorktreeSessions("p1", "/wt/a") }},
		{"DeleteProject", "", func(s *Service) error { return s.DeleteProject("p1") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTombstoneStore(t)
			seedConversation(t, svc, "s1", tc.path, "conv-1")
			if err := tc.delete(svc); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if !forgotten(t, svc)["conv-1"] {
				t.Errorf("conv-1 not remembered after %s", tc.name)
			}
		})
	}
}

// TestResumingKeepsTheConversationLive pins the resume: it deletes the parked
// row and inserts one under a new id with the same conversation, and that
// conversation is lich's, not forgotten.
func TestResumingKeepsTheConversationLive(t *testing.T) {
	svc := newTombstoneStore(t)
	seedConversation(t, svc, "s1", "", "conv-1")
	if err := svc.CloseSession("p1", "s1", ""); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	restored, err := svc.ReopenSession("s1", "s2")
	if err != nil || restored == nil {
		t.Fatalf("ReopenSession = %v, %v; want the parked row", restored, err)
	}
	if forgotten(t, svc)["conv-1"] {
		t.Error("conv-1 forgotten although the resumed session holds it")
	}
}

// TestASessionTakingBackAConversationClearsItsStone pins the way back: a
// conversation forgotten once and then held again, by a new row reporting it,
// is lich's again.
func TestASessionTakingBackAConversationClearsItsStone(t *testing.T) {
	svc := newTombstoneStore(t)
	seedConversation(t, svc, "s1", "", "conv-1")
	if err := svc.DeleteSession("p1", "s1", ""); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	seedConversation(t, svc, "s2", "", "conv-1")
	if forgotten(t, svc)["conv-1"] {
		t.Error("conv-1 still forgotten after a session reported it again")
	}
}

// TestDeletingASessionWithNoConversationLeavesNoStone pins that a shell, or an
// agent that never reported an id, adds nothing: an empty id would match every
// such row and say nothing about any conversation.
func TestDeletingASessionWithNoConversationLeavesNoStone(t *testing.T) {
	svc := newTombstoneStore(t)
	if err := svc.AddSession("p1", "s1", "Shell", "shell", "", 2, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.DeleteSession("p1", "s1", ""); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if got := forgotten(t, svc); len(got) != 0 {
		t.Errorf("forgotten = %v, want none", got)
	}
}
