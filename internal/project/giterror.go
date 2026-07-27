package project

import "errors"

// gitFailure turns one failed git call into the message the screen shows. See
// toolerror.go for why git's own stderr never travels with it.
func gitFailure(args []string, stderr string, runErr error) error {
	logToolFailure("git", args, stderr, runErr)
	return errors.New(gitMessage(stderr, runErr))
}

// gitFailures maps what git reports onto what the person reading the screen can
// do about it. Only the failures lich's own flows can produce are here — the
// worktree lifecycle, a discard, a read of a folder that turned out not to be a
// repository.
var gitFailures = []toolFailure{
	{"not a git repository", "This folder is not a git repository."},
	{"already exists", "A branch or worktree with that name already exists."},
	{"is already checked out", "That branch is already checked out in another worktree."},
	{
		"contains modified or untracked files",
		"That worktree has uncommitted changes. Remove it from the sidebar to discard them.",
	},
	{"is locked", "git has that worktree locked."},
	{"is not a working tree", "git no longer knows that worktree — it may already be gone."},
	{"couldn't find remote ref", "That branch is no longer on the remote."},
	{"could not read from remote repository", "The remote refused the connection — check its ssh key or credentials."},
	{"authentication failed", "The remote refused the connection — check its ssh key or credentials."},
	{"permission denied (publickey)", "The remote refused the connection — check its ssh key or credentials."},
	{"does not appear to be a git repository", "That remote could not be reached."},
	{"did not match any file", "git does not track that file."},
	{"unknown revision", "git does not know that branch or commit."},
	{"permission denied", "The filesystem refused the change."},
}

// gitMessage picks the message for one git failure. git also fails with nothing
// on stderr at all — `check-ref-format` rejects a branch name by exit status
// alone — which is why the fallback never quotes the process error either.
func gitMessage(stderr string, runErr error) string {
	if missingTool(stderr, runErr) {
		return "git is not installed."
	}
	if message, ok := matchFailure(gitFailures, stderr); ok {
		return message
	}
	return "git could not complete the operation. lich's log has what it reported."
}
