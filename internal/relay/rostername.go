package relay

import (
	"path/filepath"
	"strings"
)

// A lich session has two names, and until this file they were two address
// spaces. The card carries a label the user chose ("docs"); Claude Code's peer
// roster carries the name lich passes at spawn ("skipo-3939"). Both name the
// same session, and nothing in an agent's world said so — so an agent handed
// one of them reached for whichever tool it had and, on the first real run,
// used both channels at once.
//
// The relay therefore answers to either. It can: lich is what mints the roster
// name, so it can derive it again here. This is the Go half of
// frontend/src/lib/session/peer-name.ts, and the two have to agree — a name
// derived differently would address a session that is not the one on the card.

// rosterIDChars is how much of the session id trails the directory name. Four
// separates the handful of sessions one checkout holds and stays readable in a
// roster row.
const rosterIDChars = 4

// rosterNameOf builds the name a session answers to in the peer roster: the
// last element of its working directory, then four characters of its id. cwd is
// the session's own directory when it has one (a worktree) and its project's
// otherwise, matching what the frontend passes at spawn.
func rosterNameOf(cwd, id string) string {
	dir := filepath.Base(strings.TrimRight(strings.ReplaceAll(cwd, "\\", "/"), "/"))
	// filepath.Base answers "." for an empty path and "/" for a bare root;
	// neither names anything, and Claude Code's own fallback is the app name.
	if dir == "" || dir == "." || dir == "/" {
		dir = "lich"
	}

	tail := make([]rune, 0, rosterIDChars)
	for _, r := range id {
		if len(tail) == rosterIDChars {
			break
		}
		if isRosterChar(r) {
			tail = append(tail, r)
		}
	}
	if len(tail) == 0 {
		return dir
	}
	return dir + "-" + string(tail)
}

func isRosterChar(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
