package project

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// What a repository's rulesets say about merging into one branch. GitHub's
// pull request fields do not carry it: `mergeStateStatus` answers whether this
// merge can happen, never which methods the branch would accept, so a Merge
// menu built without this offers buttons the ruleset refuses — and gh's
// refusal arrives after the click, as one flat sentence (gherror.go).
//
// The endpoint answers for a branch, not a pull request, which is why this is
// its own read: every pull request onto the same base shares one answer.
type BranchRules struct {
	// AllowedMergeMethods is squash|merge|rebase, in GitHub's own spelling.
	// Empty means the branch has no rule about it and all three stand — which
	// is also what a repository with no rulesets at all reports.
	AllowedMergeMethods []string `json:"allowedMergeMethods"`
}

// ghBranchRule is one entry of the rules list; only the pull_request rule
// carries anything this reads.
type ghBranchRule struct {
	Type       string `json:"type"`
	Parameters struct {
		AllowedMergeMethods []string `json:"allowed_merge_methods"`
	} `json:"parameters"`
}

// BranchRules reads the rules a repository's rulesets apply to one branch.
//
// A branch nothing governs answers with an empty list, and so does a token that
// may not read the rules — both reach the screen as "no rule known", which is
// the same shape as a repository that has none. That is deliberate: this read
// exists to *narrow* what the merge menu offers, so failing to answer must
// leave the menu as wide as it was, never empty.
func (s *Service) BranchRules(path, branch string) (*BranchRules, error) {
	if branch == "" {
		return nil, fmt.Errorf("a branch is required")
	}
	out, err := s.gh(prReadTimeout, path, "api",
		"repos/{owner}/{repo}/rules/branches/"+url.PathEscape(branch))
	if err != nil {
		return nil, err
	}
	return parseBranchRules(out), nil
}

// parseBranchRules keeps what the merge menu asks for. An answer this cannot
// read is not an error: GitHub returns a plain object rather than a list for a
// branch it will not report on, and a screen that refuses to draw its Merge
// menu over that is worse than one that draws the whole menu.
func parseBranchRules(out []byte) *BranchRules {
	var rules []ghBranchRule
	if err := json.Unmarshal(out, &rules); err != nil {
		return &BranchRules{}
	}
	for _, rule := range rules {
		if rule.Type == "pull_request" && len(rule.Parameters.AllowedMergeMethods) > 0 {
			return &BranchRules{AllowedMergeMethods: rule.Parameters.AllowedMergeMethods}
		}
	}
	return &BranchRules{}
}
