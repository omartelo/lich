import { useEffect, useMemo, useState } from "react"
import type { KeyboardEvent } from "react"
import { GitBranchPlus } from "lucide-react"
import { useSessionStatus, useSessionUnread } from "@/lib/session/use-session-status"
import { filterTargetRows, flattenTargetGroups, groupTargetRows } from "@/lib/session/target-picker"
import type { DelegateGroup, DelegateTarget } from "@/lib/session/delegate-targets"
import { PickerDialog, PickerEmpty, PickerGroup, PickerRow } from "@/components/common/PickerDialog"
import { SessionStatusIcon } from "./SessionStatusIcon"

interface SessionTargetPickerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  groups: readonly DelegateGroup[]
  onPick: (target: DelegateTarget) => void
  /** Delegate to a session that does not exist yet: a fresh worktree. */
  onPickWorktree: () => void
}

const TITLE = "Delegate to session"

// A searchable stand-in for the plain, ungrouped submenu the delegate
// context-menu item used to open (SessionCard.tsx): with several sessions
// open, finding the right one took a scroll and gave no sense of which
// sessions were busy or waiting. The command palette's own surface
// (PickerDialog), scoped down to "hand this session's work to another".
//
// One row is pinned under the list: a new worktree session. It is an action,
// not a search hit, so the query never filters it out and a long list scrolls
// beneath it — and it is what the picker opens onto when no other session is
// live, which is exactly when fanning out is most useful.
export function SessionTargetPicker({
  open,
  onOpenChange,
  groups,
  onPick,
  onPickWorktree,
}: SessionTargetPickerProps) {
  const [query, setQuery] = useState("")
  const [selected, setSelected] = useState(0)

  useEffect(() => {
    if (open) {
      setQuery("")
      setSelected(0)
    }
  }, [open])

  const rows = useMemo(() => flattenTargetGroups(groups), [groups])
  const results = useMemo(() => filterTargetRows(query, rows), [query, rows])
  const displayGroups = useMemo(() => groupTargetRows(results), [results])
  useEffect(() => setSelected(0), [query])
  // The pinned worktree row sits at index results.length, so it is always the
  // last stop of the arrow keys — and the only one when nothing else is live.
  const active = Math.min(selected, results.length)
  // A project heading only earns its keep once there is more than one to tell
  // apart — the rule the old flat submenu applied to its own labels.
  const showHeadings = displayGroups.length > 1

  const pick = (target: DelegateTarget) => {
    onPick(target)
    onOpenChange(false)
  }
  const pickWorktree = () => {
    onPickWorktree()
    onOpenChange(false)
  }

  const onInputKeyDown = (event: KeyboardEvent) => {
    if (event.key === "ArrowDown") {
      event.preventDefault()
      setSelected(Math.min(active + 1, results.length))
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setSelected(Math.max(active - 1, 0))
    } else if (event.key === "Enter") {
      event.preventDefault()
      if (active === results.length) {
        pickWorktree()
        return
      }
      const row = results[active]
      if (row) {
        pick(row.target)
      }
    }
  }

  return (
    <PickerDialog
      open={open}
      onOpenChange={onOpenChange}
      title={TITLE}
      placeholder="Search sessions to delegate to…"
      searchLabel="Search sessions to delegate to"
      resultsLabel="Sessions this one can delegate to"
      query={query}
      onQueryChange={setQuery}
      onKeyDown={onInputKeyDown}
      actionHint="pick"
    >
      {results.length === 0 && query.trim() !== "" && (
        <PickerEmpty>
          No sessions match <span className="font-mono text-foreground/80">{query.trim()}</span>
        </PickerEmpty>
      )}
      {displayGroups.map((group) => (
        <PickerGroup key={group.projectId} label={group.projectName} showLabel={showHeadings}>
          {group.rows.map(({ row, index }) => (
            <TargetRowView
              key={row.target.id}
              target={row.target}
              selected={index === active}
              onSelect={() => setSelected(index)}
              onPick={() => pick(row.target)}
            />
          ))}
        </PickerGroup>
      ))}
      {/* Sticky, so a long list scrolls under it instead of burying it. It
          stays inside the listbox because an option outside one is announced
          to nobody; the negative margins let its ground bleed over the list's
          own padding while it floats. */}
      <div className="sticky -bottom-1.5 -mx-1.5 -mb-1.5 mt-1.5 border-t bg-popover p-1.5">
        <PickerRow
          selected={active === results.length}
          onSelect={() => setSelected(results.length)}
          onRun={pickWorktree}
        >
          <GitBranchPlus className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate text-sm">New worktree session…</span>
        </PickerRow>
      </div>
    </PickerDialog>
  )
}

function TargetRowView({
  target,
  selected,
  onSelect,
  onPick,
}: {
  target: DelegateTarget
  selected: boolean
  onSelect: () => void
  onPick: () => void
}) {
  const status = useSessionStatus(target.id)
  const unread = useSessionUnread(target.id)
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onPick}>
      <SessionStatusIcon kind={target.kind} status={status} unread={unread} />
      <span className="truncate text-sm">{target.label}</span>
    </PickerRow>
  )
}
