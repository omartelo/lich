package relay

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Insert is the other half of what an editor needs from lich: put text at a
// session's prompt and leave it there. Send types a task and presses Enter
// behind it, waiting for an answer; an editor's "send this selection" has none
// of that to do. The person reads what landed, adds the question around it, and
// sends it themselves.

const (
	// insertLimit bounds one insertion. Larger than a relayed prompt because it
	// is pasted, not typed a character at a time, and a selection is the
	// ordinary thing to hold a few hundred lines; far under the 1 MiB a terminal
	// frame may carry (internal/terminal.wsReadLimit), which a TUI would choke on
	// long before.
	insertLimit = 64 * 1024
	// defaultInsertWait is how long Insert waits for a session to reach its
	// prompt. An editor action is answered by a person looking at it, so this is
	// short against Send's minutes: a session still starting is worth a few
	// seconds, one stuck behind a setup script is the person's to wait out.
	defaultInsertWait = 10 * time.Second
)

// Inserted says where the text went, so an editor can tell the person which
// session took it when the target was chosen for them.
type Inserted struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Project string `json:"project"`
	Kind    string `json:"kind"`
	// Bytes is what was pasted, after control characters were stripped (sanitize).
	Bytes int `json:"bytes"`
}

// Insert pastes text at the prompt of the session target names, without
// sending it. target resolves the way Send's does. Left empty, project must
// name where to look — a project by name, or an absolute directory — and the
// session is the one live there: the directory of a session itself wins over
// the root of its project, so an editor open on a worktree reaches the session
// in that worktree and not one beside it. Two candidates are an error naming
// both, never a guess.
//
// The text goes in whole or not at all. A session in the middle of a
// permission prompt is refused: what is typed there is read by the dialog, not
// the prompt. A person's half-written line is not a reason to refuse; the text
// is added to the end of it, which is where it was meant to go.
func (s *Service) Insert(fromID, target, project, text string, waitSeconds int) (Inserted, error) {
	wait := defaultInsertWait
	if waitSeconds > 0 {
		wait = min(time.Duration(waitSeconds)*time.Second, MaxWait)
	}
	return s.insert(fromID, target, project, text, wait)
}

func (s *Service) insert(fromID, target, project, text string, wait time.Duration) (Inserted, error) {
	text = sanitize(text)
	if strings.TrimSpace(text) == "" {
		return Inserted{}, errors.New("nothing to insert: the text is empty")
	}
	if len(text) > insertLimit {
		return Inserted{}, fmt.Errorf("text is %d bytes, over the %d limit: "+
			"insert the path of the file instead of its contents", len(text), insertLimit)
	}
	dest, err := s.insertTarget(fromID, target, project)
	if err != nil {
		return Inserted{}, err
	}
	if s.reportedState(dest.ID) == stateWaiting {
		return Inserted{}, fmt.Errorf(
			"%q is waiting on a permission prompt, and text typed there goes to the dialog "+
				"instead of the prompt: answer it in that session first", dest.Peer.Label)
	}
	if err := s.awaitPrompt(dest, wait); err != nil {
		return Inserted{}, err
	}
	if err := s.term.Write(dest.ID, paste(text)); err != nil {
		return Inserted{}, fmt.Errorf("insert into %q: %w", dest.Peer.Label, err)
	}
	return Inserted{
		ID: dest.ID, Label: dest.Peer.Label, Project: dest.Peer.Project, Kind: dest.Peer.Kind, Bytes: len(text),
	}, nil
}

// insertTarget is resolve for a call that may name no session at all.
func (s *Service) insertTarget(fromID, target, project string) (candidate, error) {
	if strings.TrimSpace(target) != "" {
		return s.resolve(fromID, target, project)
	}
	if strings.TrimSpace(project) == "" {
		return candidate{}, errors.New("no session or project given: name the session, " +
			"or the project (or its directory) to find one in")
	}
	found, err := s.roster(fromID)
	if err != nil {
		return candidate{}, err
	}
	matches := narrowToDirectory(found, project)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return candidate{}, fmt.Errorf("no live session in %q. %s", project, knownLabels(found))
	default:
		return candidate{}, fmt.Errorf(
			"%d live sessions in %q (%s) — name the one to use",
			len(matches), project, labelsOf(matches),
		)
	}
}

// narrowToDirectory keeps the sessions living in project: those running in the
// directory itself when project is one and any do, otherwise those of the
// project it names or roots.
func narrowToDirectory(found []candidate, project string) []candidate {
	var inProject, inDir []candidate
	for _, c := range found {
		if !inProjectNamed(c.Peer, project) {
			continue
		}
		inProject = append(inProject, c)
		if samePath(c.Peer.Path, project) {
			inDir = append(inDir, c)
		}
	}
	if len(inDir) > 0 {
		return inDir
	}
	return inProject
}

// inProjectNamed is whether a peer belongs to the project the argument names:
// a directory matches the project rooted there or a session running there,
// anything else matches the project's name. Empty narrows nothing.
func inProjectNamed(peer Peer, project string) bool {
	if project == "" {
		return true
	}
	if !looksLikePath(project) {
		return strings.EqualFold(peer.Project, project)
	}
	return samePath(peer.ProjectPath, project) || samePath(peer.Path, project)
}

func looksLikePath(name string) bool {
	return strings.ContainsRune(name, '/') ||
		strings.ContainsRune(name, filepath.Separator) ||
		name == "~" || strings.HasPrefix(name, "~/")
}

// samePath is whether two directories are the same one. The filesystem decides
// where it can — case, symlinks and a trailing separator are not differences
// between two spellings of one directory — and the cleaned strings where one
// of them no longer exists.
func samePath(a, b string) bool {
	a, b = expandHome(a), expandHome(b)
	if a == "" || b == "" || !filepath.IsAbs(a) || !filepath.IsAbs(b) {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(infoA, infoB)
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path[1:], "/"))
}

func labelsOf(matches []candidate) string {
	labels := make([]string, 0, len(matches))
	for _, c := range matches {
		labels = append(labels, c.Peer.Label)
	}
	return strings.Join(labels, ", ")
}

// awaitPrompt waits for the session's agent to be sitting at its prompt, up to
// wait. It is awaitFree without the draft check (see Terminal.AtPrompt).
func (s *Service) awaitPrompt(dest candidate, wait time.Duration) error {
	deadline := s.now().Add(wait)
	for !s.term.AtPrompt(dest.ID) {
		if !s.term.Live(dest.ID) {
			return fmt.Errorf("%q stopped before its prompt was ready", dest.Peer.Label)
		}
		if s.now().After(deadline) {
			return fmt.Errorf("%q was not at a prompt after %s: its agent is still starting, "+
				"or the checkout's setup script holds the terminal", dest.Peer.Label, wait)
		}
		time.Sleep(readyPoll)
	}
	return nil
}
