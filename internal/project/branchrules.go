package project

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// BranchRules is what a repository's rulesets say about merging into one branch. GitHub's
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
	// CommitPatterns are the rules that read a commit's own text. Empty is the
	// common case and means only "no rule of this kind", never "nothing blocks".
	CommitPatterns []CommitPattern `json:"commitPatterns"`
	// ViewerCanBypass is whether this account administers the repository, and so
	// may merge past the rules above. It is the one answer here that must fail
	// *closed*: an unreadable one hides an action rather than offering one that
	// cannot work, because a bypass offered to an account that has none reads as
	// a broken button, not as a permission it never had.
	ViewerCanBypass bool `json:"viewerCanBypass"`
}

// CommitPattern is one ruleset rule that tests the text of a commit rather than
// the state of a pull request. It earns its own field because it is the one
// class of rule nothing on a pull request hints at: the checks pass, the review
// lands, no conflict exists, and GitHub still answers BLOCKED — because the
// message the merge *would* write breaks a pattern nobody can see from here.
type CommitPattern struct {
	// Target is the part the rule reads, already in the words the screen uses:
	// "message", "author email", "committer email".
	Target string `json:"target"`
	// Operator is GitHub's own: regex | starts_with | ends_with | contains.
	Operator string `json:"operator"`
	Pattern  string `json:"pattern"`
	// Negate flips the test — the pattern is then what the text must *not* be.
	Negate bool `json:"negate"`
}

// commitPatternTargets maps the rule types that test commit text onto the word
// the screen puts in the sentence. GitHub has pattern rules for branch and tag
// names too; they gate a push, never a merge, so they are left out.
var commitPatternTargets = map[string]string{
	"commit_message_pattern":      "message",
	"commit_author_email_pattern": "author email",
	"committer_email_pattern":     "committer email",
}

// ghBranchRule is one entry of the rules list; the pull_request rule and the
// commit pattern rules are all this reads.
type ghBranchRule struct {
	Type       string `json:"type"`
	Parameters struct {
		AllowedMergeMethods []string `json:"allowed_merge_methods"`
		Operator            string   `json:"operator"`
		Pattern             string   `json:"pattern"`
		Negate              bool     `json:"negate"`
	} `json:"parameters"`
}

// BranchRules reads the rules a repository's rulesets apply to one branch, and
// whether this account may merge past them.
//
// A branch nothing governs answers with an empty list, and so does a token that
// may not read the rules — both reach the screen as "no rule known", which is
// the same shape as a repository that has none. That is deliberate: this read
// exists to *narrow* what the merge menu offers, so failing to answer must
// leave the menu as wide as it was, never empty. The bypass is the exception
// and runs the other way; see ViewerCanBypass and viewerAdministers.
func (s *Service) BranchRules(path, branch string) (*BranchRules, error) {
	if branch == "" {
		return nil, fmt.Errorf("a branch is required")
	}
	out, err := s.gh(prReadTimeout, path, "api",
		"repos/{owner}/{repo}/rules/branches/"+url.PathEscape(branch))
	if err != nil {
		return nil, err
	}
	rules := parseBranchRules(out)
	rules.ViewerCanBypass = s.viewerAdministers(path)
	return rules, nil
}

// viewerAdministers reports whether the account gh uses for this checkout
// administers the repository — GitHub's own bar for the merge override.
//
// A second call, because nothing on a pull request or in the rules answers it.
// It costs one small read per base branch, which the screen caches, and buys
// the thing a click cannot: an action that is *absent* rather than offered and
// refused. Every failure — the call, the decode, a repository gh cannot see —
// is that same absence, never an error: the rules this rides along with are
// worth drawing on their own.
//
// Only ADMIN counts. A ruleset can name any role as a bypass actor, so an
// account that may bypass without administering the repository does not get
// the option here; reading each ruleset's bypass list to find out is the
// upgrade path.
func (s *Service) viewerAdministers(path string) bool {
	out, err := s.gh(prReadTimeout, path, "repo", "view", "--json", "viewerPermission")
	if err != nil {
		return false
	}
	var repo struct {
		ViewerPermission string `json:"viewerPermission"`
	}
	if err := json.Unmarshal(out, &repo); err != nil {
		return false
	}
	return repo.ViewerPermission == "ADMIN"
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
	parsed := &BranchRules{}
	for _, rule := range rules {
		if rule.Type == "pull_request" && len(rule.Parameters.AllowedMergeMethods) > 0 {
			parsed.AllowedMergeMethods = rule.Parameters.AllowedMergeMethods
		}
		// A pattern with nothing to test against explains nothing, so it is
		// dropped rather than shown as an empty requirement.
		if target, ok := commitPatternTargets[rule.Type]; ok && rule.Parameters.Pattern != "" {
			parsed.CommitPatterns = append(parsed.CommitPatterns, CommitPattern{
				Target:   target,
				Operator: rule.Parameters.Operator,
				Pattern:  rule.Parameters.Pattern,
				Negate:   rule.Parameters.Negate,
			})
		}
	}
	return parsed
}
