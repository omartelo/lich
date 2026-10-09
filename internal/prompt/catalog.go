package prompt

// Catalog is every prompt text in one language. Add a field here, then its
// text in en.go and in every other locale file; TestCatalogParity fails until
// each locale has it with the same format verbs.
//
// Untranslated in every locale, because agents and tools match them literally:
// the [lich] token, tool and CLI names, ticket ids, and tool parameters.
type Catalog struct {
	isOne func(int) bool

	// internal/relay: SpawnBriefing. These are passed as one argv entry and a
	// provider shipped as a .cmd is spawned through cmd.exe, so they must not
	// contain a double quote or angle brackets in any locale.
	BriefingIntro        string
	BriefingCards        string
	BriefingNative       string
	BriefingSessions     string
	BriefingCommandTools string
	BriefingCommandCLI   string

	// internal/relay: compose, origin and replyInstruction.
	RelayMessage      string
	OriginCLI         string
	OriginSession     string
	ReplyCommand      string
	ReplyRouteCLI     string
	ReplyRouteTools   string
	ReplyOnlyWayBack  string
	WorkerTask        string
	PickTicketNudge   string
	NudgeResultsReady Plural
	NudgeRouteCLI     string
	NudgeRouteTools   string
	NudgeNotice       string
}
