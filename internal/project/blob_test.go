package project

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// pngBytes is a PNG signature followed by a NUL, which is what makes git and
// lich call a file binary.
const pngBytes = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

func TestBlobWorkTree(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "img/logo.PNG", pngBytes)

	blob, err := New(nil).Blob(repo, "img/logo.PNG", "")
	if err != nil {
		t.Fatalf("Blob: %v", err)
	}
	if string(blob.Data) != pngBytes {
		t.Errorf("data = %q, want %q", blob.Data, pngBytes)
	}
	if blob.ContentType != "image/png" {
		t.Errorf("content type = %q, want image/png", blob.ContentType)
	}
}

// The review's two sides: HEAD for a working-tree diff, a tree oid for a turn.
// Each reads its own revision while the working tree has moved on.
func TestBlobAtRevision(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "doc.pdf", "%PDF-old\x00")
	git("add", "doc.pdf")
	git("commit", "-m", "pdf")
	tree := strings.TrimSpace(git("rev-parse", "HEAD^{tree}"))
	write(t, repo, "doc.pdf", "%PDF-new\x00")

	svc := New(nil)
	for _, ref := range []string{"HEAD", tree} {
		blob, err := svc.Blob(repo, "doc.pdf", ref)
		if err != nil {
			t.Fatalf("Blob at %s: %v", ref, err)
		}
		if string(blob.Data) != "%PDF-old\x00" || blob.ContentType != "application/pdf" {
			t.Errorf("at %s: %q as %q, want the committed PDF", ref, blob.Data, blob.ContentType)
		}
	}
}

// An added file has no old side and a deleted one no new side; both must read
// as absent, which the page draws as no column at all.
func TestBlobMissingSideIsNotExist(t *testing.T) {
	repo, _ := initRepo(t)
	svc := New(nil)
	for _, ref := range []string{"", "HEAD"} {
		if _, err := svc.Blob(repo, "new.png", ref); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("absent at %q: err = %v, want fs.ErrNotExist", ref, err)
		}
	}
}

func TestBlobRefusals(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "assets/x.png", pngBytes)
	write(t, repo, "notes.txt", "text\n")
	git("add", ".")
	git("commit", "-m", "assets")

	tests := []struct {
		name, rel, ref string
		want           error
	}{
		{"traversal", "../x.png", "", errBlobRequest},
		{"branch name is not a revision", "assets/x.png", "main", errBlobRequest},
		{"option-shaped revision", "assets/x.png", "--output=x", errBlobRequest},
		{"text file", "notes.txt", "", errPreviewUnsupported},
		{"svg stays a text diff", "icon.svg", "", errPreviewUnsupported},
	}
	svc := New(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Blob(repo, tt.rel, tt.ref); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBlobDirectoryNamedLikeAnImage(t *testing.T) {
	repo, git := initRepo(t)
	write(t, repo, "shots.png/a.txt", "x\n")
	git("add", ".")
	git("commit", "-m", "dir")

	svc := New(nil)
	for _, ref := range []string{"", "HEAD"} {
		if _, err := svc.Blob(repo, "shots.png", ref); !errors.Is(err, errBlobRequest) {
			t.Errorf("directory at %q: err = %v, want errBlobRequest", ref, err)
		}
	}
}

// The limit is pinned to a literal and probed on both sides, in the working
// tree and in git's object store, which are read by different code.
func TestBlobSizeLimit(t *testing.T) {
	if maxPreviewBytes != 20<<20 {
		t.Fatalf("maxPreviewBytes = %d, want 20 MiB", maxPreviewBytes)
	}
	repo, git := initRepo(t)
	write(t, repo, "at.png", strings.Repeat("\x00", maxPreviewBytes))
	write(t, repo, "over.png", strings.Repeat("\x00", maxPreviewBytes+1))
	git("add", ".")
	git("commit", "-m", "big")

	svc := New(nil)
	for _, ref := range []string{"", "HEAD"} {
		if _, err := svc.Blob(repo, "at.png", ref); err != nil {
			t.Errorf("at the limit (%q): %v", ref, err)
		}
		if _, err := svc.Blob(repo, "over.png", ref); !errors.Is(err, errPreviewTooLarge) {
			t.Errorf("over the limit (%q): err = %v, want errPreviewTooLarge", ref, err)
		}
	}
}

func serveBlob(t *testing.T, method string, query url.Values, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/blob?"+query.Encode(), nil)
	for key, values := range header {
		req.Header[key] = values
	}
	rec := httptest.NewRecorder()
	New(nil).ServeBlob(rec, req)
	return rec
}

func TestServeBlob(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "logo.png", pngBytes)

	rec := serveBlob(t, http.MethodGet, url.Values{"path": {repo}, "rel": {"logo.png"}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", rec.Code, rec.Body)
	}
	if !bytes.Equal(rec.Body.Bytes(), []byte(pngBytes)) {
		t.Errorf("body = %q, want the file", rec.Body.Bytes())
	}
	for key, want := range map[string]string{
		"Content-Type":           "image/png",
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store",
	} {
		if got := rec.Header().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// Chromium's PDF viewer fetches a large document in ranges.
func TestServeBlobHonoursRange(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "doc.pdf", "%PDF-1.4\x00rest")

	rec := serveBlob(t, http.MethodGet, url.Values{"path": {repo}, "rel": {"doc.pdf"}},
		http.Header{"Range": {"bytes=0-3"}})
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "%PDF" {
		t.Errorf("range: %d %q, want 206 %q", rec.Code, rec.Body, "%PDF")
	}
}

func TestServeBlobStatuses(t *testing.T) {
	repo, _ := initRepo(t)
	write(t, repo, "big.png", strings.Repeat("\x00", maxPreviewBytes+1))

	tests := []struct {
		name   string
		method string
		rel    string
		ref    string
		want   int
	}{
		{"missing side", http.MethodGet, "gone.png", "", http.StatusNotFound},
		{"over the limit", http.MethodGet, "big.png", "", http.StatusRequestEntityTooLarge},
		{"no preview for the type", http.MethodGet, "a.txt", "", http.StatusUnsupportedMediaType},
		{"bad revision", http.MethodGet, "big.png", "main", http.StatusBadRequest},
		{"write method", http.MethodPost, "big.png", "", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := url.Values{"path": {repo}, "rel": {tt.rel}, "ref": {tt.ref}}
			if rec := serveBlob(t, tt.method, query, nil); rec.Code != tt.want {
				t.Errorf("status = %d (%s), want %d", rec.Code, rec.Body, tt.want)
			}
		})
	}
}
