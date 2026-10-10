package relay

import (
	"context"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/prompt"
)

// The briefing is one argv entry that cmd.exe reads on Windows (see
// SpawnBriefing): a translation that brings a double quote or an angle bracket
// in breaks every Windows spawn of a .cmd provider in that language.
func TestSpawnBriefingIsCmdSafeInEveryLanguage(t *testing.T) {
	for _, lang := range prompt.Langs {
		for _, route := range []SubagentRoute{RouteSessions, RouteCards, RouteNative} {
			for _, hasTools := range []bool{true, false} {
				briefing := SpawnBriefing(lang, hasTools, route)
				if strings.ContainsAny(briefing, `"<>`) {
					t.Errorf("%s route %d tools %v carries a cmd.exe metacharacter:\n%s", lang, route, hasTools, briefing)
				}
			}
		}
	}
}

// Every message renders in every language with its arguments in place: a
// format that drops or misnumbers one shows as %!, which an agent would read as
// part of the instruction.
func TestEveryMessageRendersInEveryLanguage(t *testing.T) {
	for _, lang := range prompt.Langs {
		messages := []string{
			compose(lang, "", "t1", "task", false),
			compose(lang, "sender", "t1", "task", true),
			composeForWorker(lang, "sender", "task"),
			pickTicketNudge(lang, 2, "  t1, t2"),
			nudgeNotice(lang, 1, []string{"docs"}, false),
			nudgeNotice(lang, 3, []string{"docs", "api"}, true),
		}
		for _, message := range messages {
			if strings.Contains(message, "%!") {
				t.Errorf("%s: a format argument is off:\n%s", lang, message)
			}
			if !strings.HasPrefix(message, "[lich] ") {
				t.Errorf("%s: the [lich] token is missing:\n%s", lang, message)
			}
		}
		for _, literal := range []string{`"$LICH_BIN" reply t1`, "`" + ToolReply + "`", "t1"} {
			if !strings.Contains(compose(lang, "", "t1", "task", true), literal) {
				t.Errorf("%s: the reply instruction lost %q", lang, literal)
			}
		}
	}
}

// The language is read when a message is composed, not when the relay starts,
// so a change in Settings reaches the next message.
func TestTheRelayTypesInThePromptLanguageReadLive(t *testing.T) {
	term := newFakeTerminal("s2")
	svc := newRelay(workspace(), term, nil)
	lang := prompt.English
	svc.SetPromptLanguage(func() prompt.Lang { return lang })
	lang = prompt.PortugueseBR

	go func() { _ = svc.Reply("", waitForTicket(svc), "ok") }()
	if _, err := svc.Send(context.Background(), SendOptions{Target: "docs", Prompt: "hello", WaitSeconds: 30}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	typed := term.written("s2")
	if !strings.Contains(typed, prompt.For(prompt.PortugueseBR).OriginCLI) {
		t.Errorf("the message was not typed in the language set after startup:\n%s", typed)
	}
}
