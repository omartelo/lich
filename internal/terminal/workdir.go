package terminal

import (
	"errors"
	"io/fs"
	"os"
)

// WorkdirMissing reports whether a session cannot spawn in cwd because the
// directory is not there — what a worktree removed outside lich leaves behind,
// where the checkout is gone but the session row pointing at it is not. The PTY
// would fail to chdir, and startPTY reports that the same way it reports a
// missing binary or an exhausted fd table, so the caller cannot tell an
// unrecoverable session from one a setting would fix. This separates the one
// case that is not worth retrying.
//
// Phrased as the missing case because false is the safe answer, and every
// uncertainty takes it: an empty cwd (Start resolves it to the home directory)
// and any stat failure that is not a plain "not there" — a permission error, a
// mount answering badly — fall through to the spawn, which reports its own
// failure. True is only a directory provably absent, or a path that exists and
// is not a directory.
func (*Service) WorkdirMissing(cwd string) bool {
	if cwd == "" {
		return false
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return errors.Is(err, fs.ErrNotExist)
	}
	return !info.IsDir()
}
