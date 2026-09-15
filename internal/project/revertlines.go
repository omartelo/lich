package project

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/relpath"
)

// ErrLinesMoved is what a revert answers when the lines it was asked about are
// no longer the lines on disk: the diff on screen is older than the file.
var ErrLinesMoved = errors.New("the file changed since this diff was drawn. Refresh and try again")

// ErrStagedDiverged is a revert refused because the index holds a third version
// of these lines, neither HEAD's nor the working tree's.
var ErrStagedDiverged = errors.New("these lines have staged changes that differ from the file. Discard the file instead")

// RevertLine names one changed line of the diff the panel drew. Side "old" is a
// deletion, numbered in HEAD; "new" is an addition, numbered in the working
// tree. Text is the line without its +/- prefix, and is what proves the number
// still points at the same line.
type RevertLine struct {
	Side string `json:"side"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// RevertResult is what undoing a revert needs: the patch reversed onto the
// working tree, and the one reversed onto the index ("" when the index did not
// hold the change and was left alone).
type RevertResult struct {
	Patch      string `json:"patch"`
	IndexPatch string `json:"indexPatch"`
}

// RevertLines puts the chosen changed lines of rel back to HEAD, leaving every
// other change in the file alone. The patch is rebuilt here from git's own diff
// rather than taken from the caller, so a stale screen fails instead of
// reverting whatever those line numbers point at now.
//
// The index is reverted with the working tree whenever it holds the change:
// the panel draws staged and unstaged as one diff, so reverting only the file
// would leave the line in the next commit with nothing on screen showing it.
func (s *Service) RevertLines(path, rel string, lines []RevertLine) (RevertResult, error) {
	if err := relpath.Validate(rel); err != nil {
		return RevertResult{}, err
	}
	if len(lines) == 0 {
		return RevertResult{}, errors.New("no changed lines to revert")
	}
	base := readWorkTree(path).base
	hunks, err := parseHunks(fileDiff(path, rel, base))
	if err != nil {
		return RevertResult{}, err
	}
	patch, err := partialPatch(rel, hunks, lines)
	if err != nil {
		return RevertResult{}, err
	}
	if !applies(path, patch, "-R", "--check") {
		return RevertResult{}, ErrLinesMoved
	}
	indexPatch, err := stagedPatch(path, rel, base, lines)
	if err != nil {
		return RevertResult{}, err
	}
	result := RevertResult{Patch: patch, IndexPatch: indexPatch}
	return result, applyBoth(path, result, true)
}

// RestoreLines undoes RevertLines: the same patches applied forwards, to the
// same targets. They come back from the caller, so each is held to the one file
// it was built for before git sees it.
func (s *Service) RestoreLines(path, rel string, undo RevertResult) error {
	if err := relpath.Validate(rel); err != nil {
		return err
	}
	if err := checkPatchFile(undo.Patch, rel); err != nil {
		return err
	}
	if undo.IndexPatch != "" {
		if err := checkPatchFile(undo.IndexPatch, rel); err != nil {
			return err
		}
	}
	if !applies(path, undo.Patch, "--check") ||
		(undo.IndexPatch != "" && !applies(path, undo.IndexPatch, "--cached", "--check")) {
		return ErrLinesMoved
	}
	return applyBoth(path, undo, false)
}

// fileDiff is the one file's slice of what DiffText draws: tracked against the
// same base, or an untracked file as a new-file diff.
func fileDiff(path, rel, base string) string {
	if out, ok := gitQuiet(path, "diff", base, "--", rel); ok && out != "" {
		return out
	}
	return untrackedDiff(path, rel)
}

// stagedPatch finds the picked lines in the index. Each is looked up in the
// HEAD→index diff, an addition first carried to its index line number through
// the index→worktree diff. None staged leaves the index alone; all staged
// reverts it too; some staged is a third version of these lines, refused.
func stagedPatch(path, rel, base string, lines []RevertLine) (string, error) {
	if _, tracked := gitQuiet(path, "ls-files", "--error-unmatch", "--", rel); !tracked {
		return "", nil
	}
	cached, _ := gitQuiet(path, "diff", "--cached", base, "--", rel)
	staged, err := parseHunks(cached)
	if err != nil {
		return "", err
	}
	worktree, _ := gitQuiet(path, "diff", "--", rel)
	unstaged, err := parseHunks(worktree)
	if err != nil {
		return "", err
	}
	var picks []RevertLine
	for _, line := range lines {
		if pick, ok := inIndex(staged, unstaged, line); ok {
			picks = append(picks, pick)
		}
	}
	if len(picks) == 0 {
		return "", nil
	}
	if len(picks) != len(lines) {
		return "", ErrStagedDiverged
	}
	patch, err := partialPatch(rel, staged, picks)
	if err != nil || !applies(path, patch, "-R", "--cached", "--check") {
		return "", ErrStagedDiverged
	}
	return patch, nil
}

// inIndex restates one pick in the index's numbering, if the index holds it.
func inIndex(staged, unstaged []patchHunk, line RevertLine) (RevertLine, bool) {
	number := line.Line
	if line.Side == "new" {
		indexLine, unchanged := oldLineOf(unstaged, line.Line)
		if !unchanged {
			return RevertLine{}, false
		}
		number = indexLine
	}
	pick := RevertLine{Side: line.Side, Line: number, Text: line.Text}
	return pick, hasChange(staged, pickKey{pick.Side, pick.Line}, pick.Text)
}

// applyBoth applies the patches, the working tree's first. Both were checked
// already; should the index still refuse, the working tree is put back so a
// failure never leaves the two half-reverted.
func applyBoth(path string, result RevertResult, reverse bool) error {
	if err := applyPatch(path, result.Patch, direction(reverse)...); err != nil {
		return err
	}
	if result.IndexPatch == "" {
		return nil
	}
	if err := applyPatch(path, result.IndexPatch, append(direction(reverse), "--cached")...); err != nil {
		if rollback := applyPatch(path, result.Patch, direction(!reverse)...); rollback != nil {
			return fmt.Errorf("%w (and restoring the file failed: %v)", err, rollback)
		}
		return err
	}
	return nil
}

func direction(reverse bool) []string {
	if reverse {
		return []string{"-R"}
	}
	return nil
}

// applies reports whether git apply accepts the patch with these flags; a
// refusal is the answer, not a failure worth logging.
func applies(dir, patch string, args ...string) bool {
	_, err := gitApply(dir, patch, args)
	return err == nil
}

func applyPatch(dir, patch string, args ...string) error {
	stderr, err := gitApply(dir, patch, args)
	if err != nil {
		return gitFailure(append([]string{"apply"}, args...), stderr, err)
	}
	return nil
}

func gitApply(dir, patch string, args []string) (string, error) {
	full := append([]string{"-C", dir, "apply"}, args...)
	cmd := command("git", append(full, "-")...)
	cmd.Stdin = strings.NewReader(patch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}
