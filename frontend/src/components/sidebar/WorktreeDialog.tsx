import { useEffect, useRef, useState } from "react"
import type { KeyboardEvent } from "react"
import { ProjectService } from "@/lib/rpc"
import type { Branches, Issue, Worktree } from "@/lib/api-types"
import { SearchInput } from "@/components/common/SearchInput"
import { WorktreeSandboxRow } from "@/components/sidebar/WorktreeSandboxRow"
import { WorktreeScriptRows } from "@/components/sidebar/WorktreeScriptRows"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { toBranchName } from "@/lib/git/branch-name"
import { issueBrief, issueName, parseIssueRef } from "@/lib/issue"
import { useSandboxChoice, type SandboxAnswer } from "@/lib/use-sandbox-choice"
import { cn, errorText } from "@/lib/utils"

// How long the field has to settle before the issue behind a reference is
// looked up. Every keystroke of "#381" is a valid reference on its way to the
// one that was meant, so without this one issue costs three gh calls.
const ISSUE_LOOKUP_MS = 400

interface WorktreeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectPath: string
  /** The project the worktree belongs to, and the provider its session will
   * run: together they scope the sandbox rung the confinement row starts on. */
  projectId: string
  providerId: string
  /** The repo's checked-out branch, preselected as the base. */
  currentBranch: string
  /** Create the worktree and open its session; rejections show in the dialog.
   * sandbox is the confinement answer for that session ("on"/"off", "" when the
   * machine cannot confine and nothing was asked). prompt is what that session
   * is handed once it comes up — the issue this worktree is for, or "" when the
   * name was not one. */
  onCreate: (
    name: string,
    base: string,
    baseIsRemote: boolean,
    sandbox: SandboxAnswer,
    prompt: string,
  ) => Promise<void>
  /** Reopen a session on an already-existing worktree. */
  onResume: (wt: { name: string; path: string }) => void
  /** The session whose conversation this worktree's session will carry, when
   * the dialog was opened by a fork rather than by the + button. It only
   * changes what the dialog says: the branching itself is the caller's, and
   * the base list, the setup row and the confinement row are the same either
   * way. */
  forkOf?: { label: string } | null
}

// Row values carry their group so one string identifies the selection:
// "local:main", "remote:origin/main", "worktree:/path/to/checkout".
const rowValue = (group: string, id: string): string => `${group}:${id}`

const splitValue = (value: string): [string, string] => {
  const sep = value.indexOf(":")
  return [value.slice(0, sep), value.slice(sep + 1)]
}

// filterBranches narrows every group to the rows matching the search, so a repo
// with dozens of remote branches collapses to the one being looked for.
function filterBranches(branches: Branches | null, query: string) {
  const needle = query.trim().toLowerCase()
  const match = (name: string) => name.toLowerCase().includes(needle)
  return {
    worktrees: (branches?.worktrees ?? []).filter((w) => match(w.name)),
    local: (branches?.local ?? []).filter(match),
    remote: (branches?.remote ?? []).filter(match),
  }
}

// flatValues is the visible rows in display order, so arrow keys and the
// filter's auto-select can walk them without caring which group they sit in.
function flatValues(vis: ReturnType<typeof filterBranches>): string[] {
  return [
    ...vis.worktrees.map((w) => rowValue("worktree", w.path)),
    ...vis.local.map((b) => rowValue("local", b)),
    ...vis.remote.map((b) => rowValue("remote", b)),
  ]
}

interface GroupProps {
  title: string
  items: ReadonlyArray<{ value: string; label: string }>
  base: string
  onSelect: (value: string) => void
}

function Group({ title, items, base, onSelect }: GroupProps) {
  if (items.length === 0) {
    return null
  }
  return (
    <div>
      <div className="px-2 pb-1 pt-2 text-[0.625rem] font-semibold tracking-wider text-muted-foreground uppercase">
        {title} <span className="font-normal">({items.length})</span>
      </div>
      {items.map((item) => (
        <button
          key={item.value}
          type="button"
          role="option"
          aria-selected={base === item.value}
          onClick={() => onSelect(item.value)}
          className={cn(
            "flex w-full items-center rounded-md px-2 py-1.5 text-left font-mono text-xs outline-none transition-colors",
            base === item.value ? "bg-accent text-accent-foreground" : "hover:bg-accent/50",
          )}
        >
          <span className="truncate">{item.label}</span>
        </button>
      ))}
    </div>
  )
}

