package doctor

import (
	"github.com/omartelo/lich/internal/chromium"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/singleton"
)

// The three lookups below reach the machine the same way for both diagnostics:
// `lich doctor` checks what a launch would find, and `lich rage` reports it.
// They live here, exported, because a probe that drifted between the two tools
// would have them describe two different machines.

// Browser resolves the browser the window is launched as, and says which rung
// of the resolution ladder produced it — a machine where lich opens the "wrong"
// browser is diagnosed by that step and by nothing else.
func Browser() (string, error) {
	found, err := chromium.Resolve(chromium.RealEnv())
	if err != nil {
		return "", err
	}
	return found.Describe(), nil
}

// Providers lists the harnesses on PATH. A detection that fails outright is
// reported as none found: both callers treat "no provider" as their finding,
// and neither has anywhere to put the error.
func Providers() []providers.Detected {
	found, err := providers.New().Detect()
	if err != nil {
		return nil
	}
	return found
}

// Instance returns the lich recorded in the runtime file and whether it still
// answers. A recorded instance that does not is itself a finding, so the
// caller gets both halves rather than a single boolean.
func Instance(configDir string) (*singleton.Info, bool) {
	info, err := singleton.Read(configDir)
	if err != nil || info == nil {
		return nil, false
	}
	return info, singleton.Ping(info.Port, info.Token)
}
