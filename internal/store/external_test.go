package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// externalRig is a store with one project at a real directory and a worktree of
// it elsewhere, which is the layout lich creates worktrees in.
type externalRig struct {
	svc      *Service
	project  string
	worktree string
}

func newExternalRig(t *testing.T) externalRig {
	t.Helper()
	root := t.TempDir()
	r := externalRig{
		svc:      newTestStore(t),
		project:  filepath.Join(root, "alpha"),
		worktree: filepath.Join(root, "worktrees", "alpha-feat"),
	}
	for _, dir := range []string{r.project, r.worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.svc.AddProject("p1", "alpha", r.project); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	r.svc.SetCheckoutsOf(func(path string) ([]string, error) {
		if path != r.project {
			return nil, errors.New("not a repository")
		}
		return []string{r.project, r.worktree}, nil
	})
	return r
}

func (r externalRig) list(t *testing.T) []ExternalSession {
	t.Helper()
	got, err := r.svc.ExternalSessions()
	if err != nil {
		t.Fatalf("ExternalSessions: %v", err)
	}
	return got
}

func externalIDs(sessions []ExternalSession) []string {
	out := []string{}
	for _, s := range sessions {
		out = append(out, s.ProviderSessionID)
	}
	return out
}

func TestExternalSessionsTiesAConversationToTheCheckoutItRanIn(t *testing.T) {
	r := newExternalRig(t)
	r.svc.SetConversationsOf(func() []Conversation {
		return []Conversation{
			{Kind: "claude", ID: "in-project", Title: "fix login", Cwd: r.project, UpdatedAt: 10},
			{Kind: "codex", ID: "in-worktree", Title: "port theme", Cwd: r.worktree, UpdatedAt: 20},
		}
	})
	got := r.list(t)
	want := []ExternalSession{
		{Kind: "codex", ProviderSessionID: "in-worktree", Title: "port theme", Path: r.worktree,
			ProjectID: "p1", ProjectName: "alpha", ProjectPath: r.project, UpdatedAt: 20},
		{Kind: "claude", ProviderSessionID: "in-project", Title: "fix login", Path: "",
			ProjectID: "p1", ProjectName: "alpha", ProjectPath: r.project, UpdatedAt: 10},
	}
	if len(got) != len(want) {
		t.Fatalf("ExternalSessions = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestExternalSessionsLeavesOutWhatLichCannotResume pins every reason a
// conversation on disk is not offered: lich already holds it, lich deleted it,
// it ran somewhere no project owns, or its directory is gone.
func TestExternalSessionsLeavesOutWhatLichCannotResume(t *testing.T) {
	r := newExternalRig(t)
	elsewhere := t.TempDir()
	r.svc.SetConversationsOf(func() []Conversation {
		return []Conversation{
			{Kind: "claude", ID: "held", Cwd: r.project},
			{Kind: "claude", ID: "deleted", Cwd: r.project},
			{Kind: "claude", ID: "elsewhere", Cwd: elsewhere},
			{Kind: "claude", ID: "gone", Cwd: filepath.Join(r.project, "removed")},
			{Kind: "claude", ID: "no-cwd"},
			{Kind: "claude", ID: "offered", Cwd: r.project},
		}
	})
	seedConversation(t, r.svc, "s1", "", "held")
	seedConversation(t, r.svc, "s2", "", "deleted")
	if err := r.svc.DeleteSession("p1", "s2", ""); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if got := externalIDs(r.list(t)); len(got) != 1 || got[0] != "offered" {
		t.Errorf("ExternalSessions = %v, want [offered]", got)
	}
}

// TestExternalSessionsMatchesAProjectGitCannotList pins the fallback: a project
// that is not a repository still owns its own directory.
func TestExternalSessionsMatchesAProjectGitCannotList(t *testing.T) {
	r := newExternalRig(t)
	plain := t.TempDir()
	if err := r.svc.AddProject("p2", "plain", plain); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	r.svc.SetConversationsOf(func() []Conversation {
		return []Conversation{{Kind: "kiro", ID: "k1", Cwd: plain}}
	})
	got := r.list(t)
	if len(got) != 1 || got[0].ProjectID != "p2" || got[0].Path != "" {
		t.Errorf("ExternalSessions = %+v, want k1 in p2's own directory", got)
	}
}

// TestExternalSessionsMatchesThroughASymlink pins the comparison: a provider
// records the cwd its process saw, git reports the resolved checkout.
func TestExternalSessionsMatchesThroughASymlink(t *testing.T) {
	r := newExternalRig(t)
	link := filepath.Join(t.TempDir(), "alpha-link")
	if err := os.Symlink(r.project, link); err != nil {
		t.Skipf("symlink: %v", err)
	}
	r.svc.SetConversationsOf(func() []Conversation {
		return []Conversation{{Kind: "omp", ID: "o1", Cwd: link}}
	})
	if got := externalIDs(r.list(t)); len(got) != 1 {
		t.Errorf("ExternalSessions = %v, want o1 matched through the link", got)
	}
}

func TestExternalSessionsWithoutWiringIsEmpty(t *testing.T) {
	svc := newTestStore(t)
	got, err := svc.ExternalSessions()
	if err != nil || len(got) != 0 {
		t.Errorf("ExternalSessions = %v, %v; want empty", got, err)
	}
}

// TestAdoptingAConversationParksItForTheHistoryResume pins the adoption's
// contract: the row it files is what ReopenSession brings back, carrying the
// conversation, and the conversation stops being external.
func TestAdoptingAConversationParksItForTheHistoryResume(t *testing.T) {
	r := newExternalRig(t)
	r.svc.SetConversationsOf(func() []Conversation {
		return []Conversation{{Kind: "codex", ID: "c1", Title: "port theme", Cwd: r.worktree}}
	})
	if err := r.svc.AdoptExternalSession("p1", "s1", "codex", r.worktree, "c1", "port theme"); err != nil {
		t.Fatalf("AdoptExternalSession: %v", err)
	}
	if got := r.list(t); len(got) != 0 {
		t.Errorf("ExternalSessions after adoption = %+v, want none", got)
	}
	restored, err := r.svc.ReopenSession("s1", "s2")
	if err != nil || restored == nil {
		t.Fatalf("ReopenSession = %v, %v; want the adopted row", restored, err)
	}
	if restored.Kind != "codex" || restored.ProviderSessionID != "c1" ||
		restored.Path != r.worktree || restored.Label != "port theme" {
		t.Errorf("restored = %+v, want codex c1 at the worktree", restored)
	}
}

func TestAdoptingRefusesWhatItCannotFile(t *testing.T) {
	r := newExternalRig(t)
	seedConversation(t, r.svc, "s1", "", "held")
	cases := []struct {
		name, kind, id string
		want           error
	}{
		{"held", "claude", "held", ErrConversationHeld},
		{"unknown provider", "vim", "c2", nil},
		{"no id", "claude", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.svc.AdoptExternalSession("p1", "new", tc.kind, "", tc.id, "x")
			if err == nil {
				t.Fatal("AdoptExternalSession = nil, want a refusal")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
