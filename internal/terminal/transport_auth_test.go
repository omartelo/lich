package terminal

import (
	"fmt"
	"net/http"
	"testing"
)

// The token check mount applies is the whole of lich's authentication: the RPC
// dispatcher, the event socket and every hook endpoint are reached through it.
// Nothing else asserts it from the outside, so an inverted condition there
// would open the entire surface and still pass this package's suite.

// probe answers 200 to anything that reaches it, so the status code out here is
// the gate's verdict and nothing else.
var probe = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func mountedTransport(t *testing.T) *transport {
	t.Helper()
	tr, err := newTransport(func(string, []byte) {}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	tr.mount("/gated", probe)
	tr.mountPublic("/open", probe)
	return tr
}

func status(t *testing.T, tr *transport, path, token string) int {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d%s", tr.port, path)
	if token != "" {
		url += "?token=" + token
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func TestMountRequiresTheToken(t *testing.T) {
	tr := mountedTransport(t)
	if code := status(t, tr, "/gated", ""); code != http.StatusForbidden {
		t.Errorf("no token: want 403, got %d", code)
	}
	if code := status(t, tr, "/gated", "wrong"); code != http.StatusForbidden {
		t.Errorf("wrong token: want 403, got %d", code)
	}
	if code := status(t, tr, "/gated", tr.token); code != http.StatusOK {
		t.Errorf("with the token: want 200, got %d", code)
	}
}

func TestMountPublicServesWithoutTheToken(t *testing.T) {
	tr := mountedTransport(t)
	if code := status(t, tr, "/open", ""); code != http.StatusOK {
		t.Errorf("public without token: want 200, got %d", code)
	}
}
