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

	// internal/relay: a subagent's report and block, a merge notice, a late
	// scheduled prompt and the continuation parked at a usage limit.
	SubagentReport       string
	SubagentReportBranch string
	BlockedNotice        string
	ReportSummary        Plural
	BlockedSummary       string
	MergeNotice          string
	MergeSummary         string
	LateNotice           string
	// ResumePrompt is stored in the sessions table as a parked prompt, so a row
	// written in one language has to be recognised in every other: match it
	// with relay's isResumePrompt, never with ==.
	ResumePrompt string

	// internal/drop: the line under a copied file's path.
	CopyNotice string

	// internal/cli: the lich command line and its MCP server, read from
	// EnvVar. MCP tool descriptions, flag help and errors stay English.
	MCPInstructions        string
	HandOverFailed         string
	SendToSessionPrivate   string
	OutcomeUnread          string
	OutcomeUndelivered     string
	OutcomeUnanswered      string
	OutcomeStopped         string
	OutcomeExpired         string
	StillWorkingPrivate    string
	StillWorkingOpen       string
	StillWorkingPrivateCLI string
	StillWorkingOpenCLI    string
	CollectedNothing       string
	CollectedAnswer        string
	CollectedStillWorking  string
	OpenedProject          string
	OpenedWorktree         string
	OpenedConfined         string
	OpenedSession          string
	ClosedRemoved          string
	ClosedKept             string
	ClosedPlain            string
}
