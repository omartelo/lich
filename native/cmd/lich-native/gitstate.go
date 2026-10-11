package main

import (
	"context"
	"log"

	"github.com/omartelo/lich/native/lichclient"
)

// baseStanding is the part of base-status.ts baseReadout a card draws: how
// many files would conflict with the base, else how many commits it is
// behind. The zero value draws nothing.
type baseStanding struct {
	conflict bool
	count    int
}

func standingOf(b *lichclient.BaseStatus) baseStanding {
	if b == nil {
		return baseStanding{}
	}
	if n := len(b.Conflicts); n > 0 {
		return baseStanding{conflict: true, count: n}
	}
	return baseStanding{count: b.Behind}
}

// prLookup is a path's pull request as asked for at a branch and head. The
// pr survives a new commit on the same branch, so the badge does not blink,
// and is dropped with a new branch (use-pull-request.ts resetOn).
type prLookup struct {
	branch, head string
	pr           *lichclient.PullRequest
}

// refreshPath re-reads a shown path's changes and base standing, and asks gh
// for its pull request again when the branch or head moved: a PR the session
// just opened, or a merge that closed one, shows without waiting.
func (m *model) refreshPath(ctx context.Context, path string) {
	d, err := m.client.Diff(ctx, path)
	if err != nil {
		return
	}
	b, err := m.client.BaseStatus(ctx, path)
	if err != nil {
		return
	}
	base := standingOf(b)
	m.mu.Lock()
	changed := m.git[path] != d || m.base[path] != base
	m.git[path], m.base[path] = d, base
	prev, looked := m.prs[path]
	ask := !looked || prev.branch != d.Branch || prev.head != d.Head
	if ask {
		next := prLookup{branch: d.Branch, head: d.Head}
		if prev.branch == d.Branch {
			next.pr = prev.pr
		}
		changed = changed || next.pr != prev.pr
		m.prs[path] = next
	}
	m.mu.Unlock()
	if ask {
		go m.lookupPR(ctx, path, d.Branch, d.Head)
	}
	if changed {
		m.invalidate()
	}
}

// lookupPR stores gh's answer unless the checkout moved on while it was
// asked. A failed lookup (no gh, not a GitHub repo) reads as no PR.
func (m *model) lookupPR(ctx context.Context, path, branch, head string) {
	pr, err := m.client.PullRequest(ctx, path)
	if err != nil {
		log.Printf("pull request of %s: %v", path, err)
		pr = nil
	}
	m.mu.Lock()
	cur := m.prs[path]
	current := cur.branch == branch && cur.head == head
	if current {
		cur.pr = pr
		m.prs[path] = cur
	}
	m.mu.Unlock()
	if current {
		m.invalidate()
	}
}

// stalePullRequests makes the next git tick ask gh again for every path,
// keeping the badges up meanwhile: a PR opened or merged in the browser
// shows when the user comes back. Callers hold m.mu.
func (m *model) stalePullRequests() {
	for path, l := range m.prs {
		l.head = ""
		m.prs[path] = l
	}
}
