package project

import "strings"

// emptyTreeHash is git's well-known empty tree object, the diff base for a
// repository whose HEAD does not exist yet (no commits).
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// workTreeStatus is everything one `git status --porcelain=v2 --branch` read
// answers at once: the branch, the commit it sits on, and every dirty entry.
// It replaces three separate git children — `symbolic-ref --short HEAD`,
// `rev-parse --verify HEAD` and `ls-files --others` — on a read the frontend
// runs per second per checkout on screen, where process startup, not work, is
// what a tick costs on a normal repository.
type workTreeStatus struct {
	branch    string   // "" for a detached HEAD, as Branch answers
	head      string   // "" in a repository with no commits
	base      string   // what a diff runs against: "HEAD", or the empty tree
	files     int      // dirty entries: changed, renamed, unmerged, untracked
	untracked []string // untracked paths, relative to the work tree root
}

// readWorkTree runs the single status call and parses it. A path git will not
// read as a work tree yields the zero readout, whose base is the empty tree —
// the same pair diffBase handed back before, so the callers behind it fail the
// way they always did rather than on a new code path.
//
// -z, because the counters index the untracked paths straight into the
// filesystem and v2 C-quotes them otherwise. --untracked-files=all, because the
// default collapses a new directory into one "? dir/" entry: an agent writing
// 25 files into fresh packages would count as one changed file.
func readWorkTree(path string) workTreeStatus {
	out, ok := gitQuiet(path, "status", "--porcelain=v2", "--branch",
		"--untracked-files=all", "-z")
	if !ok {
		return workTreeStatus{base: emptyTreeHash}
	}
	return parseWorkTree(out)
}

// parseWorkTree reads porcelain v2's NUL-terminated records. Every record shape
// git does not promise here is skipped rather than guessed at: this one parse
// feeds the branch, the head and the dirty count together, so a line it cannot
// read must cost that line alone.
func parseWorkTree(out string) workTreeStatus {
	status := workTreeStatus{base: emptyTreeHash}
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		switch record[0] {
		case '#':
			status.readHeader(record)
		case '2':
			// A rename or copy spells its original path as a second record of
			// its own, which v1 kept on the one "R orig -> new" line. Consumed
			// here, or one renamed file would count as two dirty ones.
			i++
			status.files++
		case '1', 'u':
			status.files++
		case '?':
			status.files++
			if _, path, ok := strings.Cut(record, " "); ok {
				status.untracked = append(status.untracked, path)
			}
		}
	}
	return status
}

// readHeader takes the two `# branch.*` lines the readout needs, mapping v2's
// sentinels back onto the contracts the separate calls held: a detached HEAD
// names no branch, and a repository with no commits has no HEAD to diff
// against — git's empty tree stands in for it. Left unmapped, the badge would
// read a literal "(detached)" and a diff would run against "(initial)".
func (s *workTreeStatus) readHeader(record string) {
	key, value, ok := strings.Cut(strings.TrimPrefix(record, "# "), " ")
	if !ok {
		return
	}
	switch key {
	case "branch.head":
		if value != "(detached)" {
			s.branch = value
		}
	case "branch.oid":
		if value != "(initial)" {
			s.head, s.base = value, "HEAD"
		}
	}
}
