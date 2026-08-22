package system

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/sandbox"
)

// agentListTimeout caps the call to ssh-add. Listing identities is a round trip
// to a socket on the same machine; an agent that has not answered in a second is
// one the setting can describe as empty rather than one worth blocking the
// settings pane on.
const agentListTimeout = time.Second

// SSHAgentKeys lists the identities loaded in the user's ssh agent, one line
// each ("me@example.com (ED25519 256)"), newest gh has none first.
//
// It exists for one sentence in the UI. The setting that hands a confined
// session the ssh agent is read as "let it push with my GitHub key", and what it
// actually hands over is every identity in the agent, usable against any host —
// so the control shows the list instead of asking the user to remember it. A
// choice made without seeing this is a choice about a different thing.
//
// Empty for everything that is not a list of keys: no agent in the environment,
// no ssh-add on the machine, an agent holding nothing, output this cannot read.
// The caller draws "nothing loaded", which is the truthful reading of all of
// them — the flag hands over whatever is there, and nothing is a real answer.
func (s *Service) SSHAgentKeys() []string {
	if sandbox.AgentSocket() == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), agentListTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh-add", "-l").Output()
	if err != nil {
		return nil
	}
	return parseAgentKeys(string(out))
}

// parseAgentKeys reads `ssh-add -l` output: "<bits> <fingerprint> <comment>
// (<TYPE>)", one identity per line. The fingerprint is dropped — it identifies a
// key to a machine, and the comment is what identifies it to the person who
// loaded it — and the bits move in beside the type.
//
// A line that does not have all four parts is kept whole rather than skipped: an
// identity the parser cannot read is still an identity the setting hands over,
// and a shorter list here would understate what the flag opens.
func parseAgentKeys(out string) []string {
	var keys []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "The agent has no identities") {
			continue
		}
		keys = append(keys, formatAgentKey(line))
	}
	return keys
}

// formatAgentKey turns one ssh-add line into the phrase the control shows, or
// returns it unchanged when it is not shaped like one.
func formatAgentKey(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 || !strings.HasPrefix(fields[len(fields)-1], "(") {
		return line
	}
	kind := strings.Trim(fields[len(fields)-1], "()")
	comment := strings.Join(fields[2:len(fields)-1], " ")
	return comment + " (" + kind + " " + fields[0] + ")"
}
