package store

import (
	"testing"

	"github.com/omartelo/lich/internal/prompt"
)

func TestPromptLanguageIsEnglishUntilSetAndPersists(t *testing.T) {
	svc := newTestStore(t)

	if got := svc.PromptLanguage(); got != prompt.English {
		t.Errorf("PromptLanguage = %q on a fresh workspace, want en", got)
	}
	if err := svc.SetSetting("prompt.language", "", "pt-BR"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.PortugueseBR {
		t.Errorf("PromptLanguage = %q after choosing pt-BR", got)
	}
	if err := svc.SetSetting("prompt.language", "", "klingon"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.English {
		t.Errorf("PromptLanguage = %q for an unknown tag, want en", got)
	}
}
