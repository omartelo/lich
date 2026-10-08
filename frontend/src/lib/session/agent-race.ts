import { toBranchName } from "@/lib/git/branch-name"
import type { ProviderKind, Session } from "./sessions"

// A race is one task handed to several agents at once, each in a worktree of
// its own, the whole set filed in one folder. Nothing records it as a race: the
// folder is the grouping, and keeping a winner or removing the race is a
// question asked of any folder (raceRivals), so a race is just the shape the
// race dialog leaves behind.

/**
 * Whether a provider can race: whether its hook reports session-start from the
 * agent's own prompt before the first turn, which is what terminal.Ready waits
 * for before a race's task is sent (internal/terminal, startgate.go). Measured in a fresh worktree (docs/ceilings.md):
 * terminal.Ready only hears the screen go quiet, five providers open a
 * first-run question there that is just as quiet, and the Enter that sends the
 * task answers it; Claude Code and Cursor CLI exit on it. Codex, Antigravity
 * and Crush ask first and report nothing until their first turn. opencode asks nothing,
 * but goes quiet mid-boot under the load of a race, and a task typed then lands
 * on the terminal before its TUI does.
 */
const RACES: Record<ProviderKind, boolean> = {
  claude: true,
  codex: false,
  antigravity: false,
  opencode: false,
  omp: true,
  crush: false,
  cursor: true,
  kiro: true,
}

/** Whether a race or a consolidation can run this provider. */
export function canRace(kind: ProviderKind): boolean {
  return RACES[kind]
}

export interface RaceCheckout {
  kind: ProviderKind
  branch: string
}

export interface RacePlan {
  folder: string
  checkouts: RaceCheckout[]
}

/**
 * The folder and one branch per agent for a race. name is the branch the
 * dialog's name field makes; when it is blank the task names the race instead,
 * since a race always has a task and every branch has to share one name to
 * read as a set. Each branch is that name plus the agent's provider id, which
 * is what tells the cards apart in the folder.
 */
export function planRace(name: string, task: string, kinds: ProviderKind[]): RacePlan {
  const folder = name || toBranchName(task)
  return { folder, checkouts: kinds.map((kind) => ({ kind, branch: `${folder}-${kind}` })) }
}

/**
 * The agents picked after kind is ticked or unticked, in the order the
 * providers are offered so the branches and cards come out in that order too.
 */
export function toggleAgent(
  offered: ProviderKind[],
  picked: ProviderKind[],
  kind: ProviderKind,
): ProviderKind[] {
  const next = picked.includes(kind) ? picked.filter((k) => k !== kind) : [...picked, kind]
  return offered.filter((k) => next.includes(k))
}

/** A checkout removing the race takes away, with every session that lives in it. */
export interface RaceRival {
  path: string
  sessions: Session[]
}

/**
 * The worktrees filed in folder, each with its sessions, except the one at
 * keepPath. Left out, and so kept:
 * - the checkout at keepPath, even when another card in the folder runs in it;
 * - the project's own directory, which is not a worktree to remove;
 * - a checkout with a pinned session in it, because a pinned card refuses a
 *   close and its checkout cannot go without it.
 */
export function folderCheckouts(sessions: Session[], folder: string, keepPath = ""): RaceRival[] {
  const byPath = new Map<string, Session[]>()
  for (const session of sessions) {
    const path = session.path ?? ""
    if (session.folder !== folder || path === "" || path === keepPath) {
      continue
    }
    byPath.set(path, [...(byPath.get(path) ?? []), session])
  }
  return [...byPath]
    .filter(([, members]) => !members.some((session) => session.pinned))
    .map(([path, members]) => ({ path, sessions: members }))
}

/**
 * The folder every one of sessions is filed in, or "" when they are not all in
 * the same one. A race's wall is drawn in place of its folder's block while
 * the split exists, so this is how the wall still answers for its race.
 */
export function sharedFolder(sessions: Session[]): string {
  const folder = sessions[0]?.folder ?? ""
  return sessions.every((session) => session.folder === folder) ? folder : ""
}

/** The checkouts "keep this, remove the others" removes for winner. */
export function raceRivals(sessions: Session[], winner: Session): RaceRival[] {
  return winner.folder ? folderCheckouts(sessions, winner.folder, winner.path ?? "") : []
}

export interface ConsolidationSource {
  branch: string
  path: string
}

/**
 * The prompt a consolidation opens with: every raced branch with its
 * worktree, the task when this window still remembers it, and what to do with
 * them. The path is there for the work an agent never committed, which no
 * branch diff shows.
 */
export function consolidationPrompt(base: string, sources: ConsolidationSource[], task: string) {
  const list = sources.map((source) => `- ${source.branch}, at ${source.path}`).join("\n")
  const given = task
    ? `The task each one was given:\n\n${task}`
    : "The task each one was given: (write it here)"
  return [
    `${sources.length} agents worked the same task in parallel, each in its own worktree off ${base}:`,
    list,
    given,
    `Read each one's work: git diff ${base}...<branch> for what it committed, and git -C <path> diff for what it did not. ` +
      "Compare them, then build in this worktree one implementation that combines the best of each. " +
      "When you are done, explain what you took from which branch and why, and what you left out.",
  ].join("\n\n")
}

// What this window remembers about the races it started, until it reloads: the
// task, which the consolidation prompt quotes, and the consolidating session,
// which the folder offers to keep when the race is removed. Nothing here is
// stored. A reload forgets both, and the dialogs fall back on what the user
// types and on "keep this, remove the others" from the card (docs/ceilings.md).
const tasks = new Map<string, string>()
const consolidators = new Map<string, string>()
const raceKey = (projectId: string, folder: string) => `${projectId}\u0000${folder}`

export function rememberRaceTask(projectId: string, folder: string, task: string): void {
  tasks.set(raceKey(projectId, folder), task)
}

export function raceTask(projectId: string, folder: string): string {
  return tasks.get(raceKey(projectId, folder)) ?? ""
}

export function rememberConsolidator(projectId: string, folder: string, sessionId: string): void {
  consolidators.set(raceKey(projectId, folder), sessionId)
}

export function consolidatorOf(projectId: string, folder: string): string {
  return consolidators.get(raceKey(projectId, folder)) ?? ""
}
