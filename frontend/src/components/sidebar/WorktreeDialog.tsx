import {useEffect, useRef, useState} from "react"
import type {KeyboardEvent} from "react"
import {Search} from "lucide-react"
import {ProjectService} from "@/lib/rpc"
import type {Branches, Worktree} from "@/lib/api-types"
import {Button} from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {isValidBranchName} from "@/lib/branch-name"
import {cn, errorText} from "@/lib/utils"

interface WorktreeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectPath: string
  /** The repo's checked-out branch, preselected as the base. */
  currentBranch: string
  /** Create the worktree and open its session; rejections show in the dialog. */
  onCreate: (name: string, base: string, baseIsRemote: boolean) => Promise<void>
  /** Reopen a session on an already-existing worktree. */
  onResume: (wt: { name: string; path: string }) => void
}

// Row values carry their group so one string identifies the selection:
// "local:main", "remote:origin/main", "worktree:/path/to/checkout".
const valueOf = (group: string, id: string): string => `${group}:${id}`

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
    ...vis.worktrees.map((w) => valueOf("worktree", w.path)),
    ...vis.local.map((b) => valueOf("local", b)),
    ...vis.remote.map((b) => valueOf("remote", b)),
  ]
}

interface GroupProps {
  title: string
  items: ReadonlyArray<{ value: string; label: string }>
  base: string
  onSelect: (value: string) => void
}

function Group({title, items, base, onSelect}: GroupProps) {
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
            base === item.value
              ? "bg-accent text-accent-foreground"
              : "hover:bg-accent/50",
          )}
        >
          <span className="truncate">{item.label}</span>
        </button>
      ))}
    </div>
  )
}

// WorktreeDialog collects a worktree name (blank = random adjective-noun) and a
// base picked from a searchable list — existing worktrees to resume, then local
// and remote branches (remote bases are fetched and tracked). It stays open on
// failure so git's error is readable in place.
export function WorktreeDialog({
                                 open,
                                 onOpenChange,
                                 projectPath,
                                 currentBranch,
                                 onCreate,
                                 onResume,
                               }: WorktreeDialogProps) {
  const [branches, setBranches] = useState<Branches | null>(null)
  const [name, setName] = useState("")
  const [base, setBase] = useState("")
  const [filter, setFilter] = useState("")
  const [loadError, setLoadError] = useState("")
  const [submitError, setSubmitError] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const listRef = useRef<HTMLDivElement>(null)

  const vis = filterBranches(branches, filter)
  const flat = flatValues(vis)
  const noMatches = branches !== null && flat.length === 0
  const trimmed = name.trim()
  const nameInvalid = trimmed !== "" && !isValidBranchName(trimmed)
  const isResume = base.startsWith("worktree:")

  // Keep the selected base in view as it changes — the preselected current
  // branch after load, or the row arrow keys walk to.
  useEffect(() => {
    listRef.current
      ?.querySelector('[aria-selected="true"]')
      ?.scrollIntoView({block: "nearest"})
  }, [base, branches])

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
        setBase(preferred ? valueOf("local", preferred) : "")
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
      if (!nameInvalid && base && !submitting) {
        void submit()
      }
    }
  }

  const submit = async () => {
    const [group, id] = splitValue(base)
    if (group === "worktree") {
      const wt = vis.worktrees.find((w: Worktree) => w.path === id)
        ?? branches?.worktrees?.find((w: Worktree) => w.path === id)
      if (wt) {
        onResume({name: wt.name, path: wt.path})
      }
      return
    }
    setSubmitting(true)
    setSubmitError("")
    try {
      await onCreate(trimmed, id, group === "remote")
    } catch (err) {
      setSubmitError(errorText(err))
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* Fixed-height dialog: the base-branch section takes the leftover row
        (minmax(0,1fr)) so its list scrolls instead of growing the modal. */}
      <DialogContent className="h-[85vh] grid-rows-[auto_auto_minmax(0,1fr)_auto_auto] sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>New worktree</DialogTitle>
          <DialogDescription>
            Pick a base branch and (optionally) a name for the new worktree.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="worktree-name" className="text-xs uppercase tracking-wide">
            Worktree name
          </Label>
          <Input
            id="worktree-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Leave blank to auto-generate (e.g. swift-rabbit)"
            disabled={isResume}
            aria-invalid={nameInvalid || undefined}
            autoFocus
          />
          {nameInvalid ? (
            <span className="text-xs text-destructive">Invalid branch name</span>
          ) : (
            <span className="font-mono text-xs text-muted-foreground">
              {isResume
                ? "Opens the selected worktree"
                : `Branch: ${trimmed || "<auto-generated>"}`}
            </span>
          )}
        </div>

        <div className="flex min-h-0 flex-col gap-1.5">
          <div className="flex items-center justify-between">
            <Label className="text-xs uppercase tracking-wide">Base branch</Label>
            {!branches && !loadError && (
              <span className="text-xs text-muted-foreground">Loading branches…</span>
            )}
          </div>
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={filter}
              onChange={(e) => onFilter(e.target.value)}
              onKeyDown={onSearchKeyDown}
              placeholder="Search branches…"
              aria-label="Search base branches"
              autoComplete="off"
              spellCheck={false}
              className="pl-8 font-mono"
            />
          </div>
          <div
            ref={listRef}
            role="listbox"
            aria-label="Base branch"
            className="min-h-0 flex-1 overflow-y-auto rounded-md border border-input p-1"
          >
            <Group
              title="Worktrees"
              items={vis.worktrees.map((wt) => ({value: valueOf("worktree", wt.path), label: wt.name}))}
              base={base}
              onSelect={setBase}
            />
            <Group
              title="Local branches"
              items={vis.local.map((branch) => ({value: valueOf("local", branch), label: branch}))}
              base={base}
              onSelect={setBase}
            />
            <Group
              title="Remote branches"
              items={vis.remote.map((branch) => ({value: valueOf("remote", branch), label: branch}))}
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

        {(loadError || submitError) && (
          <span className="text-xs break-words text-destructive">
            {loadError || submitError}
          </span>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="ghost"/>}>Cancel</DialogClose>
          <Button
            onClick={() => void submit()}
            disabled={nameInvalid || !base || submitting}
          >
            {submitting
              ? "Creating…"
              : isResume
                ? "Open worktree"
                : "Create worktree"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
