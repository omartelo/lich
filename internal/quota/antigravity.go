package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/providers"
)

// Antigravity keeps its Google credential in the OS keyring and mints its own
// access token, so no endpoint can be asked with a token lich reads. Its CLI
// answers a `/usage` slash command in print mode instead. Measured on agy 1.2.5:
// the reply carries num_turns 0, every token counter 0 and an empty
// conversation id, so the reading spends no quota and starts no conversation.
//
// Do not add --disable-slash-commands: it sends `/usage` to the model as an
// ordinary prompt, which starts a billed turn (the Orca project measured it on
// agy 1.2.11).
var agyUsageArgs = []string{"-p", "/usage", "--output-format", "json", "--print-timeout", "20s"}

const (
	// agyMinUsageVersion is the first agy whose print mode runs `/usage` as a
	// command; older ones answer it as a prompt and spend a turn (agy's own
	// changelog, as cited by the Orca project).
	agyMinUsageVersion = "1.1.11"
	// agyTimeout bounds one agy run. A warm start answers in 2-3 seconds; a cold
	// one also pays the binary's self-check. agyUsageArgs gives agy 20s to exit
	// cleanly on its own first.
	agyTimeout = 30 * time.Second
	// agyWaitDelay bounds the wait for agy's output pipe once it is killed: a
	// child it left behind holding the pipe must not hold the cache lock.
	agyWaitDelay = 2 * time.Second
)

// agySignedOut is what agy prints, in any case, when there is no login to ask
// with. Carried over from the Orca project's classifier: a signed-out agy has
// not been measured here, since that means logging the user out.
var agySignedOut = []string{
	"unauthenticated",
	"not logged into antigravity",
	"not logged in",
	"not signed in",
	"not authenticated",
	"run agy login",
	"please sign in",
	"please log in",
	"sign in to antigravity",
	"no credentials",
	"authentication required",
}

// agyReply is one line of agy's print-mode JSON envelope. command.data is the
// contract; `response` is a lossy tab-joined rendering of it.
type agyReply struct {
	Status         string `json:"status"`
	ConversationID string `json:"conversation_id"`
	NumTurns       int    `json:"num_turns"`
	Command        *struct {
		Name string `json:"name"`
		Data struct {
			Groups []agyGroup `json:"groups"`
		} `json:"data"`
	} `json:"command"`
}

// agyGroup is a pool of models sharing one limit ("Gemini Models", "Claude and
// GPT models"), metered by a weekly bucket and, on some tiers, a 5h one.
type agyGroup struct {
	Name    string      `json:"name"`
	Buckets []agyBucket `json:"buckets"`
}

type agyBucket struct {
	Name              string   `json:"name"`
	Window            string   `json:"window"`
	RemainingFraction *float64 `json:"remaining_fraction"`
	ResetTime         string   `json:"reset_time"`
	// Disabled is a bucket the tier does not meter at all; drawn, it would read
	// as headroom the account does not have.
	Disabled bool `json:"disabled"`
}

// antigravityPlan asks agy for the quota of the login it holds. A machine with
// no agy has no Antigravity plan to show, and is left out.
func (s *Service) antigravityPlan(a Account) Plan {
	if s.runAgy == nil {
		return noPlan
	}
	p := plan(providers.Antigravity)
	if a.hidden() {
		return unknown(p)
	}
	if s.agyTurns {
		return failed(p)
	}
	env := a.environ()
	version, err := s.runAgy(env, "--version")
	if errors.Is(err, exec.ErrNotFound) {
		return noPlan
	}
	if err != nil || !versionAtLeast(strings.TrimSpace(string(version)), agyMinUsageVersion) {
		return failed(p)
	}
	// A non-zero exit still prints the envelope or the error worth classifying.
	out, err := s.runAgy(env, agyUsageArgs...)
	if errors.Is(err, context.DeadlineExceeded) {
		return failed(p)
	}
	reply, ok := agyUsage(string(out))
	switch {
	case ok:
		p.Windows = reply.windows()
		if len(p.Windows) == 0 {
			return failed(p)
		}
		return p
	case reply.NumTurns > 0 || reply.ConversationID != "":
		s.agyTurns = true
		return failed(p)
	case agyLoggedOut(string(out)):
		return signedOut(p)
	default:
		return failed(p)
	}
}

// agyUsage finds the usage reply among agy's output lines, which may carry log
// noise. The bool is false when no line is a successful usage reply, and the
// reply returned is then the last envelope seen, for the turn check.
func agyUsage(out string) (agyReply, bool) {
	var last agyReply
	for line := range strings.Lines(out) {
		line = strings.TrimSpace(line)
		var reply agyReply
		if !strings.HasPrefix(line, "{") || json.Unmarshal([]byte(line), &reply) != nil {
			continue
		}
		if reply.Status == "SUCCESS" && reply.Command != nil && reply.Command.Name == "usage" {
			return reply, true
		}
		last = reply
	}
	return last, false
}

func agyLoggedOut(out string) bool {
	lower := strings.ToLower(out)
	for _, phrase := range agySignedOut {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// windows draws every metered bucket, named by its group: every bucket agy
// reports is called "Weekly Limit Remaining", so the group is what tells two
// rows apart. The bucket name is appended only inside a group with two.
func (r agyReply) windows() []Window {
	var out []Window
	for _, group := range r.Command.Data.Groups {
		buckets := make([]agyBucket, 0, len(group.Buckets))
		for _, b := range group.Buckets {
			if !b.Disabled && b.RemainingFraction != nil {
				buckets = append(buckets, b)
			}
		}
		for _, b := range buckets {
			label := group.Name
			if len(buckets) > 1 {
				label += " · " + b.Name
			}
			out = append(out, Window{
				Label:    label,
				Seconds:  agyWindowSeconds(b.Window),
				Percent:  percent((1 - *b.RemainingFraction) * 100),
				ResetsAt: b.ResetTime,
			})
		}
	}
	return out
}

// agyWindowSeconds maps agy's window names onto lengths. An unknown name is
// drawn without one rather than forced into a window it may not be.
func agyWindowSeconds(window string) int {
	switch window {
	case "weekly":
		return weeklyWindow
	case "5h":
		return sessionWindow
	default:
		return 0
	}
}

// versionAtLeast compares two dotted major.minor.patch versions; false for a
// version it cannot read, since an unreadable agy may be one that spends a turn.
func versionAtLeast(version, floor string) bool {
	var have, want [3]int
	if _, err := fmt.Sscanf(version, "%d.%d.%d", &have[0], &have[1], &have[2]); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(floor, "%d.%d.%d", &want[0], &want[1], &want[2]); err != nil {
		return false
	}
	for i := range have {
		if have[i] != want[i] {
			return have[i] > want[i]
		}
	}
	return true
}

// runAgy runs the agy on lich's PATH, never a configured launch command: that
// may be a wrapper or carry flags of its own, and `-p /usage` appended to it
// could reach the wrong program or the wrong argv slot.
func runAgy(env []string, args ...string) ([]byte, error) {
	path, err := exec.LookPath(providers.DefaultBinary(providers.Antigravity))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), agyTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = env
	cmd.WaitDelay = agyWaitDelay
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	return out, err
}

// environ is the account's environment as a process takes it; nil, which
// inherits lich's own, for the machine-wide reading.
func (a Account) environ() []string {
	if a.Env == nil {
		return nil
	}
	env := make([]string, 0, len(a.Env))
	for k, v := range a.Env {
		env = append(env, k+"="+v)
	}
	return env
}
