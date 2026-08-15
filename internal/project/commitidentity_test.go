package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// isolateGitConfig points git's global and system config at files this test
// owns, so what the machine running the suite has configured cannot decide the
// answer — the whole point here is which scope a value came from.
func isolateGitConfig(t *testing.T, global string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, []byte(global), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// TestCommitIdentity pins the three answers the settings row renders: the
// global identity, a repository-local override of it, and no identity at all —
// which is a state to show, not an error.
func TestCommitIdentity(t *testing.T) {
	const global = "[user]\n\tname = Jane Doe\n\temail = jane@example.com\n"
	tests := []struct {
		name       string
		globalConf string
		local      map[string]string
		want       CommitIdentity
	}{
		{
			"the global identity, with nothing local to override it",
			global,
			nil,
			CommitIdentity{Name: "Jane Doe", Email: "jane@example.com"},
		},
		{
			"a repository-local identity wins and says so",
			global,
			map[string]string{"user.name": "Work Jane", "user.email": "jane@work.example"},
			CommitIdentity{Name: "Work Jane", Email: "jane@work.example", Local: true},
		},
		{
			// Only the email decides Local: it is the value git refuses a
			// commit without, so a local name over a global email is still the
			// global identity being used.
			"a local name over a global email is not an override",
			global,
			map[string]string{"user.name": "Work Jane"},
			CommitIdentity{Name: "Work Jane", Email: "jane@example.com"},
		},
		{
			"no identity anywhere is the empty answer",
			"",
			nil,
			CommitIdentity{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateGitConfig(t, tt.globalConf)
			repo := initBareRepo(t)
			for key, value := range tt.local {
				if out, err := exec.Command("git", "-C", repo, "config", key, value).CombinedOutput(); err != nil {
					t.Fatalf("git config %s: %v (%s)", key, err, out)
				}
			}
			if got := New(nil).CommitIdentity(repo); got != tt.want {
				t.Errorf("CommitIdentity() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCommitIdentityOutsideARepositoryStaysSilent covers the path a settings
// page can always reach — a project directory that is not a checkout. git has
// no local scope to read there, so the answer must degrade to the global
// identity without filing a warning for the scope that does not exist.
func TestCommitIdentityOutsideARepositoryStaysSilent(t *testing.T) {
	isolateGitConfig(t, "[user]\n\tname = Jane Doe\n\temail = jane@example.com\n")
	logged := captureLog(t)

	want := CommitIdentity{Name: "Jane Doe", Email: "jane@example.com"}
	if got := New(nil).CommitIdentity(t.TempDir()); got != want {
		t.Errorf("CommitIdentity(non-repo) = %+v, want %+v", got, want)
	}
	if out := logged(); out != "" {
		t.Errorf("reading the identity logged:\n%s", out)
	}
}
