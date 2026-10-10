package terminal

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/providers"
)

// opencodeConfigContentVar is opencode's inline config, merged over every config
// file it reads as the last local-scope layer (measured in opencode 1.18.35).
const opencodeConfigContentVar = "OPENCODE_CONFIG_CONTENT"

// pasteUnfoldEnv turns off opencode's paste summary, the "[Pasted ~N lines]"
// placeholder, when the user asked for long pastes to land unfolded. Claude Code
// is answered in the window instead (frontend/src/lib/terminal/paste-unfold.ts),
// and no other provider has a switch for its fold.
//
// An OPENCODE_CONFIG_CONTENT the user already exports is extended rather than
// replaced: the child keeps only the last value of a repeated key, so appending
// a second one would drop theirs without a word.
func pasteUnfoldEnv(env []string, kind string, on bool) ([]string, error) {
	if !on || kind != providers.OpenCode {
		return env, nil
	}
	config := map[string]any{}
	if own := lastEnvValue(env, opencodeConfigContentVar); own != "" {
		if err := json.Unmarshal([]byte(own), &config); err != nil {
			return nil, fmt.Errorf("%s must be a JSON object to turn off opencode's paste summary: %w",
				opencodeConfigContentVar, err)
		}
	}
	experimental, isObject := config["experimental"].(map[string]any)
	if config["experimental"] != nil && !isObject {
		return nil, fmt.Errorf("%s has an \"experimental\" that is not an object", opencodeConfigContentVar)
	}
	if experimental == nil {
		experimental = map[string]any{}
	}
	experimental["disable_paste_summary"] = true
	config["experimental"] = experimental
	content, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", opencodeConfigContentVar, err)
	}
	return append(env, opencodeConfigContentVar+"="+string(content)), nil
}

// lastEnvValue is the value a child process sees for key: the last entry wins.
func lastEnvValue(env []string, key string) string {
	value := ""
	for _, entry := range env {
		if k, v, ok := strings.Cut(entry, "="); ok && k == key {
			value = v
		}
	}
	return value
}
