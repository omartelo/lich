package prompt

// en is the source of truth: every other locale translates these.
var en = Catalog{
	isOne: isOneEnglish,

	BriefingIntro: "You are running inside lich, which runs coding-agent sessions side by side and can " +
		"open more of them beside this one — each a card the user watches and can take over " +
		"mid-task, in its own git worktree when the work needs its own checkout. ",
	BriefingCards: "Here your own Agent tool opens them: a general-purpose subagent runs as " +
		"one of those cards, in this checkout unless you ask for isolation 'worktree', " +
		"in the background, and its report comes back to you on its own. Fan work out with " +
		"it. Open a session yourself only for what a subagent cannot be: another agent " +
		"kind, a branch the user named, or work that must outlive this session. ",
	BriefingNative: "This session is itself a subagent card another session opened, and here " +
		"your own Agent tool runs a subagent inside this session rather than as a card. Fan " +
		"work out with it rather than with new lich sessions: your final message is the " +
		"report the session that opened you is waiting for.",
	BriefingSessions: "When work is " +
		"to be fanned out — several tasks at once, one per branch or checkout — those sessions " +
		"are what to open, not the subagents your own harness runs: a subagent has no checkout, " +
		"no card, and nothing the user can steer or resume. ",
	BriefingCommandTools: "The lich tools in your list open one and hand it the task.",
	BriefingCommandCLI: "Open one with `lich open --worktree BRANCH --prompt 'the task'`, which opens the " +
		"session and hands it the task in one command.",

	RelayMessage:  "[lich] %[1]s, not from your own prompt.\n\n%[2]s\n\n%[3]s",
	OriginCLI:     "Message relayed by the lich command line",
	OriginSession: "Message from session %q",
	ReplyCommand:  "  \"$LICH_BIN\" reply %s \"<your answer>\"",
	ReplyRouteCLI: "When you have an answer, send it back by running:\n%s",
	ReplyRouteTools: "When you have an answer, send it back with the lich tool `%[1]s` (ticket %[2]s), " +
		"or by running:\n%[3]s",
	ReplyOnlyWayBack: "That ticket is the only way back: whoever asked is blocked on it and " +
		"is reading nothing else. Do not answer by messaging a peer session — an answer " +
		"sent any other way is lost. Keep the answer a concise report — what was done, " +
		"where, and what remains — never a transcript: the sender pays to read every byte, " +
		"and the detail is in your commits and files anyway.",
	WorkerTask: "[lich] Task from session %[1]q, which opened this session as its subagent. " +
		"Your final message when you finish is sent back to it as your report.\n\n%[2]s",
	PickTicketNudge: "[lich] Your turn ended with no answer sent, so %[1]d requests went back to their senders " +
		"unanswered:\n%[2]s\nNothing outside this session can say which of them that turn was, " +
		"which is why none of them could be answered for you. One you are still working on " +
		"still takes its answer, and every answer from here on has to name its ticket: " +
		"lich reply <ticket> \"<answer>\".",
	NudgeResultsReady: Plural{
		One:   "The task you sent %[2]s has its result ready",
		Other: "Results from %[1]d tasks you sent are ready (%[2]s)",
	},
	NudgeRouteCLI:   "run:\n  \"$LICH_BIN\" wait",
	NudgeRouteTools: "call the lich tool `%s` with no ticket, or run:\n  \"$LICH_BIN\" wait",
	NudgeNotice:     "[lich] %[1]s. To collect everything at once, %[2]s",
}
