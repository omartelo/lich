import { useEffect, useMemo, useState } from "react"
import type { KeyboardEvent } from "react"
import { useSessionStatus } from "@/lib/session/use-session-status"
import { filterTargetRows, flattenTargetGroups, groupTargetRows } from "@/lib/session/target-picker"
import type { DelegateGroup, DelegateTarget } from "@/lib/session/delegate-targets"
import { PickerDialog, PickerEmpty, PickerGroup, PickerRow } from "@/components/common/PickerDialog"
import { SessionStatusIcon } from "./SessionStatusIcon"

interface SessionTargetPickerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  groups: readonly DelegateGroup[]
  onPick: (target: DelegateTarget) => void
}

const TITLE = "Delegate to session"

// A searchable stand-in for the plain, ungrouped submenu the delegate
// context-menu item used to open (SessionCard.tsx): with several sessions
// open, finding the right one took a scroll and gave no sense of which
// sessions were busy or waiting. The command palette's own surface
// (PickerDialog), scoped down to "hand this session's work to another".
export function SessionTargetPicker({
  open,
  onOpenChange,
  groups,
  onPick,
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
  const active = Math.min(selected, Math.max(0, results.length - 1))
  // A project heading only earns its keep once there is more than one to tell
  // apart — the rule the old flat submenu applied to its own labels.
  const showHeadings = displayGroups.length > 1

  const pick = (target: DelegateTarget) => {
    onPick(target)
    onOpenChange(false)
  }

  const onInputKeyDown = (event: KeyboardEvent) => {
    if (event.key === "ArrowDown") {
      event.preventDefault()
      setSelected(Math.min(active + 1, results.length - 1))
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setSelected(Math.max(active - 1, 0))
    } else if (event.key === "Enter") {
      event.preventDefault()
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
      {results.length === 0 ? (
        <PickerEmpty>
          No sessions match <span className="font-mono text-foreground/80">{query.trim()}</span>
        </PickerEmpty>
      ) : (
        displayGroups.map((group) => (
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
        ))
      )}
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
  return (
    <PickerRow selected={selected} onSelect={onSelect} onRun={onPick}>
      <SessionStatusIcon kind={target.kind} status={status} />
      <span className="truncate text-sm">{target.label}</span>
    </PickerRow>
  )
}
