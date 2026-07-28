package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/omartelo/lich/internal/relpath"
)

// maxReadFileSize caps a previewed file. CodeMirror is a source viewer, not a
// blob viewer, and the RPC body limit rejects a response this large anyway
// (internal/rpc.bodyLimit). Kept in step with the diff viewer's own ceilings.
const maxReadFileSize = 1 << 20

// Tree lists the work tree's files as repo-relative, slash-separated paths,
// sorted. It merges tracked files with untracked-but-not-ignored ones
// (`ls-files --cached --others --exclude-standard`) and drops any tracked file
// deleted from disk (`--deleted`), so a file created or removed since the
// session began shows without a commit. .gitignore is honored for free, so no
// node_modules and no build output leak in. A non-repository path yields an
// error, matching DiffText's contract.
func (s *Service) Tree(path string) ([]string, error) {
	present, err := lsFiles(path, "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	deleted, err := lsFiles(path, "--deleted")
	if err != nil {
		return nil, err
	}
	gone := make(map[string]struct{}, len(deleted))
	for _, rel := range deleted {
		gone[rel] = struct{}{}
	}
	var files []string
	for _, rel := range present {
		if _, ok := gone[rel]; !ok {
			files = append(files, rel)
		}
	}
	// The --cached/--others merge is not globally sorted; the tree wants one order.
	slices.Sort(files)
	return files, nil
}

// lsFiles runs `git ls-files -z` with the given selectors and splits its
// NUL-delimited output into repo-relative paths.
func lsFiles(path string, args ...string) ([]string, error) {
	out, err := runGit(path, append([]string{"ls-files", "-z"}, args...)...)
	if err != nil {
		return nil, err
	}
	return splitNUL(out), nil
}

// ReadFile returns the text content of one repo-relative file for the read-only
// preview. rel is validated against traversal (internal/relpath, shared with
// DiscardFile and the editor launch) before it is joined onto the work-tree
// root. Binaries, irregular files, and files above maxReadFileSize are refused —
// the preview is for source, not blobs.
func (s *Service) ReadFile(path, rel string) (string, error) {
	if err := relpath.Validate(rel); err != nil {
		return "", err
	}
	full := filepath.Join(path, rel)
	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", rel)
	}
	if info.Size() > maxReadFileSize {
		return "", fmt.Errorf("%s is too large to preview (%d bytes)", rel, info.Size())
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	// git's own binary heuristic: a NUL byte in the first 8000 bytes.
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		return "", fmt.Errorf("%s is a binary file", rel)
	}
	return string(data), nil
}
