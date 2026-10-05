package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/semver"
)

// modAnswerRequest is a worker's final message, which its mod reports as the
// answer to its subagent errand (docs/hooks/mod-answer.md).
type modAnswerRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
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
	if strings.TrimSpace(req.Text) == "" {
		return modAnswerRequest{}, errors.New("mod answer text is blank")
	}
	return req, nil
}

// modAnswer hands a worker's report to the relay, which decides whether there
// is an errand for it to answer.
func (t *transport) modAnswer(w http.ResponseWriter, r *http.Request) {
	servePostLimited(t, w, r, modAckBodyLimit, parseModAnswer, func(req modAnswerRequest) error {
		t.mu.Lock()
		fn := t.workerAnswered
		t.mu.Unlock()
		if fn != nil {
			fn(req.SessionID, req.Text)
		}
		return nil
	})
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

// ModAnswers is whether running session id's mod answers a subagent errand
// with its turn's final message: it polls, from a plugin release that does.
func (s *Service) ModAnswers(id string) bool {
	return s.ModAttached(id) && !semver.Less(s.ws.plugins.of(id), agentplugin.ModAnswerRelease)
}
