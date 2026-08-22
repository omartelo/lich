package system

import (
	"slices"
	"testing"
)

// The list is what the setting shows instead of asking the user to remember
// what is in their agent, so it has to read like the person who loaded the key
// would name it: the comment they gave it, not the fingerprint ssh identifies it
// by.
func TestParseAgentKeysReadsSSHAddOutput(t *testing.T) {
	out := "4096 SHA256:abc+def/ghi me@example.com (RSA)\n" +
		"256 SHA256:jkl work laptop key (ED25519)\n"
	want := []string{
		"me@example.com (RSA 4096)",
		"work laptop key (ED25519 256)",
	}
	if got := parseAgentKeys(out); !slices.Equal(got, want) {
		t.Errorf("parseAgentKeys = %v, want %v", got, want)
	}
}

// An agent holding nothing is not a key called "The agent has no identities."
func TestParseAgentKeysReadsAnEmptyAgent(t *testing.T) {
	if got := parseAgentKeys("The agent has no identities.\n"); len(got) != 0 {
		t.Errorf("an empty agent listed %v, want nothing", got)
	}
	if got := parseAgentKeys("\n  \n"); len(got) != 0 {
		t.Errorf("blank output listed %v, want nothing", got)
	}
}

// A line this cannot read is still an identity the flag hands over. Dropping it
// would understate what the setting opens, which is the one thing this list
// exists to state.
func TestParseAgentKeysKeepsALineItCannotRead(t *testing.T) {
	const odd = "something ssh-add printed"
	got := parseAgentKeys(odd + "\n")
	if !slices.Equal(got, []string{odd}) {
		t.Errorf("parseAgentKeys = %v, want the line kept whole", got)
	}
}
