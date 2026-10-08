package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/relpath"
)

// maxPreviewBytes caps one side of an image or PDF preview. It covers a 4K
// screenshot saved as PNG (5 to 15 MB) with room to spare, and keeps a stray
// asset from being held whole in memory twice, here and in the page.
const maxPreviewBytes = 20 << 20

// previewTypes is both the allowlist and the Content-Type table: what Chromium
// decodes in an <img>, plus PDF for its built-in viewer. SVG is left out on
// purpose: git diffs it as text, and served from lich's own origin a document
// with a script would run there.
var previewTypes = map[string]string{
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".gif":  "image/gif",
	".ico":  "image/x-icon",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".webp": "image/webp",
}

var (
	errPreviewUnsupported = errors.New("this file type can't be previewed")
	errPreviewTooLarge    = errors.New("above the preview limit")
	errBlobRequest        = errors.New("invalid preview request")
)

// Blob is one file's bytes at one revision, with the type a page renders it as.
type Blob struct {
	Data        []byte
	ContentType string
}

// Blob reads rel at ref for an image or PDF preview. ref is "" for the working
// tree, "HEAD" for the commit a checkout's uncommitted diff stands on, or an
// object this clone holds (a turn's snapshot tree). A pull request's head is
// not reached: unlike FileLines there is no GitHub fallback.
//
// A path absent at ref is fs.ErrNotExist: the side of an added or deleted file
// that has nothing to show.
func (s *Service) Blob(dir, rel, ref string) (Blob, error) {
	if err := relpath.Validate(rel); err != nil {
		return Blob{}, fmt.Errorf("%w: %w", errBlobRequest, err)
	}
	if ref != "" && ref != "HEAD" && !isOID(ref) {
		return Blob{}, fmt.Errorf("%w: revision %q", errBlobRequest, ref)
	}
	contentType, ok := previewTypes[strings.ToLower(path.Ext(rel))]
	if !ok {
		return Blob{}, fmt.Errorf("%s: %w", rel, errPreviewUnsupported)
	}
	var data []byte
	var err error
	if ref == "" {
		data, err = workTreeBlob(dir, rel)
	} else {
		data, err = objectBlob(dir, rel, ref)
	}
	if err != nil {
		return Blob{}, err
	}
	return Blob{Data: data, ContentType: contentType}, nil
}

// workTreeBlob reads through relpath.Resolve for the reason ReadFile does: a
// symlink out of the checkout is not the checkout's file.
func workTreeBlob(dir, rel string) ([]byte, error) {
	full, err := relpath.Resolve(dir, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", errBlobRequest, rel)
	}
	if err := checkPreviewSize(rel, info.Size()); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	return data, nil
}

// objectBlob asks git for the size before the bytes, so an oversize object is
// refused without being read.
func objectBlob(dir, rel, ref string) ([]byte, error) {
	object := ref + ":" + rel
	// gitQuiet: a path absent at the revision is the expected answer for an
	// added or deleted file, not a failure to report.
	kind, ok := gitQuiet(dir, "cat-file", "-t", object)
	if !ok {
		return nil, fmt.Errorf("%s at %s: %w", rel, ref, fs.ErrNotExist)
	}
	if strings.TrimSpace(kind) != "blob" {
		return nil, fmt.Errorf("%w: %s is not a regular file", errBlobRequest, rel)
	}
	sizeText, err := runGit(dir, "cat-file", "-s", object)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(sizeText), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("size of %s at %s: %w", rel, ref, err)
	}
	if err := checkPreviewSize(rel, size); err != nil {
		return nil, err
	}
	out, err := runGit(dir, "cat-file", "blob", object)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func checkPreviewSize(rel string, size int64) error {
	if size > maxPreviewBytes {
		return fmt.Errorf("%s is %d bytes, %w of %d bytes", rel, size, errPreviewTooLarge, maxPreviewBytes)
	}
	return nil
}

// ServeBlob is Blob over HTTP, for an <img> or a PDF viewer to load directly
// rather than as base64 inside an RPC answer. The query carries path, rel and
// ref. Range requests are honoured because the PDF viewer makes them.
//
// The page tells the sides apart by status: 404 is a side with no file, 413 a
// file over the limit, 415 a type with no preview. The body says which, in the
// words the page shows.
func (s *Service) ServeBlob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	blob, err := s.Blob(query.Get("path"), query.Get("rel"), query.Get("ref"))
	if err != nil {
		http.Error(w, err.Error(), blobStatus(err))
		return
	}
	w.Header().Set("Content-Type", blob.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// The working tree answers differently under the same URL as the agent
	// writes; the page keys each side by its blob oid instead of by caching.
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(blob.Data))
}

func blobStatus(err error) int {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, errPreviewTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, errPreviewUnsupported):
		return http.StatusUnsupportedMediaType
	case errors.Is(err, errBlobRequest):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
