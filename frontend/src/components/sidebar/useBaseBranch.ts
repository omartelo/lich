import { useEffect, useRef, useState } from "react"
import type { KeyboardEvent, RefObject } from "react"
import type { Branches, Worktree } from "@/lib/api-types"
import { useGitStatus } from "@/lib/git/use-git-status"
import { ProjectService } from "@/lib/rpc"
import { errorText } from "@/lib/utils"

// A checkout as a base: its branch at the same commit, plus the work it has not
// committed. The checkout is the forked session's, or the project's own for the
// + button. A fork is usually made at the moment there is uncommitted work to
// take two ways, so a fork opens on this row; the + button is usually a new
// task, so it opens on the branch and leaves the row one pick away. Either way
// the branch itself is that same branch at its last commit.
export interface WorkingTree {
  path: string
  branch: string
  files: number
}

// Row values carry their group so one string identifies the selection:
// "local:main", "remote:origin/main", "worktree:/path/to/checkout".
export const rowValue = (group: string, id: string): string => `${group}:${id}`

const splitValue = (value: string): [string, string] => {
  const sep = value.indexOf(":")
  return [value.slice(0, sep), value.slice(sep + 1)]
}

// filterBranches narrows every group to the rows matching the search, so a repo
// with dozens of remote branches collapses to the one being looked for. The
// working-tree row answers to its branch name, which is the only name it has.
// Existing worktrees are rows only where picking one can mean reopening it.
function filterBranches(
  branches: Branches | null,
  query: string,
  tree: WorkingTree | null,
  offerResume: boolean,
) {
  const needle = query.trim().toLowerCase()
  const match = (name: string) => name.toLowerCase().includes(needle)
  return {
    tree: tree && match(tree.branch) ? tree : null,
    worktrees: offerResume ? (branches?.worktrees ?? []).filter((w) => match(w.name)) : [],
    local: (branches?.local ?? []).filter(match),
    remote: (branches?.remote ?? []).filter(match),
  }
}

export type VisibleBranches = ReturnType<typeof filterBranches>

// flatValues is the visible rows in display order, so arrow keys and the
// filter's auto-select can walk them without caring which group they sit in.
function flatValues(vis: VisibleBranches): string[] {
  return [
    ...(vis.tree ? [rowValue("tree", vis.tree.path)] : []),
    ...vis.worktrees.map((w) => rowValue("worktree", w.path)),
    ...vis.local.map((b) => rowValue("local", b)),
    ...vis.remote.map((b) => rowValue("remote", b)),
  ]
}

/** What the picked row asks for: reopening a worktree that exists, or a new
 * one off a branch, local or remote, optionally carrying a checkout's
 * uncommitted work. */
export type BaseChoice =
  | { resume: Worktree }
  | { resume: null; branch: string; remote: boolean; carryFrom: string }

export interface BaseBranch {
  branches: Branches | null
  loadError: string
  base: string
  setBase: (value: string) => void
  filter: string
  onFilter: (value: string) => void
  vis: VisibleBranches
  noMatches: boolean
  listRef: RefObject<HTMLDivElement>
  isResume: boolean
  /** Arrow keys walk the rows; Enter calls onEnter once a row is picked. */
  onSearchKeyDown: (event: KeyboardEvent<HTMLInputElement>) => void
  /** The picked row as what to create, or null while nothing is picked. */
  choice: () => BaseChoice | null
}

interface UseBaseBranchOptions {
  open: boolean
  projectPath: string
  /** The repo's checked-out branch, preselected as the base. */
  currentBranch: string
  /** The session a fork carries, whose checkout is the working-tree row and
   * the base the dialog opens on; null for the + button and a race. */
  forkOf: { path: string } | null
  /** Whether existing worktrees are offered, picking one meaning reopen it. */
  offerResume: boolean
  onEnter: () => void
}

