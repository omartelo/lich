import { Terminal } from "@/lib/rpc"

// Text written into a session the moment its card exists lands wherever that
// PTY has got to — the checkout's setup script, a provider still taking the tty
// over — and both read it and throw it away. What is left on screen is a prompt
// that was never given anything, which is how this was found: a pull request
// handed to a session opened for it, and nothing at the prompt when the agent
// came up.
//
// So the write waits for a prompt to write at. `terminal.Ready` is the same
// question the relay asks before delivering a task, and it answers a session
// that has not spawned yet as well as one still installing its dependencies —
// so one call covers the session opened a moment ago and the one that has been
// sitting there all along, which answers immediately.
//
// It also answers no while the person at that prompt has unsent input, which is
// the relay's reason rather than this one — nothing is sent here. The wait is
// kept anyway: a paste landing in the middle of someone's half-typed sentence is
// the worse of the two, and the draft releases itself (`internal/terminal`,
// draftIdle).
//
// Polled from here rather than waited on in Go: the wait can be minutes, and a
// request held open for that long is one of the browser's handful of
// connections to this backend. A short call every quarter second is the cheaper
// half of that trade on loopback.

const POLL_MS = 250

// How long a prompt has to appear. A worktree's first setup run is minutes of
// `pnpm install` before its provider starts, and the paste is worth waiting
// out; a session that never comes up must still stop being waited for.
const LIMIT_MS = 5 * 60 * 1000

/**
 * Write text into a session's terminal once its prompt can take it. Rejects
 * when no prompt appears in time — the caller is the one who knows what was
 * being handed over, so it words what was lost.
 */
export async function writeAtPrompt(sessionId: string, text: string): Promise<void> {
  const deadline = Date.now() + LIMIT_MS
  while (!(await Terminal.Ready(sessionId))) {
    if (Date.now() >= deadline) {
      throw new Error("the session never reached a prompt")
    }
    await new Promise((resolve) => setTimeout(resolve, POLL_MS))
  }
  await Terminal.Write(sessionId, text)
}
