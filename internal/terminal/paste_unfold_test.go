package terminal

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

func opencodeConfig(t *testing.T, env []string) map[string]any {
	t.Helper()
	config := map[string]any{}
	if err := json.Unmarshal([]byte(lastEnvValue(env, opencodeConfigContentVar)), &config); err != nil {
		t.Fatalf("%s is not JSON: %v", opencodeConfigContentVar, err)
	}
	return config
}

func TestPasteUnfoldEnvTurnsOffOpencodePasteSummary(t *testing.T) {
	env, err := pasteUnfoldEnv([]string{"HOME=/h"}, providers.OpenCode, true)
	if err != nil {
		t.Fatalf("pasteUnfoldEnv: %v", err)
	}
	want := `{"experimental":{"disable_paste_summary":true}}`
	if got := lastEnvValue(env, opencodeConfigContentVar); got != want {
		t.Errorf("%s = %s, want %s", opencodeConfigContentVar, got, want)
	}
	if lastEnvValue(env, "HOME") != "/h" {
		t.Error("the rest of the environment was not kept")
	}
}

func TestPasteUnfoldEnvLeavesEveryOtherSpawnAlone(t *testing.T) {
	base := []string{"HOME=/h"}
	cases := []struct {
		kind string
		on   bool
	}{
		{providers.OpenCode, false},
		{providers.Claude, true},
		{providers.Codex, true},
		{"shell", true},
	}
	for _, c := range cases {
		env, err := pasteUnfoldEnv(slices.Clone(base), c.kind, c.on)
		if err != nil {
			t.Fatalf("%s on=%v: %v", c.kind, c.on, err)
		}
		if !slices.Equal(env, base) {
			t.Errorf("%s on=%v: env = %v, want it untouched", c.kind, c.on, env)
		}
	}
}

func TestPasteUnfoldEnvExtendsTheUsersOwnInlineConfig(t *testing.T) {
	own := opencodeConfigContentVar + `={"model":"m","experimental":{"other":1}}`
	env, err := pasteUnfoldEnv([]string{own}, providers.OpenCode, true)
	if err != nil {
		t.Fatalf("pasteUnfoldEnv: %v", err)
	}
	config := opencodeConfig(t, env)
	if config["model"] != "m" {
		t.Errorf("model = %v, want the user's own kept", config["model"])
	}
	experimental := config["experimental"].(map[string]any)
	if experimental["other"] != float64(1) || experimental["disable_paste_summary"] != true {
		t.Errorf("experimental = %v, want the user's key and disable_paste_summary", experimental)
	}
}

func TestPasteUnfoldEnvRefusesAnInlineConfigItCannotExtend(t *testing.T) {
	for _, own := range []string{`not json`, `{"experimental":true}`} {
		_, err := pasteUnfoldEnv([]string{opencodeConfigContentVar + "=" + own}, providers.OpenCode, true)
		if err == nil {
			t.Errorf("%s=%s: want an error, not the user's config dropped", opencodeConfigContentVar, own)
		}
	}
}
