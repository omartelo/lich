package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/semver"
)

// modAnswerRequest is a worker's final message, which its mod reports as the
// answer to its subagent errand, or why its turn ended without one
// (docs/hooks/mod-answer.md). It carries exactly one of the two.
type modAnswerRequest struct {
	SessionID  string `json:"session_id"`
	Text       string `json:"text"`
	Unanswered string `json:"unanswered"`
}

func (r modAnswerRequest) session() string { return r.SessionID }

func parseModAnswer(body []byte) (modAnswerRequest, error) {
	var req modAnswerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return modAnswerRequest{}, fmt.Errorf("invalid mod answer body: %w", err)
	}
	if req.SessionID == "" {
		return modAnswerRequest{}, errors.New("mod answer missing session_id")
	}
	if (req.Text == "") == (req.Unanswered == "") {
		return modAnswerRequest{}, errors.New("mod answer carries exactly one of text and unanswered")
	}
	if req.Unanswered != "" && !relay.IsUnansweredReason(req.Unanswered) {
		return modAnswerRequest{}, fmt.Errorf("mod answer unanswered %q is not blank, error or refusal", req.Unanswered)
	}
	if req.Unanswered == "" && strings.TrimSpace(req.Text) == "" {
		return modAnswerRequest{}, errors.New("mod answer text is blank")
	}
	return req, nil
}

// modAnswer hands a worker's report to the relay, which decides whether there
// is an errand for it to answer or end.
func (t *transport) modAnswer(w http.ResponseWriter, r *http.Request) {
	servePostLimited(t, w, r, modAckBodyLimit, parseModAnswer, func(req modAnswerRequest) error {
		t.mu.Lock()
		answered, unanswered := t.workerAnswered, t.workerUnanswered
		t.mu.Unlock()
		if req.Unanswered != "" {
			if unanswered != nil {
				unanswered(req.SessionID, req.Unanswered)
			}
			return nil
		}
		if answered != nil {
			answered(req.SessionID, req.Text)
		}
		return nil
	})
}

func (t *transport) setWorkerUnanswered(fn func(id, reason string)) {
	t.mu.Lock()
	t.workerUnanswered = fn
	t.mu.Unlock()
}

func (t *transport) setWorkerAnswered(fn func(id, text string)) {
	t.mu.Lock()
	t.workerAnswered = fn
	t.mu.Unlock()
}

// SetWorkerAnswer wires where a worker's reported answer goes: the relay's
// WorkerAnswered. Startup wiring, called once before any session reports.
func (s *Service) SetWorkerAnswer(fn func(id, text string)) {
	if s.ws == nil {
		return
	}
	s.ws.setWorkerAnswered(fn)
}

// SetWorkerUnanswered wires where a worker's turn that ended without an answer
// goes: the relay's WorkerUnanswered. Startup wiring, like SetWorkerAnswer.
func (s *Service) SetWorkerUnanswered(fn func(id, reason string)) {
	if s.ws == nil {
		return
	}
	s.ws.setWorkerUnanswered(fn)
}

// ModAnswers is whether running session id's mod answers a subagent errand
// with its turn's final message: it polls, from a plugin release that does.
func (s *Service) ModAnswers(id string) bool {
	return s.ModAttached(id) && !semver.Less(s.ws.plugins.of(id), agentplugin.ModAnswerRelease)
}
