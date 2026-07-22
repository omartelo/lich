package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxReadFileSize caps a previewed file. CodeMirror is a source viewer, not a
// blob viewer, and the RPC body limit rejects a response this large anyway
// (internal/rpc.bodyLimit). Kept in step with the diff viewer's own ceilings.
const maxReadFileSize = 1 << 20

// Tree lists the repository's tracked files as repo-relative, slash-separated
// paths, already sorted (git's own order). It uses `git ls-files`, so
// .gitignore is honored for free and only versioned files appear — no
// node_modules, no build output. A non-repository path yields an error,
// matching DiffText's contract.
//
// Ceiling: untracked files are invisible to ls-files, matching the file tree's
// documented limit; merge `git status --porcelain` if they ever need to show.
func (s *Service) Tree(path string) ([]string, error) {
	out, err := runGit(path, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	var files []string
	for rel := range strings.SplitSeq(out, "\x00") {
		if rel != "" {
			files = append(files, rel)
		}
	}
	return files, nil
}

// ReadFile returns the text content of one repo-relative file for the read-only
// preview. rel is validated against traversal (validateRelPath, shared with
// DiscardFile) before it is joined onto the work-tree root. Binaries, irregular
// files, and files above maxReadFileSize are refused — the preview is for
// source, not blobs.
func (s *Service) ReadFile(path, rel string) (string, error) {
	if err := validateRelPath(rel); err != nil {
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
