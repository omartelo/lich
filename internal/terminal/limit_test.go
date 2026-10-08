package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claudeLimitLine is the synthetic assistant line Claude Code 2.1.293 wrote when
// a session limit ended a turn, trimmed to the fields that matter from a real
// transcript.
const claudeLimitLine = `{"type":"assistant","isSidechain":false,"message":{"model":"<synthetic>","role":"assistant","content":[{"type":"text","text":"You've hit your session limit · resets 9:10pm (America/Sao_Paulo)"}]},"quotaLimits":{"status":"rejected","resetsAt":1788653400,"rateLimitType":"five_hour","overageStatus":"rejected"},"error":"rate_limit","isApiErrorMessage":true,"apiErrorStatus":429}`

func TestClaudeUsageLimitIsReadOffTheLastAssistantLine(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  usageLimit
		found bool
	}{
		{
			name:  "a session limit ends the turn",
			lines: []string{`{"type":"user","message":{"content":"go on"}}`, claudeLimitLine},
			want:  usageLimit{window: limitWindowSession, resetsAt: 1788653400},
			found: true,
		},
		{
			name:  "a weekly limit",
			lines: []string{strings.Replace(claudeLimitLine, `"five_hour"`, `"seven_day"`, 1)},
			want:  usageLimit{window: limitWindowWeekly, resetsAt: 1788653400},
			found: true,
		},
		{
			name:  "a window the card has no name for",
			lines: []string{strings.Replace(claudeLimitLine, `"five_hour"`, `"overage"`, 1)},
			want:  usageLimit{resetsAt: 1788653400},
			found: true,
		},
		{
			name: "the conversation went on after the limit",
			lines: []string{
				claudeLimitLine,
				`{"type":"user","message":{"content":"continue"}}`,
				`{"type":"assistant","message":{"content":[{"type":"text","text":"Picking up."}]}}`,
			},
		},
		{
			name: "a sub-agent's line after the limit is not the session's",
			lines: []string{
				claudeLimitLine,
				`{"type":"assistant","isSidechain":true,"message":{"content":[{"type":"text","text":"sub"}]}}`,
			},
			want:  usageLimit{window: limitWindowSession, resetsAt: 1788653400},
			found: true,
		},
		{
			name:  "another API error is no limit",
			lines: []string{strings.Replace(claudeLimitLine, `"error":"rate_limit"`, `"error":"overloaded"`, 1)},
		},
		{
			name:  "no assistant line at all",
			lines: []string{`{"type":"user","message":{"content":"hi"}}`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := parseClaudeUsageLimit([]byte(strings.Join(tc.lines, "\n") + "\n"))
			if found != tc.found || got != tc.want {
				t.Fatalf("parseClaudeUsageLimit = %+v, %v; want %+v, %v", got, found, tc.want, tc.found)
			}
		})
	}
}

// The rollout lines a Codex turn wrote when its server answered
// usage_limit_reached (0.144.5, measured against a fake responses endpoint),
// with the payloads cut to the fields the reader looks at.
const (
	codexTaskStarted   = `{"type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}`
	codexUserMessage   = `{"type":"event_msg","payload":{"type":"user_message","message":"say hi"}}`
	codexLimitTokens   = `{"type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"limit_id":"codex","primary":{"used_percent":100.0,"window_minutes":300,"resets_at":1791432079},"secondary":null}}}`
	codexSilentFinish  = `{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":null}}`
	codexSpokenFinish  = `{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1","last_agent_message":"Done."}}`
	codexHealthyTokens = `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":10}},"rate_limits":{"primary":{"used_percent":26.0,"window_minutes":10080,"resets_at":1787629462}}}}`
)

func TestCodexUsageLimitIsReadOffTheTurnsEnd(t *testing.T) {
	weeklyToo := strings.Replace(codexLimitTokens, `"secondary":null`,
		`"secondary":{"used_percent":100.0,"window_minutes":10080,"resets_at":1791900000}`, 1)
	tests := []struct {
		name  string
		lines []string
		want  usageLimit
		found bool
	}{
		{
			name:  "the measured turn",
			lines: []string{codexTaskStarted, codexUserMessage, codexLimitTokens, codexSilentFinish},
			want:  usageLimit{window: limitWindowSession, resetsAt: 1791432079},
			found: true,
		},
		{
			name:  "both windows spent wait for the later reset",
			lines: []string{codexTaskStarted, weeklyToo, codexSilentFinish},
			want:  usageLimit{window: limitWindowWeekly, resetsAt: 1791900000},
			found: true,
		},
		{
			name:  "a turn that said something finished",
			lines: []string{codexTaskStarted, codexLimitTokens, codexSpokenFinish},
		},
		{
			name:  "a silent finish with room left in the plan",
			lines: []string{codexTaskStarted, codexHealthyTokens, codexSilentFinish},
		},
		{
			name:  "a turn still running",
			lines: []string{codexTaskStarted, codexLimitTokens},
		},
		{
			name:  "a silent finish with no token count in its turn",
			lines: []string{codexLimitTokens, codexSilentFinish, codexTaskStarted, codexSilentFinish},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rollout.jsonl")
			if err := os.WriteFile(path, []byte(strings.Join(tc.lines, "\n")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			got, found := scanCodexUsageLimit(path)
			if found != tc.found || got != tc.want {
				t.Fatalf("scanCodexUsageLimit = %+v, %v; want %+v, %v", got, found, tc.want, tc.found)
			}
		})
	}
}

func TestTurnLogNamesEveryOpenTurn(t *testing.T) {
	var l turnLog
	l.report("a", statusBusy)
	l.report("b", statusBusy)
	l.report("b", statusDone)
	if got := l.openIDs(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("openIDs = %v, want [a]", got)
	}
}