// WorktreeDialog collects a worktree name (blank = random adjective-noun, plain
// words = a slug of them) and a base picked from a searchable list — existing
// worktrees to resume, then local and remote branches (remote bases are fetched
// and tracked). It stays open on failure so git's error is readable in place.
export function WorktreeDialog({
  open,
  onOpenChange,
  projectPath,
  projectId,
  providerId,
  currentBranch,
  onCreate,
  onResume,
  forkOf = null,
}: WorktreeDialogProps) {
  const [branches, setBranches] = useState<Branches | null>(null)
  const [name, setName] = useState("")
  const [issue, setIssue] = useState<Issue | null>(null)
  const [issueError, setIssueError] = useState("")
  const [base, setBase] = useState("")
  const [filter, setFilter] = useState("")
  const [loadError, setLoadError] = useState("")
  const [submitError, setSubmitError] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const listRef = useRef<HTMLDivElement>(null)
  // A worktree is always the linked checkout, so the rung is read on that side.
  const sandbox = useSandboxChoice(providerId, projectId, true, open)

  const vis = filterBranches(branches, filter)
  const flat = flatValues(vis)
  const noMatches = branches !== null && flat.length === 0
  // The issue the field names, if it names one. The reference is what was
  // typed; the issue is what GitHub answered for it, and only that carries the
  // title the branch is named after.
  const issueRef = parseIssueRef(name)
  const lookingUp = issueRef !== null && !issue && !issueError
  // What the typed name creates: itself when git takes it, a slug of the first
  // few words when it is a phrase, blank when nothing usable was typed.
  //
  // A reference is never named by the "#381" that was typed, resolved or not.
  // git accepts "#381" as a branch name, and most shells read the "#" as the
  // start of a comment — `git checkout #381` is `git checkout`. So the issue
  // names it once GitHub has answered, and its bare number until then, which is
  // also what is left when the lookup fails and the worktree is made anyway.
  const newBranch = toBranchName(
    issue ? issueName(issue) : issueRef !== null ? String(issueRef) : name,
  )
  const isResume = base.startsWith("worktree:")

  // Keep the selected base in view as it changes — the preselected current
  // branch after load, or the row arrow keys walk to.
  useEffect(() => {
    listRef.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" })
  }, [base, branches])

  // The issue behind the typed reference, looked up while the field is still
  // being typed in. A failure is shown but never blocks: the branch under the
  // field is still a branch, and creating the worktree without the issue's text
  // is a worse outcome than not creating it at all only if lich decides so.
  useEffect(() => {
    setIssue(null)
    setIssueError("")
    if (issueRef === null) {
      return
    }
    let stale = false
    const timer = setTimeout(() => {
      ProjectService.Issue(projectPath, issueRef)
        .then((found) => {
          if (!stale) {
            setIssue(found)
          }
        })
        .catch((err: unknown) => {
          if (!stale) {
            setIssueError(errorText(err))
          }
        })
    }, ISSUE_LOOKUP_MS)
    return () => {
      stale = true
      clearTimeout(timer)
    }
  }, [issueRef, projectPath])

  useEffect(() => {
    if (!open) {
      return
    }
    setBranches(null)
    setName("")
    setBase("")
    setFilter("")
    setLoadError("")
    setSubmitError("")
    setSubmitting(false)
    let stale = false
    ProjectService.ListBranches(projectPath)
      .then((loaded) => {
        if (stale) {
          return
        }
        setBranches(loaded)
        const local = loaded.local ?? []
        const preferred = local.includes(currentBranch) ? currentBranch : local[0]
        setBase(preferred ? rowValue("local", preferred) : "")
      })
      .catch((err: unknown) => {
        if (!stale) {
          setLoadError(errorText(err))
        }
      })
    return () => {
      stale = true
    }
  }, [open, projectPath, currentBranch])

  // Typing a filter drops the current base only when it scrolls out of view, so
  // "type develop, press Enter" lands on the top match without a click.
  const onFilter = (value: string) => {
    setFilter(value)
    const next = flatValues(filterBranches(branches, value))
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
      if (base && !submitting) {
        void submit()
      }
    }
  }

  const submit = async () => {
    const [group, id] = splitValue(base)
    if (group === "worktree") {
      const wt =
        vis.worktrees.find((w: Worktree) => w.path === id) ??
        branches?.worktrees?.find((w: Worktree) => w.path === id)
      if (wt) {
        onResume({ name: wt.name, path: wt.path })
      }
      return
    }
    setSubmitting(true)
    setSubmitError("")
    try {
      await onCreate(
        newBranch,
        id,
        group === "remote",
        sandbox.answer,
        issue ? issueBrief(issue) : "",
      )
    } catch (err) {
      setSubmitError(errorText(err))
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Fixed-height dialog: the base-branch section takes the leftover row
        (minmax(0,1fr)) so its list scrolls instead of growing the modal. */}
      <DialogContent
        className={cn(
          "h-[85vh] sm:max-w-2xl",
          forkOf
            ? "grid-rows-[auto_auto_auto_minmax(0,1fr)_auto_auto_auto]"
            : "grid-rows-[auto_auto_minmax(0,1fr)_auto_auto_auto]",
        )}
      >
        <DialogHeader>
          <DialogTitle>
            {forkOf ? `Fork ${forkOf.label} to a new worktree` : "New worktree"}
          </DialogTitle>
          <DialogDescription>
            Pick a base branch and (optionally) a name for the new worktree.
          </DialogDescription>
        </DialogHeader>

        {forkOf && (
          <div className="border-l-2 border-primary/60 pl-3">
            <div className="text-sm font-medium">Carries the conversation</div>
            <span className="text-xs text-muted-foreground">
              The new session opens on a copy of {forkOf.label}&rsquo;s history and keeps going from
              there. The original card is untouched.
            </span>
          </div>
        )}

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="worktree-name" className="text-xs uppercase tracking-wide">
            Worktree name
          </Label>
          <Input
            id="worktree-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="A branch name, an issue (#128), or the task in plain words"
            disabled={isResume}
            autoFocus
          />
          <div className="flex flex-col font-mono text-xs text-muted-foreground">
            {isResume ? (
              <span>Opens the selected worktree</span>
            ) : lookingUp ? (
              <span>Looking up #{issueRef}…</span>
            ) : (
              <>
                {issue && (
                  <span className="text-foreground">
                    #{issue.number} {issue.title}
                  </span>
                )}
                {issueError && <span className="font-sans text-destructive">{issueError}</span>}
                <span>Branch: {newBranch || "<auto-generated>"}</span>
              </>
            )}
          </div>
        </div>

        <div className="flex min-h-0 flex-col gap-1.5">
          <div className="flex items-center justify-between">
            <Label className="text-xs uppercase tracking-wide">Base branch</Label>
            {!branches && !loadError && (
              <span className="text-xs text-muted-foreground">Loading branches…</span>
            )}
          </div>
          <SearchInput
            value={filter}
            onChange={(e) => onFilter(e.target.value)}
            onKeyDown={onSearchKeyDown}
            placeholder="Search branches…"
            aria-label="Search base branches"
            autoComplete="off"
            spellCheck={false}
            className="font-mono"
          />
          <div
            ref={listRef}
            role="listbox"
            aria-label="Base branch"
            className="min-h-0 flex-1 overflow-y-auto rounded-md border border-input p-1"
          >
            <Group
              title="Worktrees"
              items={
                forkOf
                  ? []
                  : vis.worktrees.map((wt) => ({
                      value: rowValue("worktree", wt.path),
                      label: wt.name,
                    }))
              }
              base={base}
              onSelect={setBase}
            />
            <Group
              title="Local branches"
              items={vis.local.map((branch) => ({
                value: rowValue("local", branch),
                label: branch,
              }))}
              base={base}
              onSelect={setBase}
            />
            <Group
              title="Remote branches"
              items={vis.remote.map((branch) => ({
                value: rowValue("remote", branch),
                label: branch,
              }))}
              base={base}
              onSelect={setBase}
            />
            {noMatches && (
              <div className="px-2 py-6 text-center text-xs text-muted-foreground">
                {filter.trim() ? (
                  <>
                    No branches match{" "}
                    <span className="font-mono text-foreground/80">{filter.trim()}</span>
                  </>
                ) : (
                  "No branches found"
                )}
              </div>
            )}
          </div>
        </div>

        <WorktreeScriptRows projectPath={projectPath} />

        <WorktreeSandboxRow choice={sandbox} />

        {(loadError || submitError) && (
          <span className="text-xs break-words text-destructive">{loadError || submitError}</span>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>Cancel</DialogClose>
          <Button onClick={() => void submit()} disabled={!base || submitting}>
            {submitting
              ? "Creating…"
              : isResume
                ? "Open worktree"
                : forkOf
                  ? "Fork"
                  : "Create worktree"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
