package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/singleton"
)

// call posts one RPC to the lich this session belongs to.
func (c *client) call(method string, args []any, timeout time.Duration, out any) error {
	port, token, err := c.coordinates()
	if err != nil {
		return err
	}
	body, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encode arguments: %w", err)
	}

	httpClient := &http.Client{Timeout: timeout}
	status, payload, err := post(httpClient, endpoint(port, token, method), body)
	if err != nil {
		return err
	}
	// A refused token is not this caller's mistake — the coordinates it was given
	// can have gone stale under it (see reissued). Retried once, on the same port,
	// with the token the running instance recorded.
	if status == http.StatusForbidden || status == http.StatusUnauthorized {
		token, err = c.reissued(port, token)
		if err != nil {
			return err
		}
		if status, payload, err = post(httpClient, endpoint(port, token, method), body); err != nil {
			return err
		}
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s", failureOf(payload, status))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode the reply: %w", err)
	}
	return nil
}

// post sends one RPC and reads the whole reply. The body is read here rather
// than returned open because a refused call is sent a second time, and the
// first response has to be finished with before the second one starts.
func post(httpClient *http.Client, endpoint string, body []byte) (int, []byte, error) {
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("reach lich: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read the reply: %w", err)
	}
	return resp.StatusCode, payload, nil
}

// endpoint builds the RPC URL for one call.
func endpoint(port, token, method string) string {
	return fmt.Sprintf(
		"http://127.0.0.1:%s/rpc/%s?token=%s",
		url.PathEscape(port), method, url.QueryEscape(token),
	)
}

// reissued answers a listener that refused this call's token, with the token the
// lich on that same port recorded — or with why there is none to try.
//
// The connect token is minted per launch and lives only in memory, so a lich
// closed and opened again answers on the pinned port with a token no older
// process can know. The coordinates in a PTY's environment are that older
// process's, and a caller that outlives the session it was started in keeps
// them: a background agent the harness parks in its own daemon, a nohup, a
// detached pane. Every call it makes 403s from then on, forever, with nothing
// on screen saying why — measured on a Claude Code background agent whose
// `lich mcp` outlived a restart of the lich that spawned it.
//
// Only the same port is retried. The runtime file names one instance, and a
// machine running a daily driver beside a `task dev` build has two: answering to
// the other one would put this message in a window the caller cannot see, which
// is the rule coordinates() follows too.
func (c *client) reissued(port, refused string) (string, error) {
	stale := fmt.Sprintf(
		"lich refused this token on port %s: the coordinates in this environment "+
			"were exported by an earlier lich, and the one running now cannot be "+
			"reached with them", port,
	)
	if c.running == nil {
		return "", fmt.Errorf("%s. Run this from a session of the running lich", stale)
	}
	info, err := c.running()
	if err != nil {
		return "", fmt.Errorf("%s, and the running instance could not be read: %w", stale, err)
	}
	if info == nil || info.Token == "" {
		return "", fmt.Errorf("%s, and no running instance is recorded to ask instead", stale)
	}
	if strconv.Itoa(info.Port) != port {
		return "", fmt.Errorf(
			"%s. The lich recorded on this machine listens on port %d, and a message "+
				"sent there would land in a window this caller cannot see — start it "+
				"again from a session of that lich",
			stale, info.Port,
		)
	}
	if info.Token == refused {
		return "", fmt.Errorf(
			"lich refused this token on port %s, and it is the token the running "+
				"instance recorded — so something other than lich is answering there",
			port,
		)
	}
	return info.Token, nil
}

// coordinates finds the lich to talk to: the one that spawned this PTY when
// there is one, otherwise the instance running on this machine.
//
// The environment comes first on purpose. A session belongs to the lich that
// spawned it, and on a machine running a daily driver beside a `task dev`
// build, the runtime file names only one of them — answering to the wrong lich
// would put a message in a window the caller cannot see.
func (c *client) coordinates() (string, string, error) {
	if port, token := c.env("LICH_PORT"), c.env("LICH_TOKEN"); port != "" && token != "" {
		return port, token, nil
	}
	if c.running == nil {
		return "", "", fmt.Errorf("no lich is running")
	}
	info, err := c.running()
	if err != nil {
		return "", "", err
	}
	if info == nil || info.Port == 0 || info.Token == "" {
		return "", "", fmt.Errorf("no lich is running — open lich, or run this inside one of its sessions")
	}
	return strconv.Itoa(info.Port), info.Token, nil
}

// runningLich reads the loopback coordinates of the lich running on this
// machine. It is the same runtime file install.sh reads to reach a running lich
// from outside a session, and LICH_DEV selects the dev instance's own file just
// as it does everywhere else.
func runningLich() (*singleton.Info, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config directory: %w", err)
	}
	return singleton.Read(dir)
}

// sessionID is which card this command runs in, empty for a caller that has no
// session — a script, a scheduled job, a plain shell. The relay words the
// message it delivers differently for those, so an empty sender is a fact to
// pass on rather than an error.
func (c *client) sessionID() string {
	return c.env("LICH_SESSION_ID")
}

// failureOf unwraps the RPC's {"error": "..."} envelope, falling back to the
// HTTP status for a body that is not one.
func failureOf(payload []byte, status int) string {
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && envelope.Error != "" {
		return envelope.Error
	}
	return fmt.Sprintf("%d %s", status, http.StatusText(status))
}

// waitBudget is how long the client gives a call that blocks on another
// session: the wait it asked for, plus room for the answer's trip back. A zero
// timeout means the relay's own default.
//
// It is capped at the relay's own bound, and capped in seconds rather than in
// Duration for the reason relay.waitFor gives: a number past about 9.2e9
// overflows into a negative Duration, which http.Client reads as a deadline
// already past — the POST fails on the spot and reports the send as unreachable
// while the task it carried is being delivered.
func waitBudget(seconds int) time.Duration {
	if seconds <= 0 {
		return relay.DefaultWait + callSlack
	}
	if seconds > relay.MaxWaitSeconds {
		seconds = relay.MaxWaitSeconds
	}
	return time.Duration(seconds)*time.Second + callSlack
}
