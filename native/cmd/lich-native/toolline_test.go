package main

import "testing"

// Mirrors frontend/src/lib/session/tool-label.test.ts.
func TestToolLine(t *testing.T) {
	servers := []string{"lich", "ai-memory"}
	cases := []struct {
		name, detail      string
		servers           []string
		label, wantDetail string
	}{
		{"mcp__ai-memory__memory_write_page", "", nil, "ai-memory · memory_write_page", ""},
		{"mcp__lich__open_session", "", nil, "lich · open_session", ""},
		{"mcp__srv__do__the__thing", "", nil, "srv · do__the__thing", ""},
		{"mcp__lich_list_sessions", "", servers, "lich · list_sessions", ""},
		{"mcp__ai-memory_memory_query", "", servers, "ai-memory · memory_query", ""},
		{"lich_open_session", "", servers, "lich · open_session", ""},
		{"lich_relay_send", "", []string{"lich", "lich_relay"}, "lich_relay · send", ""},
		{"lichen_pick", "", servers, "lichen_pick", ""},
		{"lichprobe_list_sessions", "", servers, "lichprobe_list_sessions", ""},
		{"mcp__other_list_sessions", "", servers, "other_list_sessions", ""},
		{"mcp__lich_list_sessions", "", nil, "lich_list_sessions", ""},
		{"Bash", "", servers, "Bash", ""},
		{"apply_patch", "", servers, "apply_patch", ""},
		{"", "", servers, "", ""},
		{"mcp__lich", "", servers, "lich", ""},
		{"mcp____tool", "", servers, "__tool", ""},
		{"lich_", "", servers, "lich_", ""},
		{"Bash", "pnpm test", servers, "Bash", "pnpm test"},
		{"call_mcp_tool", "lich/open_session", servers, "lich · open_session", ""},
		{"call_mcp_tool", "", servers, "call_mcp_tool", ""},
		{"call_mcp_tool", "open_session", servers, "call_mcp_tool", "open_session"},
		{"call_mcp_tool", "lich/", servers, "call_mcp_tool", "lich/"},
	}
	for _, c := range cases {
		label, detail := toolLine(sessionTool{Name: c.name, Detail: c.detail}, c.servers)
		if label != c.label || detail != c.wantDetail {
			t.Errorf("toolLine(%q, %q) = %q, %q; want %q, %q", c.name, c.detail, label, detail, c.label, c.wantDetail)
		}
	}
}
