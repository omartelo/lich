package lichclient

import "context"

// BaseStatus mirrors project.BaseStatus: where a checkout stands against the
// branch it merges into.
type BaseStatus struct {
	Base      string   `json:"base"`
	Behind    int      `json:"behind"`
	Conflicts []string `json:"conflicts"`
}

// PullRequest mirrors project.PullRequest.
type PullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// BaseStatus returns where path stands against its base, nil when it has no
// base to stand against (the base branch itself, no remote).
func (c *Client) BaseStatus(ctx context.Context, path string) (*BaseStatus, error) {
	var b *BaseStatus
	return b, c.Call(ctx, "project.BaseStatus", &b, path)
}

// PullRequest returns the open pull request of path's branch through gh, nil
// when there is none or gh cannot answer. Each call is a network round trip.
func (c *Client) PullRequest(ctx context.Context, path string) (*PullRequest, error) {
	var pr *PullRequest
	return pr, c.Call(ctx, "project.PullRequest", &pr, path)
}

// OpenExternal opens url in the user's browser.
func (c *Client) OpenExternal(ctx context.Context, url string) error {
	return c.Call(ctx, "system.OpenExternal", nil, url)
}
