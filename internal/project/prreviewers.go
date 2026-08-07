package project

import (
	"encoding/json"
	"fmt"
)

// prAssignableQuery lists the accounts that may be asked for a review — GitHub's
// assignableUsers, the same list gh's own `pr edit` prompt offers, and one a
// read-only account can still see (the collaborators endpoint needs push).
//
// Ceiling: the first 100, unpaged. A larger organisation keeps a tail the picker
// cannot reach, and github.com is the way to those until somebody hits it.
const prAssignableQuery = `query($owner:String!,$repo:String!){
  repository(owner:$owner,name:$repo){
    assignableUsers(first:100){nodes{login name}}
  }
}`

// PRReviewer is one entry of a pull request's review roster: somebody who was
// asked and has not answered yet, or somebody who has.
type PRReviewer struct {
	// Login is the account's login, or a team's slug — which is not what gh takes
	// to add or remove one (that is org/slug), so a team is shown and not edited.
	Login  string `json:"login"`
	IsTeam bool   `json:"isTeam"`
	// State is "" while the review is still owed, else gh's APPROVED |
	// CHANGES_REQUESTED | COMMENTED | DISMISSED.
	State string `json:"state"`
}

// PRCandidate is one account the reviewer picker may offer.
type PRCandidate struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// ghReviewRequest is one entry of gh's reviewRequests: a User carries a login, a
// Team a slug, and __typename is the only thing that says which.
type ghReviewRequest struct {
	TypeName string `json:"__typename"`
	Login    string `json:"login"`
	Slug     string `json:"slug"`
}

// ghLatestReview is one entry of gh's latestReviews — the last review each
// person left.
type ghLatestReview struct {
	Author ghPRAuthor `json:"author"`
	State  string     `json:"state"`
}

// toReviewers merges gh's two halves of the roster into one list. They have to
// be read together: a submitted review clears its request, so the pending half
// alone reads as "nobody is reviewing this" the moment the first review lands,
// and the answered half alone hides everyone still owing one.
func toReviewers(reviews []ghLatestReview, requests []ghReviewRequest) []PRReviewer {
	out := make([]PRReviewer, 0, len(reviews)+len(requests))
	at := make(map[string]int, len(reviews))
	for _, r := range reviews {
		login := firstNonEmpty(r.Author.Login, r.Author.Name)
		if login == "" {
			continue
		}
		at[login] = len(out)
		out = append(out, PRReviewer{Login: login, State: r.State})
	}
	for _, req := range requests {
		login := firstNonEmpty(req.Login, req.Slug)
		if login == "" {
			continue
		}
		// Asked again after reviewing: GitHub keeps the old verdict in
		// latestReviews and re-files the request, and what stands now is the
		// answer owed, not the one already given.
		if i, ok := at[login]; ok {
			out[i].State = ""
			continue
		}
		out = append(out, PRReviewer{Login: login, IsTeam: req.TypeName == "Team"})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// AssignableReviewers lists who this repository allows on a review, so the
// picker offers accounts GitHub will accept rather than a typed login it would
// refuse. Returns nil for a repository that reports none.
func (s *Service) AssignableReviewers(path string) ([]PRCandidate, error) {
	out, err := s.gh(prReadTimeout, path, "api", "graphql",
		"-F", "owner={owner}", "-F", "repo={repo}", "-f", "query="+prAssignableQuery)
	if err != nil {
		return nil, err
	}
	return parseAssignable(out)
}

func parseAssignable(out []byte) ([]PRCandidate, error) {
	var payload struct {
		Data struct {
			Repository struct {
				AssignableUsers struct {
					Nodes []PRCandidate `json:"nodes"`
				} `json:"assignableUsers"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, errUnreadableAnswer(err)
	}
	nodes := payload.Data.Repository.AssignableUsers.Nodes
	if len(nodes) == 0 {
		return nil, nil
	}
	return nodes, nil
}

// RequestReview asks login for a review on the pull request, or withdraws the
// request. GitHub refuses one addressed to the pull request's own author, which
// is why the picker never offers them (the detail carries the author).
func (s *Service) RequestReview(path string, number int, login string, requested bool) error {
	if login == "" {
		return fmt.Errorf("a reviewer is required")
	}
	flag := "--remove-reviewer"
	if requested {
		flag = "--add-reviewer"
	}
	_, err := s.gh(prEditTimeout, path, prArgs("edit", number, flag, login)...)
	return err
}
