package lichclient

import (
	"strings"
	"testing"
)

func envOf(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestRuntimeFromEnv(t *testing.T) {
	rt, err := RuntimeFromEnv(envOf(map[string]string{"LICH_PORT": "47822", "LICH_TOKEN": "tok"}))
	if err != nil || rt.Port != 47822 || rt.Token != "tok" {
		t.Fatalf("got %+v, %v; want port 47822 and token tok", rt, err)
	}
}

func TestRuntimeFromEnvNamesWhatIsMissing(t *testing.T) {
	for _, vars := range []map[string]string{{}, {"LICH_PORT": "47822"}, {"LICH_TOKEN": "tok"}} {
		_, err := RuntimeFromEnv(envOf(vars))
		if err == nil {
			t.Errorf("%v: got no error", vars)
			continue
		}
		for _, name := range []string{"LICH_PORT", "LICH_TOKEN", "-runtime"} {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("%v: error %q does not name %s", vars, err, name)
			}
		}
	}
}

func TestRuntimeFromEnvRejectsABadPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "-1"} {
		if _, err := RuntimeFromEnv(envOf(map[string]string{"LICH_PORT": port, "LICH_TOKEN": "tok"})); err == nil {
			t.Errorf("LICH_PORT=%q: got no error", port)
		}
	}
}
