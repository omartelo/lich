package lichclient

import "context"

// CloseSession parks a session: its row leaves the open list but stays for a
// later reopen, and activeID becomes the project's active session ("" for
// none). It does not stop the session's process; CloseTerminal does.
func (c *Client) CloseSession(ctx context.Context, projectID, sessionID, activeID string) error {
	return c.Call(ctx, "store.CloseSession", nil, projectID, sessionID, activeID)
}

// CloseTerminal stops a session's process and drops its PTY.
func (c *Client) CloseTerminal(ctx context.Context, id string) error {
	return c.Call(ctx, "terminal.Close", nil, id)
}
