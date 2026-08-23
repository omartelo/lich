package terminal

// SessionAccount reports what a session says about the account its provider
// spends: the environment of the process running in its PTY, whether that
// environment could be read at all, and whether the session was spawned from a
// binary the user configured rather than the provider's own.
//
// It exists for the quota reader (internal/quota), which asks it before deciding
// which login to measure. read is false for a session with no live process as
// well as for a platform that exposes no environment (env_other.go); custom is
// answered from the settings either way, because the whole point of the pairing
// is to tell "this session spends another account and I cannot see which" apart
// from "this session spends the default one".
//
// Denied to the frontend (denyInternal in main.go): the environment it returns
// carries that session's own credentials.
func (s *Service) SessionAccount(id string) (env map[string]string, custom, read bool) {
	custom = s.store.SessionCustomBin(id)
	if !envReadable {
		return nil, custom, false
	}
	p := s.ptyOf(id)
	if p == nil {
		return nil, custom, false
	}
	env = readEnv(p.Pid())
	return env, custom, env != nil
}
