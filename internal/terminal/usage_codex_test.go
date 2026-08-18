package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanCodexContextUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	body := `{"type":"turn_context","payload":{"model":"gpt-5.6-sol","effort":"high"}}` + "\n" +
		`{"type":"response_item","payload":{"text":"` + strings.Repeat("x", 2*int(codexScanBlockBytes)) + `"}}` + "\n" +
		`not json` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":300000},"model_context_window":258400}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":null}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := scanCodexContextUsage(path)
	if !ok {
		t.Fatal("scanCodexContextUsage: want ok, got false")
	}
	want := contextUsage{
		tokens: 300000, percent: 100, window: 258400, model: "gpt-5.6-sol", effort: "high",
	}
	if got != want {
		t.Errorf("usage = %+v, want %+v", got, want)
	}
}

func TestCodexContextUsageMisses(t *testing.T) {
	for _, body := range []string{
		`{"type":"event_msg","payload":{"type":"token_count","info":null}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"model_context_window":258400}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":-1},"model_context_window":258400}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":1},"model_context_window":0}}}`,
	} {
		path := filepath.Join(t.TempDir(), "rollout.jsonl")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := scanCodexContextUsage(path); ok {
			t.Errorf("scanCodexContextUsage(%s): want not-ok", body)
		}
	}
	if _, ok := scanCodexContextUsage(filepath.Join(t.TempDir(), "missing.jsonl")); ok {
		t.Error("a missing rollout should be not-ok")
	}
}

func TestCodexMaxContextWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	path := filepath.Join(dir, "models_cache.json")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"models":[{"slug":"gpt-5.6-sol","context_window":272000,"max_context_window":872000,"effective_context_window_percent":95}]}`)
	if got, ok := codexMaxContextWindow("gpt-5.6-sol"); !ok || got != 828400 {
		t.Errorf("codexMaxContextWindow = %d, %v; want 828400, true", got, ok)
	}
	if _, ok := codexMaxContextWindow("unknown"); ok {
		t.Error("unknown model should be not-ok")
	}

	write(`{"models":[{"slug":"gpt-5.6-sol","max_context_window":872000,"effective_context_window_percent":101}]}`)
	if _, ok := codexMaxContextWindow("gpt-5.6-sol"); ok {
		t.Error("invalid effective percent should be not-ok")
	}
	write(`not json`)
	if _, ok := codexMaxContextWindow("gpt-5.6-sol"); ok {
		t.Error("malformed cache should be not-ok")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := codexMaxContextWindow("gpt-5.6-sol"); ok {
		t.Error("missing cache should be not-ok")
	}
}
