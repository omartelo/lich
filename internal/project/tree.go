package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
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

// FileListing is one listing of a directory's files: the paths, plus the two
// things the Files panel has to say out loud about how they were gathered. A
// tree that silently stops short, or silently leaves a directory out, reads as
// a folder with fewer files in it than it has.
type FileListing struct {
	Files []string `json:"files"`
	// Cut reports a listing that stopped at walkLimit. Only a plain-folder walk
	// can set it; git's own listing is never capped.
	Cut bool `json:"cut"`
	// Hidden names the walkIgnore directories this folder actually had, sorted
	// and deduped: the panel names them, so somebody keeping source under a
	// build/ or vendor/ reads why it is missing instead of guessing.
	Hidden []string `json:"hidden"`
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
func (s *Service) Tree(path string) (FileListing, error) {
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
		return FileListing{}, err
	}
	deleted, err := lsFiles(path, "--deleted")
	if err != nil {
		return FileListing{}, err
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
	return FileListing{Files: files}, nil
}

// walkLimit caps a non-repository walk. walkIgnore keeps the usual dependency
// and build trees out, but it is a name list and not a .gitignore: a folder can
// still hold more files than the tree has any business rendering, so the cap
// stays as the backstop, and a walk that hits it says so (FileListing.Cut).
const walkLimit = 20000

// walkIgnore is the directory names a plain-folder walk skips at any depth.
// Without git there is no .gitignore to obey, and these are the trees nobody
// opens the Files panel to browse: a machine-written dependency, cache or
// build directory. Deliberately a short list matched by name alone: the
// alternative is a per-project ignore setting for a panel that is a file
// browser, not a build tool.
var walkIgnore = map[string]bool{
	".cache":       true,
	".git":         true,
	".venv":        true,
	"__pycache__":  true,
	"build":        true,
	"dist":         true,
	"node_modules": true,
	"target":       true,
	"vendor":       true,
	"venv":         true,
}

// walkFiles lists a plain directory's regular files, root-relative and
// slash-separated, stopping at limit files (walkLimit; a parameter so the cap
// is testable without laying down 20k files) and reporting whether it stopped
// there and which ignored directories it passed. Symlinks are skipped (nothing
// here resolves one, and a link to a directory is a walk that may not
// terminate), walkIgnore directories are skipped whole, and an unreadable
// subdirectory costs its own subtree rather than the answer.
func walkFiles(root string, limit int) (FileListing, error) {
	var files []string
	cut := false
	hidden := map[string]bool{}
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
			if full != root && walkIgnore[entry.Name()] {
				hidden[entry.Name()] = true
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
			cut = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return FileListing{}, fmt.Errorf("list %s: %w", root, err)
	}
	slices.Sort(files)
	return FileListing{Files: files, Cut: cut, Hidden: slices.Sorted(maps.Keys(hidden))}, nil
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
