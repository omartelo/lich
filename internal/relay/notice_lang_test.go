package relay

import (
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/prompt"
)

// resumePrompt is the English continuation text, which existing rows and tests
// were written against.
var resumePrompt = prompt.For(prompt.English).ResumePrompt

func other(lang prompt.Lang) prompt.Lang {
	if lang == prompt.English {
		return prompt.PortugueseBR
	}
	return prompt.English
}

// Every notice renders in every language with its arguments in place and the
// [lich] token its readers match on.
func TestEveryNoticeRendersInEveryLanguage(t *testing.T) {
	for _, lang := range prompt.Langs {
		entry := &inboxEntry{target: "docs", ticket: "t1", answer: "all done"}
		notices := map[string]string{
			"report":         subagentReport(lang, entry, "feat/x"),
			"report no tree": subagentReport(lang, entry, ""),
			"blocked":        blockedNotice(lang, "docs"),
			"merge":          mergeNotice(lang, 42, "Title", "feat/x", "main"),
			"late":           lateNotice(lang, 1000, 1000+3600),
			"resume":         prompt.For(lang).ResumePrompt,
		}
		for name, notice := range notices {
			if strings.Contains(notice, "%!") {
				t.Errorf("%s %s: a format argument is off:\n%s", lang, name, notice)
			}
			if !strings.HasPrefix(notice, "[lich] ") {
				t.Errorf("%s %s: the [lich] token is missing:\n%s", lang, name, notice)
			}
		}
		for _, literal := range []string{`"docs"`, "t1", "all done", "feat/x"} {
			if !strings.Contains(notices["report"], literal) {
				t.Errorf("%s: the report lost %q", lang, literal)
			}
		}
		if strings.Contains(notices["report no tree"], "feat/x") {
			t.Errorf("%s: the report names a branch it was not given", lang)
		}
		for _, literal := range []string{"#42", `"Title"`, "feat/x", "main"} {
			if !strings.Contains(notices["merge"], literal) {
				t.Errorf("%s: the merge notice lost %q", lang, literal)
			}
		}
		summaries := []string{
			reportNotification(lang, []*inboxEntry{{target: "docs"}}).Summary,
			reportNotification(lang, []*inboxEntry{{target: "docs"}, {target: "api"}}).Summary,
			blockedNotification(lang, "docs").Summary,
		}
		for _, summary := range summaries {
			if strings.Contains(summary, "%!") || !strings.Contains(summary, `"docs"`) {
				t.Errorf("%s: summary %q is off", lang, summary)
			}
		}
	}
}

// A parked continuation is stored as text, so a row written in one language
// must still be recognised, and dropped, once the setting is another.
func TestAParkedContinuationIsRecognisedInEveryLanguage(t *testing.T) {
	for _, stored := range prompt.Langs {
		for _, setting := range prompt.Langs {
			writer := &parkWriter{}
			row := prompt.For(stored).ResumePrompt
			svc := newRelay(scheduledWith(5000, row, writer.set), newFakeTerminal("s1"), &fakeEvents{})
			svc.SetPromptLanguage(func() prompt.Lang { return setting })
			svc.now = at(1000)

			svc.dropResume("s1")

			if got := writer.all(); len(got) != 1 || got[0] != (ScheduleEvent{ID: "s1"}) {
				t.Errorf("stored %s, setting %s: writes = %+v, want the row cleared", stored, setting, got)
			}
		}
	}
}

func TestAPersonsPromptIsNotTakenForTheContinuationInAnyLanguage(t *testing.T) {
	for _, setting := range prompt.Langs {
		writer := &parkWriter{}
		svc := newRelay(scheduledWith(5000, "ship the release notes", writer.set), newFakeTerminal("s1"), &fakeEvents{})
		svc.SetPromptLanguage(func() prompt.Lang { return setting })

		svc.dropResume("s1")

		if got := writer.all(); len(got) != 0 {
			t.Errorf("setting %s: writes = %+v, want the person's prompt kept", setting, got)
		}
	}
}

func TestTheContinuationIsParkedInThePromptLanguage(t *testing.T) {
	for _, lang := range prompt.Langs {
		writer := &parkWriter{}
		svc := newRelay(scheduledWith(0, "", writer.set), newFakeTerminal("s1"), &fakeEvents{})
		svc.SetPromptLanguage(func() prompt.Lang { return lang })
		svc.now = at(1000)

		svc.ParkResume("s1", 1000+3600)

		got := writer.all()
		if len(got) != 1 || got[0].Prompt != prompt.For(lang).ResumePrompt {
			t.Errorf("%s: writes = %+v, want the continuation in that language", lang, got)
		}
		if got[0].Prompt == prompt.For(other(lang)).ResumePrompt {
			t.Errorf("%s: both locales share one continuation text, so rows could not be told apart", lang)
		}
	}
}
