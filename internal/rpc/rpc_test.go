package rpc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeService struct{}

type pair struct {
	A string `json:"a"`
	B int    `json:"b"`
}

func (fakeService) Add(a, b int) int                { return a + b }
func (fakeService) Fail() error                     { return errors.New("boom") }
func (fakeService) Both(ok bool) (string, error)    { return "yes", nil }
func (fakeService) Struct(p pair) (pair, error)     { return p, nil }
func (fakeService) Nothing(id string) error         { return nil }
func (fakeService) Slices(ids []string) (int, bool) { return len(ids), true }

func newTestHandler() *Handler {
	h := New()
	h.Register("fake", fakeService{})
	h.Deny("fake.Nothing")
	return h
}

func call(t *testing.T, h *Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rpc/"+method, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCallWithResult(t *testing.T) {
	rec := call(t, newTestHandler(), "fake.Add", `[2, 3]`)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "5" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestCallReturningError(t *testing.T) {
	rec := call(t, newTestHandler(), "fake.Fail", `[]`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "boom" {
		t.Fatalf("error body: %q", rec.Body.String())
	}
}

func TestValueAndNilError(t *testing.T) {
	rec := call(t, newTestHandler(), "fake.Both", `[true]`)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `"yes"` {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestStructRoundTrip(t *testing.T) {
	rec := call(t, newTestHandler(), "fake.Struct", `[{"a":"x","b":7}]`)
	var p pair
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.A != "x" || p.B != 7 {
		t.Fatalf("round trip: %q", rec.Body.String())
	}
}

func TestErrorOnlyMethodDenied(t *testing.T) {
	rec := call(t, newTestHandler(), "fake.Nothing", `["id"]`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("denied method: want 404, got %d", rec.Code)
	}
}

func TestUnknownServiceAndMethod(t *testing.T) {
	if rec := call(t, newTestHandler(), "nope.Add", `[]`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown service: %d", rec.Code)
	}
	if rec := call(t, newTestHandler(), "fake.Nope", `[]`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown method: %d", rec.Code)
	}
	if rec := call(t, newTestHandler(), "malformed", `[]`); rec.Code != http.StatusNotFound {
		t.Fatalf("malformed name: %d", rec.Code)
	}
}

func TestArgumentValidation(t *testing.T) {
	if rec := call(t, newTestHandler(), "fake.Add", `[1]`); rec.Code != http.StatusBadRequest {
		t.Fatalf("arity: %d", rec.Code)
	}
	if rec := call(t, newTestHandler(), "fake.Add", `{"a":1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("non-array: %d", rec.Code)
	}
	if rec := call(t, newTestHandler(), "fake.Add", `["x", 2]`); rec.Code != http.StatusBadRequest {
		t.Fatalf("type mismatch: %d", rec.Code)
	}
}

func TestMultipleNonErrorReturnsUseFirst(t *testing.T) {
	rec := call(t, newTestHandler(), "fake.Slices", `[["a","b"]]`)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "2" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestOptionsPreflightAndMethodGate(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodOptions, "/rpc/fake.Add", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}
	req = httptest.NewRequest(http.MethodGet, "/rpc/fake.Add", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", rec.Code)
	}
}
