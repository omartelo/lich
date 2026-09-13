package terminal

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
)

// incompatibleRecorder wires a pluginVersions to a list of what it raised.
func incompatibleRecorder(p *pluginVersions) func() []string {
	var mu sync.Mutex
	var got []string
	p.setOnIncompatible(func(id, version string) {
		mu.Lock()
		got = append(got, id+"@"+version)
		mu.Unlock()
	})
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

func TestPluginVersionsRaisesOncePerSessionAndRelease(t *testing.T) {
	var p pluginVersions
	raised := incompatibleRecorder(&p)

	p.note("a", "0.13.0") // compatible
	p.note("a", "")       // a plugin older than the header, so below the floor
	p.note("a", "9.0.0")
	p.note("a", "9.0.0") // repeat report, same release
	p.note("b", "9.0.0") // another session
	p.note("a", "0.13.0")
	p.note("a", "9.0.0") // back past the ceiling after a respawn
	p.forget("b")
	p.note("b", "9.0.0") // a closed session's id reused

	want := []string{"a@", "a@9.0.0", "b@9.0.0", "a@9.0.0", "b@9.0.0"}
	if got := raised(); !slices.Equal(got, want) {
		t.Fatalf("raised %v, want %v", got, want)
	}
}

func TestPluginVersionsWithoutCallback(t *testing.T) {
	var p pluginVersions
	p.note("a", "9.0.0") // must not panic with nothing wired
}

// The header rides every hook endpoint, since servePost is where it is read.
func TestHookEndpointsReadThePluginHeader(t *testing.T) {
	for _, e := range hookEndpoints {
		t.Run(e.path, func(t *testing.T) {
			tr := newNilTransport(t)
			raised := incompatibleRecorder(&tr.plugins)
			url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", tr.port, e.path, tr.token)
			req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(e.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set(pluginVersionHeader, "9.0.0")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			_ = resp.Body.Close()
			// Out of range is warned about, never refused.
			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", resp.StatusCode)
			}
			if got := raised(); !slices.Equal(got, []string{"s@9.0.0"}) {
				t.Fatalf("raised %v, want the session and release the header named", got)
			}
		})
	}
}

func TestPluginLabel(t *testing.T) {
	if got := pluginLabel(""); got != "absent" {
		t.Errorf("pluginLabel(\"\") = %q", got)
	}
	if got := pluginLabel("0.13.0"); got != "0.13.0" {
		t.Errorf("pluginLabel(\"0.13.0\") = %q", got)
	}
}
