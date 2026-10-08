package relay

import (
	"fmt"
	"strings"
)

// Prompt sends the user's own words to a session as its next prompt, the way a
// scheduled prompt arrives: through the mod where one polls, else pasted at the
// prompt and sent (deliver), with no sender, ticket or envelope around it. The
// window calls it for the task of a race, one session per agent, once that
// session's prompt is up; it still waits for a free prompt, bounded like every
// other delivery, so a call made early holds its request open until then.
func (s *Service) Prompt(sessionID, text string) error {
	text = sanitize(text)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("nothing to send: the prompt is empty")
	}
	if len(text) > promptLimit {
		return fmt.Errorf("prompt is %d bytes, over the %d limit", len(text), promptLimit)
	}
	return s.deliver(sessionID, text, nil)
}
