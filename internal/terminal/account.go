package terminal

// SessionAccount reports what a session says about the account its provider
// spends: the environment of the process running in its PTY, and whether that
// environment could be read at all.
//
// It exists for the quota reader (internal/quota), which asks it before
// deciding which login to measure. read is false for a session with no live
// process as well as for a platform that exposes no environment
// (env_other.go), and a reading is withheld rather than taken against the
// machine-wide login lich happens to see.
//
// Denied to the frontend (denyInternal in main.go): the environment it returns
// carries that session's own credentials.
func (s *Service) SessionAccount(id string) (env map[string]string, read bool) {
	if !envReadable {
		return nil, false
	}
	p := s.ptyOf(id)
	if p == nil {
		return nil, false
	}
	env = readEnv(p.Pid())
	return env, env != nil
}
