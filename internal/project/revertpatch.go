package project

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

type patchLine struct {
	kind byte // ' ', '-' or '+'
	text string
	old  int // old-side line number; 0 for an addition
	new  int // new-side line number; 0 for a deletion
	// noEOL carries git's "\ No newline at end of file" marker, which belongs
	// to the line before it and has to travel with that line.
	noEOL bool
}

type patchHunk struct {
	oldStart, oldCount, newStart, newCount int
	lines                                  []patchLine
}

// parseHunks reads the hunks of a single-file unified diff. The file headers
// above the first hunk are skipped; the counts in each header are what end a
// hunk, so a content line that happens to read "--- " is never taken for one.
func parseHunks(diff string) ([]patchHunk, error) {
	rows := strings.Split(diff, "\n")
	var hunks []patchHunk
	for i := 0; i < len(rows); {
		if !strings.HasPrefix(rows[i], "@@") {
			i++
			continue
		}
		hunk, next, err := parseHunk(rows, i)
		if err != nil {
			return nil, err
		}
		hunks = append(hunks, hunk)
		i = next
	}
	return hunks, nil
}

func parseHunk(rows []string, at int) (patchHunk, int, error) {
	match := hunkHeader.FindStringSubmatch(rows[at])
	if match == nil {
		return patchHunk{}, 0, fmt.Errorf("malformed hunk header %q", rows[at])
	}
	hunk := patchHunk{
		oldStart: atoi(match[1]), oldCount: countOr1(match[2]),
		newStart: atoi(match[3]), newCount: countOr1(match[4]),
	}
	old, cur, oldLeft, newLeft := hunk.oldStart, hunk.newStart, hunk.oldCount, hunk.newCount
	i := at + 1
	for ; i < len(rows) && (oldLeft > 0 || newLeft > 0); i++ {
		row := rows[i]
		if row == "" {
			return patchHunk{}, 0, errors.New("diff ended inside a hunk")
		}
		line := patchLine{kind: row[0], text: row[1:]}
		switch line.kind {
		case ' ':
			line.old, line.new = old, cur
			old, cur, oldLeft, newLeft = old+1, cur+1, oldLeft-1, newLeft-1
		case '-':
			line.old, old, oldLeft = old, old+1, oldLeft-1
		case '+':
			line.new, cur, newLeft = cur, cur+1, newLeft-1
		case '\\':
			markNoEOL(&hunk)
			continue
		default:
			return patchHunk{}, 0, fmt.Errorf("unexpected diff line %q", row)
		}
		hunk.lines = append(hunk.lines, line)
	}
	if oldLeft != 0 || newLeft != 0 {
		return patchHunk{}, 0, errors.New("hunk is shorter than its header")
	}
	if i < len(rows) && strings.HasPrefix(rows[i], `\`) {
		markNoEOL(&hunk)
		i++
	}
	return hunk, i, nil
}

func markNoEOL(hunk *patchHunk) {
	if n := len(hunk.lines); n > 0 {
		hunk.lines[n-1].noEOL = true
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// countOr1 reads a hunk header count, which git omits when it is one.
func countOr1(s string) int {
	if s == "" {
		return 1
	}
	return atoi(s)
}

type pickKey struct {
	side string
	line int
}

// partialPatch cuts a patch for rel holding only the picked changes, shaped to
// be reversed onto the side the diff's new lines come from: an addition left
// alone is still there, so it stays as context, and a deletion left alone is
// not, so it drops out. Every pick has to name a changed line whose text still
// matches, or the whole cut is refused as stale.
func partialPatch(rel string, hunks []patchHunk, picks []RevertLine) (string, error) {
	wanted := make(map[pickKey]string, len(picks))
	for _, pick := range picks {
		if pick.Side != "old" && pick.Side != "new" {
			return "", fmt.Errorf("unknown side %q", pick.Side)
		}
		wanted[pickKey{pick.Side, pick.Line}] = pick.Text
	}
	var out strings.Builder
	out.WriteString(patchFileHeader(rel))
	used := 0
	for _, hunk := range hunks {
		lines, picked, err := cutHunk(hunk, wanted)
		if err != nil {
			return "", err
		}
		if picked > 0 {
			writeHunk(&out, hunk, lines)
			used += picked
		}
	}
	if used != len(wanted) {
		return "", ErrLinesMoved
	}
	return out.String(), nil
}

// cutHunk keeps the picked changes as changes; see partialPatch for the rest.
func cutHunk(hunk patchHunk, wanted map[pickKey]string) ([]patchLine, int, error) {
	var lines []patchLine
	picked := 0
	for _, line := range hunk.lines {
		key, changed := changeKey(line)
		if !changed {
			lines = append(lines, line)
			continue
		}
		if text, ok := wanted[key]; ok {
			if text != line.text {
				return nil, 0, ErrLinesMoved
			}
			lines = append(lines, line)
			picked++
			continue
		}
		if line.kind == '+' {
			line.kind = ' '
			lines = append(lines, line)
		}
	}
	return lines, picked, nil
}

func changeKey(line patchLine) (pickKey, bool) {
	switch line.kind {
	case '-':
		return pickKey{"old", line.old}, true
	case '+':
		return pickKey{"new", line.new}, true
	}
	return pickKey{}, false
}

// writeHunk renders a cut hunk. The new side keeps its real start; the old one
// is only git's search hint, since the context is what locates the hunk. A side
// left with no lines is numbered from the line before it, as git does.
func writeHunk(out *strings.Builder, hunk patchHunk, lines []patchLine) {
	oldCount, newCount := 0, 0
	for _, line := range lines {
		if line.kind != '+' {
			oldCount++
		}
		if line.kind != '-' {
			newCount++
		}
	}
	start := hunk.newStart
	fmt.Fprintf(out, "@@ -%s +%s @@\n", hunkRange(start, oldCount), hunkRange(start, newCount))
	for _, line := range lines {
		out.WriteByte(line.kind)
		out.WriteString(line.text)
		out.WriteByte('\n')
		if line.noEOL {
			out.WriteString("\\ No newline at end of file\n")
		}
	}
}

func hunkRange(start, count int) string {
	if count == 0 {
		return fmt.Sprintf("%d,0", max(start-1, 0))
	}
	return fmt.Sprintf("%d,%d", start, count)
}

func patchFileHeader(rel string) string {
	return "--- a/" + rel + "\n+++ b/" + rel + "\n"
}

// checkPatchFile holds a patch handed back by the frontend to the shape
// partialPatch writes for rel: that one file's header, then whole hunks and
// nothing else, so it cannot reach another path in the checkout.
func checkPatchFile(patch, rel string) error {
	body, ok := strings.CutPrefix(patch, patchFileHeader(rel))
	if !ok || !strings.HasPrefix(body, "@@") {
		return errors.New("patch does not belong to this file")
	}
	rows := strings.Split(body, "\n")
	for i := 0; i < len(rows); {
		if rows[i] == "" && i == len(rows)-1 {
			return nil
		}
		_, next, err := parseHunk(rows, i)
		if err != nil {
			return fmt.Errorf("patch does not belong to this file: %w", err)
		}
		i = next
	}
	return nil
}

// oldLineOf maps a line of a diff's new side to the same line on its old side,
// or false when the diff added it and the old side never had it.
func oldLineOf(hunks []patchHunk, line int) (int, bool) {
	shift := 0
	for _, hunk := range hunks {
		if line < hunk.newStart {
			break
		}
		if line >= hunk.newStart+hunk.newCount {
			shift += hunk.newCount - hunk.oldCount
			continue
		}
		for _, l := range hunk.lines {
			if l.new == line {
				return l.old, l.kind == ' '
			}
		}
	}
	return line - shift, true
}

// hasChange reports whether a diff carries this exact changed line.
func hasChange(hunks []patchHunk, key pickKey, text string) bool {
	for _, hunk := range hunks {
		for _, line := range hunk.lines {
			if k, changed := changeKey(line); changed && k == key && line.text == text {
				return true
			}
		}
	}
	return false
}
