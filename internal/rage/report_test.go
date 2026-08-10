package rage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/singleton"
)

func TestRenderSaysSoWhenAProbeFoundNothing(t *testing.T) {
	page := render(Report{Version: "dev", GoVersion: "go1.25.0", Revision: "abc1234-dirty"})

	for _, want := range []string{
		"detection failed",      // no providers came back
		"| — | — | — | — | — |", // no plugin row
		"the config directory could not be read",
		"go1.25.0, revision abc1234-dirty", // the only id a dev build has
		"none — lich logged to stderr only",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("report.md is missing %q:\n%s", want, page)
		}
	}
}

func TestRenderNamesTheProvidersAndThePlugin(t *testing.T) {
	page := render(Report{
		Providers: []providers.Detected{
			{ID: "claude", Name: "Claude Code", Installed: true, Path: "/usr/bin/claude"},
			{ID: "crush", Name: "Crush"},
		},
		Plugin: []agentplugin.Status{
			{Provider: "claude", Name: "Claude Code", Available: true, LatestVersion: "0.3.0"},
		},
		Entries: []Entry{{Name: "lich.db", Size: 2048}, {Name: "themes", Dir: true}},
	})

	for _, want := range []string{
		"| Claude Code | yes | /usr/bin/claude |",
		"| Crush | no | — |",
		"| Claude Code | yes | no | — | 0.3.0 |",
		"| lich.db | 2.0 KiB |",
		"| themes/ | directory |",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("report.md is missing %q:\n%s", want, page)
		}
	}
}

func TestEnvMasksBySecretNameAndReportsWhatIsUnset(t *testing.T) {
	env := renderEnv([]string{
		"SHELL=/bin/zsh",
		"GH_TOKEN=ghp_realvalue",
		"LICH_LISTEN_PORT=47821",
		"LICH_TOKEN=" + liveToken,
		"AWS_SECRET_ACCESS_KEY=nobody-asked",
		"malformed",
	}, liveToken)

	for _, want := range []string{
		"SHELL=/bin/zsh",
		"GH_TOKEN=<SET>",
		"GITHUB_TOKEN=<UNSET>",
		"EDITOR=<UNSET>",
		"LICH_LISTEN_PORT=47821",
		"LICH_TOKEN=<SET>",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("env.txt is missing %q:\n%s", want, env)
		}
	}
	for _, never := range []string{"ghp_realvalue", liveToken, "AWS_SECRET_ACCESS_KEY", "malformed"} {
		if strings.Contains(env, never) {
			t.Errorf("env.txt carries %q, which is outside the allowlist or is a value:\n%s", never, env)
		}
	}
}

func TestScrubLeavesAShortTokenAlone(t *testing.T) {
	// A two-character token would match half the log; masking by exact value is
	// only safe once the value is distinctive.
	got := string(scrub([]byte("a session ran at path/to/a"), "a"))
	if got != "a session ran at path/to/a" {
		t.Errorf("scrub(%q) = %q", "a", got)
	}
}

func TestHumanSizeScalesToTheUnit(t *testing.T) {
	cases := map[int64]string{
		0:       "0 B",
		512:     "512 B",
		1536:    "1.5 KiB",
		5 << 20: "5.0 MiB",
		3 << 30: "3.0 GiB",
	}
	for size, want := range cases {
		if got := humanSize(size); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", size, got, want)
		}
	}
}

func TestPathBinsAsksForTheDefaultBinary(t *testing.T) {
	// The empty string is agentplugin's "the provider's default name on PATH";
	// anything else would be rage opening the workspace database.
	if got := (pathBins{}).ProviderBin("claude", "p1"); got != "" {
		t.Errorf("ProviderBin = %q, want the default", got)
	}
}

func TestNewProbesTheConfigDirItWasGiven(t *testing.T) {
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "lich"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LICH_DEV", "")
	recorded, err := json.Marshal(singleton.Info{PID: 99, Port: 1, Token: liveToken})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "lich", "runtime.json"), recorded, 0o600); err != nil {
		t.Fatal(err)
	}

	c := New(configDir, "v9", nil)
	info, alive := c.probe()

	if info == nil || info.PID != 99 {
		t.Fatalf("probe read %+v, want the runtime file under the given config dir", info)
	}
	if alive {
		t.Error("probe called a recorded instance alive with nothing listening on its port")
	}
	if _, err := c.browser(); err != nil && !strings.Contains(err.Error(), "no chromium-family browser") {
		t.Errorf("browser probe failed for an unexpected reason: %v", err)
	}
	if len(c.detect()) == 0 {
		t.Error("provider detection returned no rows at all — every known provider should be listed")
	}
}
