package main

import (
	"regexp"
	"strings"
)

// The harnesses spell an MCP tool three ways (lib/session/tool-label.ts):
// mcp__<server>__<tool> (Claude Code, Codex), mcp__<server>_<tool>
// (oh-my-pi) and <server>_<tool> (opencode). Antigravity runs every MCP call
// as the one step call_mcp_tool, naming <server>/<tool> in the detail.
var (
	mcpDouble     = regexp.MustCompile(`^mcp__(.+?)__(.+)$`)
	mcpStepDetail = regexp.MustCompile(`^([^/]+)/(.+)$`)
)

const (
	mcpPrefix = "mcp__"
	mcpStep   = "call_mcp_tool"
)

// toolLine is tool-label.ts toolLine: the label a card draws for the tool a
// turn runs, an MCP tool as "<server> · <tool>", and the detail beside it.
func toolLine(tool sessionTool, servers []string) (label, detail string) {
	if tool.Name == mcpStep {
		if step := mcpStepDetail.FindStringSubmatch(tool.Detail); step != nil {
			return step[1] + " · " + step[2], ""
		}
	}
	return toolLabel(tool.Name, servers), tool.Detail
}

// toolLabel splits the doubled form on the string alone; the single
// underscore forms only against the servers the session's spawn found,
// longest first. A name matching none keeps what survives the prefix.
func toolLabel(name string, servers []string) string {
	if parts := mcpDouble.FindStringSubmatch(name); parts != nil {
		return parts[1] + " · " + parts[2]
	}
	rest := strings.TrimPrefix(name, mcpPrefix)
	if server := longestServer(rest, servers); server != "" {
		return server + " · " + rest[len(server)+1:]
	}
	return rest
}

// longestServer is the longest server rest spells as "<server>_" with a tool
// after it, "" for none.
func longestServer(rest string, servers []string) string {
	found := ""
	for _, server := range servers {
		if len(server) > len(found) && len(rest) > len(server)+1 && strings.HasPrefix(rest, server+"_") {
			found = server
		}
	}
	return found
}
