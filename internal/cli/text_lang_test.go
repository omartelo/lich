package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/prompt"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/spawn"
)

// Every text renders in every language with its arguments in place and the
// literals agents match on untouched.
func TestEveryTextRendersInEveryLanguage(t *testing.T) {
	opened := spawn.Session{Label: "docs", Name: "docs-1", Kind: "claude", Project: "p", Path: "/w", Confined: true}
	collected := relay.Collected{
		Results: []relay.Result{
			{Status: relay.StatusAnswered, Target: "a", Ticket: "t1", Answer: "ok"},
			{Status: relay.StatusUnread, Target: "b"},
			{Status: relay.StatusUnanswered, Target: "c"},
			{Status: relay.StatusUndelivered, Target: "d"},
			{Status: relay.StatusStopped, Target: "e"},
			{Status: relay.StatusExpired, Target: "f"},
		},
		Open: []string{"g"},
	}
	for _, lang := range prompt.Langs {
		texts := map[string]string{
			"instructions":  mcpInstructions(lang),
			"opened":        openedText(lang, opened),
			"opened plain":  openedText(lang, spawn.Session{Label: "docs", Project: "p"}),
			"removed":       closedText(lang, spawn.Closed{Label: "docs", Worktree: "/w", Removed: true}),
			"kept":          closedText(lang, spawn.Closed{Label: "docs", Worktree: "/w", Kept: true}),
			"closed":        closedText(lang, spawn.Closed{Label: "docs"}),
			"collected":     collectedText(lang, collected),
			"nothing":       collectedText(lang, relay.Collected{}),
			"private":       mcpOutcome(lang, relay.Result{Target: "docs", Ticket: "t1", Private: true}),
			"open":          mcpOutcome(lang, relay.Result{Target: "docs", Ticket: "t1"}),
			"unread":        unreadText(lang, "docs"),
			"undelivered":   undeliveredText(lang, "docs"),
			"unanswered":    unansweredText(lang, "docs"),
			"stopped":       stoppedText(lang, "docs"),
			"expired":       expiredText(lang, "docs"),
			"handover":      handOverFailure(lang, errors.New("boom"), relaySendPrivate),
			"handover open": handOverFailure(lang, errors.New("boom"), relaySend),
		}
		for name, text := range texts {
			if strings.Contains(text, "%!") {
				t.Errorf("%s %s: a format argument is off:\n%s", lang, name, text)
			}
		}
		for _, literal := range []string{
			"open_session", "send_to_session", "wait_for_answer", "close_session",
			"list_worktrees", "no_wait", "private", relay.ToolReply, "[lich]",
		} {
			if !strings.Contains(texts["instructions"], literal) {
				t.Errorf("%s: the instructions lost %q", lang, literal)
			}
		}
		for _, name := range []string{"private", "open"} {
			for _, literal := range []string{"wait_for_answer", `"t1"`} {
				if !strings.Contains(texts[name], literal) {
					t.Errorf("%s %s: lost %q:\n%s", lang, name, literal, texts[name])
				}
			}
		}
		for _, literal := range []string{`"docs"`, `"docs-1"`, "/w"} {
			if !strings.Contains(texts["opened"], literal) {
				t.Errorf("%s: the opened text lost %q", lang, literal)
			}
		}
		if !strings.Contains(texts["handover"], "send_to_session") || !strings.Contains(texts["handover"], "private") {
			t.Errorf("%s: the private hand over lost the tool call:\n%s", lang, texts["handover"])
		}
		for _, literal := range []string{"a", "t1", "ok", "b", "g"} {
			if !strings.Contains(texts["collected"], literal) {
				t.Errorf("%s: the drain lost %q", lang, literal)
			}
		}
	}
}

// The command line and the MCP server run outside lich, so the language comes
// from the variable lich exported when it spawned the session.
func TestTheClientReadsThePromptLanguageFromTheEnvironment(t *testing.T) {
	for tag, want := range map[string]prompt.Lang{
		"":      prompt.English,
		"en":    prompt.English,
		"pt-BR": prompt.PortugueseBR,
		"xx":    prompt.English,
	} {
		c := &client{env: func(key string) string {
			if key == prompt.EnvVar {
				return tag
			}
			return ""
		}}
		if got := c.lang(); got != want {
			t.Errorf("%s=%q: lang = %q, want %q", prompt.EnvVar, tag, got, want)
		}
	}
}

func TestTheHandshakeCarriesTheInstructionsInTheClientsLanguage(t *testing.T) {
	for _, lang := range prompt.Langs {
		got := mcpHandshake(nil, "test", lang)["instructions"]
		if got != mcpInstructions(lang) {
			t.Errorf("%s: handshake instructions differ from the catalog's", lang)
		}
	}
}
