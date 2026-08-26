package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/omartelo/lich/internal/relay"
)

// mcpServersKey is the key an `mcpServers` document holds its servers under —
// the Claude Desktop shape, which oh-my-pi and Cursor CLI both adopted
// wholesale. One constant, because it is one wire format and not two.
const mcpServersKey = "mcpServers"

// mcpDocument is the `mcpServers` document at path with lich's server
// registered in it, leaving every other server and every other key exactly as
// they were. Nil when lich cannot name its own binary: a registration pointing
// at a command that cannot run is worse than none.
//
// Two harnesses read one of these — oh-my-pi's, beside its extensions, and
// Cursor's under ~/.cursor — and neither takes a `--mcp-config` flag, so for
// both the file is the whole registration. It is the same trade Crush's crushrc
// block makes, one step worse: JSON has no comment to hide a marker in and no
// way to append, so lich rewrites the document. Every key survives the round
// trip; the user's formatting does not.
//
// The path written is this binary's, resolved now (lichBinary) — a document
// cannot expand a variable per session. It is only the transport; which lich a
// session reaches is decided by the coordinates in its PTY.
func mcpDocument(path, lichBin string) ([]byte, error) {
	if lichBin == "" {
		return nil, nil
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	config := map[string]any{}
	if len(existing) > 0 {
		// Refused rather than replaced: the file belongs to the user, and
		// overwriting one lich failed to understand would delete servers it
		// cannot see.
		if err := json.Unmarshal(existing, &config); err != nil {
			return nil, fmt.Errorf("%s is not a JSON object lich can merge into: %w", path, err)
		}
	}
	servers, ok := config[mcpServersKey].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}
	servers[relay.MCPServerName] = map[string]any{
		"command": lichBin,
		"args":    []string{relay.MCPSubcommand},
	}
	config[mcpServersKey] = servers

	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", path, err)
	}
	return append(body, '\n'), nil
}
