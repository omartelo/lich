package project

import "strings"

// CommitIdentity is the name and email the next commit in a checkout would be
// authored with — the fact the project's gh account (ghaccount.go) does not
// carry, since that one only governs what lich asks gh. Local says the value
// comes from the checkout's own config rather than the global one.
//
// The zero value is a legitimate answer, not a failure: a checkout with no
// identity configured is one of the states worth showing.
type CommitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Local bool   `json:"local"`
}

// CommitIdentity reads the identity git itself resolves in the checkout at
// path, so a repository-local override wins over the global config exactly as
// it would for a commit. A non-repository, an unreadable config or no identity
// at all answer the zero value — hence the quiet probes (see runGit for why).
func (s *Service) CommitIdentity(path string) CommitIdentity {
	identity := CommitIdentity{
		Name:  gitConfig(path, "user.name"),
		Email: gitConfig(path, "user.email"),
	}
	// The email alone decides "set in this repository": it is the value git
	// refuses a commit without, and the one the readout shows.
	identity.Local = identity.Email != "" && gitConfig(path, "--local", "user.email") != ""
	return identity
}

// gitConfig reads one config key in the checkout at path — with git's own
// resolution order, or narrowed to a scope ("--local") when one leads the
// arguments. "" for every key git declines to answer.
func gitConfig(path string, args ...string) string {
	out, _ := gitQuiet(path, append([]string{"config", "--get"}, args...)...)
	return strings.TrimSpace(out)
}
