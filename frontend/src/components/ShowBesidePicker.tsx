import { useEffect, useMemo, useRef, useState } from "react"
import type { KeyboardEvent } from "react"
import { PickerDialog, PickerEmpty, PickerGroup } from "@/components/common/PickerDialog"
import { Trans } from "@/components/common/Trans"
import { TargetRowView } from "@/components/sidebar/SessionTargetPicker"
import { useT } from "@/lib/i18n/i18n"
import { besideTargets } from "@/lib/session/beside-targets"
import { groupOf, type PaneGroup } from "@/lib/session/panes"
import { filterTargetRows, flattenTargetGroups, groupTargetRows } from "@/lib/session/target-picker"
import { useProjects } from "@/providers/projects"

interface ShowBesidePickerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The routed project, whose sessions are listed first. */
  projectId: string
  /** The sessions on screen, which are not offered again. */
  onScreen: readonly string[]
  /** Every wall, to name the one a session would be taken off. */
  groups: readonly PaneGroup[]
  onPick: (sessionId: string) => void
  /** Called once the close has finished, so the caller can unmount it. */
  onClosed: () => void
}

// What a pane's + opens: every session of every open project that is not on
// screen yet, searchable, grouped by project — the delegate picker's surface
// and rows, aimed at the wall instead. A session on another wall says which, so
// the question the move dialog asks next is not a surprise.
export function ShowBesidePicker({
  open,
  onOpenChange,
  projectId,
  onScreen,
  groups,
  onPick,
  onClosed,
}: ShowBesidePickerProps) {
  const t = useT()
  const { projects, sessions } = useProjects()
  const [query, setQuery] = useState("")
  const [selected, setSelected] = useState(0)
  // The pick runs once the dialog has finished closing: the move dialog it may
  // raise would otherwise open under this one's focus trap.
  const picked = useRef("")

  useEffect(() => {
    if (open) {
      setQuery("")
      setSelected(0)
    }
  }, [open])

  const rows = useMemo(
    () => flattenTargetGroups(besideTargets(projects, sessions, projectId, onScreen)),
    [projects, sessions, projectId, onScreen],
  )
  const results = useMemo(() => filterTargetRows(query, rows), [query, rows])
  const displayGroups = useMemo(() => groupTargetRows(results), [results])
  useEffect(() => setSelected(0), [query])
  const active = Math.min(selected, Math.max(results.length - 1, 0))

  const pick = (sessionId: string) => {
    picked.current = sessionId
    onOpenChange(false)
  }
  const runPicked = (isOpen: boolean) => {
    if (isOpen) {
      return
    }
    const sessionId = picked.current
    picked.current = ""
    if (sessionId) {
      onPick(sessionId)
    }
    onClosed()
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
        pick(row.target.id)
      }
    }
  }

  return (
    <PickerDialog
      open={open}
      onOpenChange={onOpenChange}
      onOpenChangeComplete={runPicked}
      title={t("terminal.showBesidePicker.title")}
      placeholder={t("terminal.showBesidePicker.placeholder")}
      searchLabel={t("terminal.showBesidePicker.search")}
      resultsLabel={t("terminal.showBesidePicker.results")}
      query={query}
      onQueryChange={setQuery}
      onKeyDown={onInputKeyDown}
      actionHint={t("terminal.showBesidePicker.pick")}
    >
      {results.length === 0 && query.trim() !== "" && (
        <PickerEmpty>
          <Trans
            k="terminal.showBesidePicker.noMatch"
            params={{
              query: <span className="font-mono text-foreground/80">{query.trim()}</span>,
            }}
          />
        </PickerEmpty>
      )}
      {displayGroups.map((group) => (
        <PickerGroup key={group.projectId} label={group.projectName}>
          {group.rows.map(({ row, index }) => {
            const wall = groupOf(groups, row.target.id)
            return (
              <TargetRowView
                key={row.target.id}
                target={row.target}
                selected={index === active}
                onSelect={() => setSelected(index)}
                onPick={() => pick(row.target.id)}
                trailing={
                  wall ? t("terminal.showBesidePicker.onWall", { group: wall.name }) : undefined
                }
              />
            )
          })}
        </PickerGroup>
      ))}
    </PickerDialog>
  )
}
