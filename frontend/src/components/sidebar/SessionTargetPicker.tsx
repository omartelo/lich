import { useEffect, useMemo, useRef, useState } from "react"
import type { KeyboardEvent } from "react"
import { Dialog as DialogPrimitive } from "@base-ui/react/dialog"
import { CornerDownLeft, Search } from "lucide-react"
import { cn } from "@/lib/utils"
import { useSessionStatus } from "@/lib/session/use-session-status"
import { filterTargetRows, flattenTargetGroups, type TargetRow } from "@/lib/session/target-picker"
import type { DelegateGroup, DelegateTarget } from "@/lib/session/delegate-targets"
import { Keys } from "@/components/common/Keys"
import { SessionStatusIcon } from "./SessionStatusIcon"

interface SessionTargetPickerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  groups: readonly DelegateGroup[]
  onPick: (target: DelegateTarget) => void
}

interface DisplayGroup {
  projectId: string
  projectName: string
  rows: { row: TargetRow; index: number }[]
}

const TITLE = "Delegate to session"

// A searchable stand-in for the plain, ungrouped submenu the delegate
// context-menu item used to open (SessionCard.tsx): with several sessions
// open, finding the right one took a scroll and gave no sense of which
// sessions were busy or waiting. Same idiom as CommandPalette — centered
// dialog, grouped rows, status-ringed icons — scoped down to "hand this
// session's work to another".
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
  useEffect(() => setSelected(0), [query])
  const active = Math.min(selected, Math.max(0, results.length - 1))

  // Grouped by project in first-appearance order, which — since `rows` is
  // built project-by-project — matches the order the caller passed them in.
  const displayGroups = useMemo(() => {
    const byProject = new Map<string, DisplayGroup>()
    results.forEach((row, index) => {
      const group = byProject.get(row.projectId) ?? {
        projectId: row.projectId,
        projectName: row.projectName,
        rows: [],
      }
      group.rows.push({ row, index })
      byProject.set(row.projectId, group)
    })
    return [...byProject.values()]
  }, [results])
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
      setSelected((i) => Math.min(i + 1, results.length - 1))
    } else if (event.key === "ArrowUp") {
      event.preventDefault()
      setSelected((i) => Math.max(i - 1, 0))
    } else if (event.key === "Enter") {
      event.preventDefault()
      const row = results[active]
      if (row) {
        pick(row.target)
      }
    }
  }

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/50 supports-backdrop-filter:backdrop-blur-xs data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0" />
        <DialogPrimitive.Popup className="fixed left-1/2 top-[20vh] z-50 flex max-h-[60vh] w-full max-w-md -translate-x-1/2 flex-col overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-2xl ring-1 ring-foreground/10 outline-none data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0">
          <DialogPrimitive.Title className="sr-only">{TITLE}</DialogPrimitive.Title>

          <div className="flex items-center gap-3 border-b px-4 py-3">
            <Search className="size-4 shrink-0 text-muted-foreground" />
            <input
              // biome-ignore lint/a11y/noAutofocus: the picker opens to be typed into right away.
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={onInputKeyDown}
              placeholder="Search sessions to delegate to…"
              aria-label={TITLE}
              autoComplete="off"
              spellCheck={false}
              className="flex-1 bg-transparent text-[0.9375rem] outline-none placeholder:text-muted-foreground"
            />
          </div>

          <div role="listbox" aria-label={TITLE} className="flex-1 overflow-y-auto p-1.5">
            {results.length === 0 ? (
              <div className="px-3 py-8 text-center text-sm text-muted-foreground">
                No sessions match{" "}
                <span className="font-mono text-foreground/80">{query.trim()}</span>
              </div>
            ) : (
              displayGroups.map((group) => (
                // biome-ignore lint/a11y/useSemanticElements: a listbox takes group, not fieldset.
                <div key={group.projectId} role="group" aria-label={group.projectName}>
                  {showHeadings && <GroupLabel>{group.projectName}</GroupLabel>}
                  {group.rows.map(({ row, index }) => (
                    <Row
                      key={row.target.id}
                      target={row.target}
                      selected={index === active}
                      onSelect={() => setSelected(index)}
                      onPick={() => pick(row.target)}
                    />
                  ))}
                </div>
              ))
            )}
          </div>

          <div className="flex items-center gap-4 border-t bg-black/10 px-4 py-2 text-xs text-muted-foreground">
            <Hint keys={["↑", "↓"]}>navigate</Hint>
            <Hint keys={["↵"]}>pick</Hint>
            <Hint keys={["esc"]}>close</Hint>
          </div>
        </DialogPrimitive.Popup>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}

function GroupLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-3 pb-1 pt-3 text-[0.65625rem] font-semibold uppercase tracking-wider text-muted-foreground">
      {children}
    </div>
  )
}

function Row({
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
  const ref = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (selected) {
      ref.current?.scrollIntoView({ block: "nearest" })
    }
  }, [selected])
  return (
    <button
      ref={ref}
      type="button"
      role="option"
      aria-selected={selected}
      onMouseMove={onSelect}
      onClick={onPick}
      className={cn(
        "group flex w-full items-center gap-3 rounded-md px-3 py-2 text-left outline-none",
        selected ? "bg-accent text-accent-foreground" : "text-foreground",
      )}
    >
      <SessionStatusIcon kind={target.kind} status={status} />
      <span className="truncate text-sm">{target.label}</span>
      <CornerDownLeft
        className={cn(
          "ml-auto size-3.5 shrink-0 text-muted-foreground",
          selected ? "opacity-100" : "opacity-0",
        )}
      />
    </button>
  )
}

function Hint({ keys, children }: { keys: string[]; children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="inline-flex gap-1">
        {keys.map((k) => (
          <Keys key={k}>{k}</Keys>
        ))}
      </span>
      {children}
    </span>
  )
}
