package store

import (
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// A long paste stays folded unless the user turned unfolding on, so only the
// literal "true" reads as on.
func TestPasteUnfoldIsOnOnlyForTheLiteralTrue(t *testing.T) {
	svc := newTestStore(t)
	if svc.PasteUnfold(providers.OpenCode) {
		t.Error("unconfigured provider unfolds pastes, want off")
	}
	_ = svc.SetSetting("provider.opencode.pasteUnfold", globalScope, "true")
	if !svc.PasteUnfold(providers.OpenCode) {
		t.Error(`"true" leaves pastes folded, want unfolded`)
	}
	if svc.PasteUnfold(providers.Claude) {
		t.Error("another provider follows opencode's setting, want its own")
	}
	_ = svc.SetSetting("provider.opencode.pasteUnfold", globalScope, "false")
	if svc.PasteUnfold(providers.OpenCode) {
		t.Error(`"false" leaves pastes unfolded, want folded`)
	}
}