// useBaseBranch is the base half of the new-worktree dialogs: the branches
// loaded on open, the search over them, the row the dialog opens on and the
// keys that walk the list. BaseBranchPicker draws it.
export function useBaseBranch({
  open,
  projectPath,
  currentBranch,
  forkOf,
  offerResume,
  onEnter,
}: UseBaseBranchOptions): BaseBranch {
  const [branches, setBranches] = useState<Branches | null>(null)
  const [base, setBase] = useState("")
  const [filter, setFilter] = useState("")
  const [loadError, setLoadError] = useState("")
  const listRef = useRef<HTMLDivElement>(null)
  // The checkout the working-tree row copies from, polled by the shared store
  // the sidebar is already subscribed to for that path.
  const source = useGitStatus(forkOf?.path ?? projectPath)
  // The branch has to be one git will take as a base: a checkout sitting on a
  // detached HEAD reports something that names no branch, and offering it would
  // buy a git refusal at the end of a dialog the user already filled in.
  const tree: WorkingTree | null =
    source && source.files > 0 && (branches?.local ?? []).includes(source.branch)
      ? { path: forkOf?.path ?? projectPath, branch: source.branch, files: source.files }
      : null

  const vis = filterBranches(branches, filter, tree, offerResume)
  const flat = flatValues(vis)

  // Keep the selected base in view as it changes — the preselected current
  // branch after load, or the row arrow keys walk to.
  useEffect(() => {
    listRef.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" })
  }, [base, branches])

  useEffect(() => {
    if (!open) {
      return
    }
    setBranches(null)
    setBase("")
    setFilter("")
    setLoadError("")
    let stale = false
    ProjectService.ListBranches(projectPath)
      .then((loaded) => {
        if (!stale) {
          setBranches(loaded)
        }
      })
      .catch((err: unknown) => {
        if (!stale) {
          setLoadError(errorText(err))
        }
      })
    return () => {
      stale = true
    }
  }, [open, projectPath])

  // The row the dialog opens on, applied while nothing is selected yet: a
  // fork's working tree when it has work to carry, then the branch
  // that session is on, then the repository's own branch. A fork waits for its
  // source's git status instead of preselecting a branch it would have to move
  // off a tick later — the card behind the dialog is polling that path already,
  // so the wait is usually no wait at all.
  useEffect(() => {
    // A working-tree row that is gone takes its selection with it: the source
    // session committing while this dialog is open drops the row, and a base
    // still naming it would reach git as no base at all. Clearing it re-runs
    // this effect on the branch below, which is that same branch.
    if (base.startsWith("tree:") && tree === null) {
      setBase("")
      return
    }
    if (!open || base !== "" || branches === null || (forkOf && source === null)) {
      return
    }
    if (forkOf && tree) {
      setBase(rowValue("tree", tree.path))
      return
    }
    const local = branches.local ?? []
    const preferred =
      [source?.branch ?? "", currentBranch].find((branch) => local.includes(branch)) ?? local[0]
    if (preferred) {
      setBase(rowValue("local", preferred))
    }
  }, [open, base, branches, forkOf, source, tree, currentBranch])

  // Typing a filter drops the current base only when it scrolls out of view, so
  // "type develop, press Enter" lands on the top match without a click.
  const onFilter = (value: string) => {
    setFilter(value)
    const next = flatValues(filterBranches(branches, value, tree, offerResume))
    if (next.length > 0 && !next.includes(base)) {
      setBase(next[0])
    }
  }

  const move = (delta: number) => {
    if (flat.length === 0) {
      return
    }
    const idx = flat.indexOf(base)
    const next = Math.min(Math.max((idx < 0 ? 0 : idx) + delta, 0), flat.length - 1)
    setBase(flat[next])
  }

  const onSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown") {
      event.preventDefault()
      move(1)
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      move(-1)
    } else if (event.key === "Enter") {
      event.preventDefault()
      onEnter()
    }
  }

  const choice = (): BaseChoice | null => {
    if (!base) {
      return null
    }
    const [group, id] = splitValue(base)
    if (group === "worktree") {
      const wt =
        vis.worktrees.find((w: Worktree) => w.path === id) ??
        branches?.worktrees?.find((w: Worktree) => w.path === id)
      return wt ? { resume: wt } : null
    }
    // The working-tree row branches off the same branch every other row would
    // have named; what makes it different is the checkout it copies after.
    return {
      resume: null,
      branch: group === "tree" ? (tree?.branch ?? "") : id,
      remote: group === "remote",
      carryFrom: group === "tree" ? id : "",
    }
  }

  return {
    branches,
    loadError,
    base,
    setBase,
    filter,
    onFilter,
    vis,
    noMatches: branches !== null && flat.length === 0,
    listRef,
    isResume: base.startsWith("worktree:"),
    onSearchKeyDown,
    choice,
  }
}
