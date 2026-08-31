package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/omartelo/lich/internal/relpath"
)

// maxReadFileSize caps a previewed file. CodeMirror is a source viewer, not a
// blob viewer, and the RPC body limit rejects a response this large anyway
// (internal/rpc.bodyLimit). Deliberately tighter than maxTextFileBytes: this one
// crosses the wire, the others only ever reach a line count or a diff.
const maxReadFileSize = 1 << 20

// maxTextFileBytes caps the files lich will read whole: the untracked-file line
// count and the untracked new-file diff. Untracked source files are small;
// stream in chunks if that assumption ever breaks.
const maxTextFileBytes = 10 << 20

// binarySniffBytes is how far into a file git looks for a NUL before calling it
// binary. Matching git means a file lich refuses to preview is the same file git
// refuses to diff — the two answers have to agree or the review panel
// contradicts the diff beside it.
const binarySniffBytes = 8000

func isBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), binarySniffBytes)], 0) >= 0
}

// Tree lists a directory's files as root-relative, slash-separated paths,
// sorted. Inside a repository it merges tracked files with
// untracked-but-not-ignored ones (`ls-files --cached --others
// --exclude-standard`) and drops any tracked file deleted from disk
// (`--deleted`), so a file created or removed since the session began shows
// without a commit; .gitignore is honored for free, so no node_modules and no
// build output leak in. Anywhere else — a plain folder, a machine without git —
// it falls back to walking the directory: browsing a project's files is not a
// git feature, and only the diff panel beside it is.
func (s *Service) Tree(path string) ([]string, error) {
	// Asked before anything is listed, and quietly: a plain directory is not a
	// failure to report, and routing its refreshes through runGit would file a
	// warning per poll for a folder that is behaving exactly as it should. A
	// work tree still goes through git, so a repository that *is* broken keeps
	// reporting its error instead of being walked as if it were a plain folder.
	if _, ok := gitQuiet(path, "rev-parse", "--is-inside-work-tree"); !ok {
		return walkFiles(path, walkLimit)
	}
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

// walkLimit caps a non-repository walk. Without git there is no .gitignore to
// obey, so the walk has no way to leave a dependency or build directory out;
// the cap is what keeps a home directory or a node_modules from arriving as one
// RPC answer the tree then has to render.
const walkLimit = 20000

// walkFiles lists a plain directory's regular files, root-relative and
// slash-separated, stopping at limit files (walkLimit; a parameter so the cap
// is testable without laying down 20k files). Symlinks are skipped (nothing here resolves one, and a link
// to a directory is a walk that may not terminate), .git is skipped whole, and
// an unreadable subdirectory costs its own subtree rather than the answer.
func walkFiles(root string, limit int) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			if full == root {
				return err
			}
			if entry != nil && !entry.IsDir() {
				return nil
			}
			return fs.SkipDir
		}
		if entry.IsDir() {
			if full != root && entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		if len(files) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", root, err)
	}
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
// preview. rel goes through relpath.Resolve rather than the lexical Validate
// its neighbours use, because this is the surface that reads bytes: a checkout
// can ship a symlink out of the tree, and a preview that followed one would
// print a file the diff beside it describes as the link's target text.
// Binaries, irregular files, and files above maxReadFileSize are refused —
// the preview is for source, not blobs.
func (s *Service) ReadFile(path, rel string) (string, error) {
	full, err := relpath.Resolve(path, rel)
	if err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return "", err
		}
		return "", fmt.Errorf("stat %s: %w", rel, pathErr.Err)
	}
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
	if isBinary(data) {
		return "", fmt.Errorf("%s is a binary file", rel)
	}
	return string(data), nil
}
