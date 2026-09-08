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
// The relay therefore answers to either. Deriving the roster name is how lich
// mints one, and RosterNameOf is how it reads one back: a session that renamed
// itself answers to the new name and to nothing lich could have derived, so the
// derivation is the fallback rather than the answer.

// rosterIDChars is how much of the session id trails the directory name. Four
// separates the handful of sessions one checkout holds and stays readable in a
// roster row.
const rosterIDChars = 4

// RosterNameOf is the name a session answers to in the peer roster now.
// recorded is what its agent has on record (terminal.AgentName) and wins
// outright: a `/rename` typed inside a session changes the name the harness
// answers to, and no derivation can see that. Falling back to the derived one
// covers everything with nothing recorded — a provider that is never handed a
// roster name, a conversation lich cannot read, a session whose first turn has
// not been written yet.
func RosterNameOf(recorded, cwd, id string) string {
	if recorded = strings.TrimSpace(recorded); recorded != "" {
		return recorded
	}
	return RosterName(cwd, id)
}

// RosterName builds the name lich gives a session at birth: the last element of
// its working directory, then four characters of its id. cwd is the session's
// own directory when it has one (a worktree) and its project's otherwise.
//
// Exported for the one caller outside this package that mints a session rather
// than resolving one: internal/spawn, which reports the name it opened a
// session under.
func RosterName(cwd, id string) string {
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
