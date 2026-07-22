import {useEffect, useRef, useState} from "react"
import type {ReactNode} from "react"
import {ChevronRight} from "lucide-react"
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

type GroupKey = "worktrees" | "local" | "remote"

// Local starts open (it holds the preselected current branch); the other groups
// start collapsed so long branch lists don't bury each other.
const DEFAULT_OPEN_GROUPS: Record<GroupKey, boolean> = {
  worktrees: false,
  local: true,
  remote: false,
}

interface BranchGroupProps {
  title: string
  count: number
  open: boolean
  onToggle: () => void
  children: ReactNode
}

// BranchGroup is one collapsible section of the list: a sticky header with a
// chevron and item count, and the rows when expanded.
function BranchGroup({title, count, open, onToggle, children}: BranchGroupProps) {
  return (
    <>
      <button
        type="button"
        aria-expanded={open}
        onClick={onToggle}
        className="sticky top-0 z-10 flex w-full items-center gap-1.5 border-y border-border bg-muted px-2.5 py-1.5 text-[0.625rem] font-semibold tracking-wider text-muted-foreground uppercase outline-none first:border-t-0 hover:text-foreground"
      >
        <ChevronRight className={cn("size-3 transition-transform", open && "rotate-90")}/>
        {title}
        <span className="font-normal">({count})</span>
      </button>
      {open && children}
    </>
  )
}

interface BranchRowProps {
  label: string
  selected: boolean
  onSelect: () => void
}

function BranchRow({label, selected, onSelect}: BranchRowProps) {
  return (
    <button
      type="button"
      role="option"
      aria-selected={selected}
      onClick={onSelect}
      className={cn(
        "flex w-full items-center py-1.5 pr-2.5 pl-4 text-left font-mono text-xs outline-none",
        selected
          ? "bg-accent text-accent-foreground"
          : "text-foreground hover:bg-accent/50",
      )}
    >
      <span className="truncate">{label}</span>
    </button>
  )
}

// WorktreeDialog collects a worktree name (blank = random adjective-noun) and a
// base picked from an inline grouped list — existing worktrees to resume, then
// local and remote branches (remote bases are fetched and tracked). It stays
// open on failure so git's error is readable in place.
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
  const [loadError, setLoadError] = useState("")
  const [submitError, setSubmitError] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [openGroups, setOpenGroups] = useState(DEFAULT_OPEN_GROUPS)
  const listRef = useRef<HTMLDivElement>(null)

  const toggleGroup = (key: GroupKey) =>
    setOpenGroups((prev) => ({...prev, [key]: !prev[key]}))

  // Bring the preselected base (current branch) into view once the list loads;
  // it usually sits inside the Local group, below the fold.
  useEffect(() => {
    listRef.current
      ?.querySelector('[aria-selected="true"]')
      ?.scrollIntoView({block: "center"})
  }, [branches])

  useEffect(() => {
    if (!open) {
      return
    }
    setBranches(null)
    setName("")
    setBase("")
    setLoadError("")
    setSubmitError("")
    setSubmitting(false)
    setOpenGroups(DEFAULT_OPEN_GROUPS)
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
        // Existing worktrees are the resume targets; surface the group so a
        // closed-but-kept worktree is one click away instead of behind a fold.
        if ((loaded.worktrees ?? []).length > 0) {
          setOpenGroups((prev) => ({...prev, worktrees: true}))
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
  }, [open, projectPath, currentBranch])

  const local = branches?.local ?? []
  const remote = branches?.remote ?? []
  const worktrees = branches?.worktrees ?? []
  const empty = branches !== null && !local.length && !remote.length && !worktrees.length
  const trimmed = name.trim()
  const nameInvalid = trimmed !== "" && !isValidBranchName(trimmed)
  const isResume = base.startsWith("worktree:")

  const submit = async () => {
    const [group, id] = splitValue(base)
    if (group === "worktree") {
      const wt = worktrees.find((w: Worktree) => w.path === id)
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
          <div
            ref={listRef}
            role="listbox"
            aria-label="Base branch"
            className="min-h-0 flex-1 overflow-y-auto rounded-md border border-input"
          >
            {worktrees.length > 0 && (
              <BranchGroup
                title="Worktrees"
                count={worktrees.length}
                open={openGroups.worktrees}
                onToggle={() => toggleGroup("worktrees")}
              >
                {worktrees.map((wt: Worktree) => (
                  <BranchRow
                    key={wt.path}
                    label={wt.name}
                    selected={base === valueOf("worktree", wt.path)}
                    onSelect={() => setBase(valueOf("worktree", wt.path))}
                  />
                ))}
              </BranchGroup>
            )}
            {local.length > 0 && (
              <BranchGroup
                title="Local branches"
                count={local.length}
                open={openGroups.local}
                onToggle={() => toggleGroup("local")}
              >
                {local.map((branch) => (
                  <BranchRow
                    key={branch}
                    label={branch}
                    selected={base === valueOf("local", branch)}
                    onSelect={() => setBase(valueOf("local", branch))}
                  />
                ))}
              </BranchGroup>
            )}
            {remote.length > 0 && (
              <BranchGroup
                title="Remote branches"
                count={remote.length}
                open={openGroups.remote}
                onToggle={() => toggleGroup("remote")}
              >
                {remote.map((branch) => (
                  <BranchRow
                    key={branch}
                    label={branch}
                    selected={base === valueOf("remote", branch)}
                    onSelect={() => setBase(valueOf("remote", branch))}
                  />
                ))}
              </BranchGroup>
            )}
            {empty && (
              <div className="px-2.5 py-2 text-xs text-muted-foreground">
                No branches found
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
