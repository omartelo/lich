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

	SubagentReport: "[lich] Session %[1]q%[2]s finished the task you handed it (ticket %[3]s). " +
		"Its report:\n\n%[4]s",
	SubagentReportBranch: " on branch %s",
	BlockedNotice: "[lich] Session %q, the subagent you opened, is waiting on a permission prompt in its " +
		"card. Its task stays open: open that card to answer it.",
	ReportSummary: Plural{
		One:   "lich session %s finished",
		Other: "lich sessions %s finished",
	},
	BlockedSummary: "lich session %q is waiting on a permission prompt",
	MergeNotice: "[lich] Pull request #%[1]d %[2]q (branch %[3]s) was merged into %[4]s from lich. " +
		"This is a notice, not a task: update whatever you keep about this work's state, and do nothing else.",
	MergeSummary: "Pull request #%[1]d merged into %[2]s",
	LateNotice:   "[lich] Scheduled for %[1]s, delivered %[2]s late.\n\n",
	ResumePrompt: "[lich] Your usage limit reset. Continue the task you were working on.",

	CopyNotice: "[lich] copy of %s; the original is not reachable from this session, edits stay in the copy.",

	MCPInstructions: `lich runs coding-agent sessions side by side in one window; these tools are how this session works with the others.

Delegate in parallel: for work that can run beside yours, open a worker session in its own git worktree (open_session with worktree) so it gets its own checkout, then hand it the task with send_to_session. Check list_worktrees first — a branch already checked out is opened, not created. Workers are full sessions on the user's screen: visible, steerable, and yours to close (close_session) when the work is done. That is the difference from the subagents your own harness runs, and it is what the user is asking for when they say to fan work out across branches or worktrees: a subagent has no checkout of its own and no card they can open, read or take over mid-task. Reach for one of those to read something and throw it away, not to run the implementation somebody asked to see. The exception is a Claude Code session whose lich-plugin turns a general-purpose subagent into one of these cards: there your Agent tool opens the worker, in the background, and its report comes back on its own, so fan out with it and open sessions here for what a subagent cannot be.

Never poll for results. A send that outlives its wait hands back a ticket and you carry on with your own work; when results are ready, one short [lich] note arrives at your prompt — collect everything at once with wait_for_answer (no ticket). That note only lands between your turns, so while a long turn of your own is running, look in with wait_for_answer and no_wait at the points where a result would change what you do next (before delegating more, after a long validation, before the final synthesis): it returns what is ready and who still owes one without waiting. One look per decision point: if nothing is ready, carry on. Calling it again in a loop is polling, and it burns the tokens this design exists to save.

A subagent or workflow step running inside this session is not its agent, and lich cannot tell them apart: it sends and opens with private, then waits on its own tickets. The [lich] note and the no-ticket wait_for_answer belong to the session's own agent.

When a [lich] message carrying a ticket arrives at YOUR prompt, you are the worker: do the task, then answer with %s — a concise report (what was done, where, what remains), never a transcript.`,
	HandOverFailed: "The task did not reach it: %[1]v. The session is open, so hand it the task with " +
		"%[2]s once whatever that says is dealt with.",
	SendToSessionPrivate: "send_to_session with private",
	OutcomeUnread: "The %[1]q session never picked the task up: it was typed at that prompt and " +
		"nothing read it, so something else has that terminal — a provider still " +
		"starting, or a question of its own on screen (Claude Code asks whether a " +
		"directory is trusted the first time it runs in one). Nothing is queued and " +
		"nothing was answered. Tell the user to open the %[1]q card and clear what is " +
		"on it; the task has to be sent again after that.",
	OutcomeUndelivered: "The task never reached the %[1]q session: it was held back because that terminal " +
		"was not at a prompt — a worktree setup script still running, a provider that " +
		"never came up — and the session ended or stayed that way for too long. Nothing " +
		"is queued anymore and nothing was answered. Tell the user to open the %[1]q card " +
		"to see what happened there; the task has to be sent again after that.",
	OutcomeUnanswered: "The %[1]q session finished its turn without answering through lich. " +
		"Whatever it produced is in that session — tell the user to open the %[1]q card to read it. " +
		"If it is still working in the background, its answer can still arrive on this ticket " +
		"for an hour.",
	OutcomeStopped: "The %q session was closed before it answered, so the task was stopped and no " +
		"answer is coming.",
	OutcomeExpired: "The %[1]q session did not answer within the hour a task is kept open, so the task " +
		"expired and no answer is coming. Open the %[1]q card to see where it stands.",
	StillWorkingPrivate: "%[1]s is still working. The errand is private: no note will announce its result and " +
		"wait_for_answer without a ticket never returns it. Call wait_for_answer with " +
		"ticket %[2]q to hold the line for it — waiting on your own ticket is not polling.",
	StillWorkingOpen: "%[1]s is still working. The errand is open — if that session was not at a prompt yet, " +
		"the task is held and goes in when it is. When its result is ready a short note " +
		"will arrive at your own prompt, so carry on — there is nothing to poll. Collect " +
		"it with wait_for_answer (ticket %[2]q, or no ticket for everything at once).",
	StillWorkingPrivateCLI: "%[1]s is still working. The errand is private: no note will be typed at the sending " +
		"session's prompt, and `lich wait` without a ticket will not return it. " +
		"Hold the line for it with:\n  lich wait %[2]s\n",
	StillWorkingOpenCLI: "%[1]s is still working. The errand is open — a message that session was not ready " +
		"for is held until it is — and a note will be typed at the sending session's " +
		"prompt when its result is ready. To hold the line for it instead:\n" +
		"  lich wait %[2]s\n",
	CollectedNothing:      "Nothing to collect: no results are waiting and no errand of yours is open.",
	CollectedAnswer:       "Answer from %[1]q (ticket %[2]s):\n%[3]s",
	CollectedStillWorking: "Still working: %s. Their results will announce themselves at your prompt.",
	OpenedProject:         "project %q",
	OpenedWorktree:        "%[1]s, in worktree %[2]s",
	OpenedConfined: " It runs confined: an empty home holding only its agent's own state, the " +
		"machine read-only, and writes only inside its checkout.",
	OpenedSession: "Opened session %[1]q (%[2]s) in %[3]s.%[4]s\n" +
		"It answers to %[1]q and to %[5]q. Its agent may still be starting — a fresh " +
		"worktree runs the project's setup script first — so a task you send it " +
		"is held until the agent is up rather than lost.\n",
	ClosedRemoved: "Closed %[1]q and removed its worktree %[2]s.\n",
	ClosedKept: "Closed %[1]q. Its worktree %[2]s is still there, and the session is parked: opening a " +
		"session on that branch again picks its conversation back up.\n",
	ClosedPlain: "Closed %q.\n",
}
